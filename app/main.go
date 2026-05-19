package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

type TorrentFile struct {
	Announce string
	Info     TorrentInfo
	InfoHash []byte
}

type TorrentInfo struct {
	Name        string
	Length      int
	PieceLength int
	Pieces      string
}

type ConnectionInfo struct {
	PeerId   []byte
	BitField []byte // Indicates which pieces the peer has
}

func parseTorrentFile(torrentPath string) TorrentFile {
	file, _ := os.OpenFile(torrentPath, os.O_RDONLY, 0777)
	decoded, _ := bencode.Decode(bufio.NewReader(file))
	buf := make([]byte, 2048)
	file.Read(buf)
	fmt.Println(hex.EncodeToString(buf))
	file.Close()
	d := decoded.(map[string]any)

	info := d["info"].(map[string]any)
	encodedInfo, _ := bencode.Encode(info)
	infoHash := sha1.Sum([]byte(encodedInfo))

	return TorrentFile{
		Announce: d["announce"].(string),
		Info: TorrentInfo{
			Name:        info["name"].(string),
			Length:      info["length"].(int),
			PieceLength: info["piece length"].(int),
			Pieces:      info["pieces"].(string),
		},
		InfoHash: infoHash[:],
	}
}

func parseMagnetLink(magnetLink string) (TorrentFile, error) {
	if magnetLink[:8] != "magnet:?" {
		return TorrentFile{}, fmt.Errorf("Malformed magnet url: %s", magnetLink)
	}
	vals, _ := url.ParseQuery(magnetLink[8:])
	infoHash, _ := hex.DecodeString(vals["xt"][0][9:])
	return TorrentFile{
		Announce: vals["tr"][0],
		Info:     TorrentInfo{Name: vals["dn"][0], Length: 999},
		InfoHash: infoHash,
	}, nil
}

func findPeers(torrent TorrentFile) []string {
	vals := make(url.Values)
	vals.Add("info_hash", string(torrent.InfoHash))
	vals.Add("peer_id", "idk_some_randomid_ig")
	vals.Add("port", "6881")
	vals.Add("uploaded", "0")
	vals.Add("downloaded", "0")
	vals.Add("left", strconv.Itoa(torrent.Info.Length))
	vals.Add("compact", "1")

	resp, _ := http.Get(fmt.Sprintf("%s?%s", torrent.Announce, vals.Encode()))
	result, _ := bencode.Decode(bufio.NewReader(resp.Body))
	peers := []byte(result.(map[string]any)["peers"].(string))

	var peerStr []string
	for i := 0; i < len(peers); i += 6 {
		peerStr = append(peerStr, fmt.Sprintf("%d.%d.%d.%d:%d", peers[i], peers[i+1], peers[i+2], peers[i+3], binary.BigEndian.Uint16(peers[i+4:i+6])))
	}

	return peerStr
}

func performHandshake(connection net.Conn, infoHash []byte, supportsExtensions bool) (ConnectionInfo, error) {
	message := make([]byte, 68)
	message[0] = 19
	var extByte byte = 0
	if supportsExtensions {
		extByte = 0x10
	}
	copy(message[1:20], []byte("BitTorrent protocol"))
	copy(message[20:28], []byte{0, 0, 0, 0, 0, extByte, 0, 0})
	copy(message[28:48], infoHash)
	copy(message[48:68], []byte("idk_some_randomid_ig"))
	connection.Write(message)

	buf := make([]byte, 68)
	io.ReadFull(connection, buf)
	peerId := buf[48:68]
	bitField := readPeerMessage(connection)
	if len(bitField) > 0 && bitField[0] != 0x05 {
		return ConnectionInfo{}, fmt.Errorf("Expected to receive a bitfield message (message id of 5), but received a message with id of %d", bitField[0])
	}

	writePeerMessage(connection, []byte{0x02})
	msg := readPeerMessage(connection)
	if !supportsExtensions && !bytes.Equal(msg, []byte{0x01}) {
		var msgId string = "[empty]"
		if len(msg) > 0 {
			msgId = string(msg[0])
		}
		return ConnectionInfo{}, fmt.Errorf("Expected to receive an unchoke message (message id of 1), but received a message with id of %s", msgId)
	}

	return ConnectionInfo{PeerId: peerId, BitField: bitField[1:]}, nil
}

func readPeerMessage(connection net.Conn) []byte {
	lenBuf := make([]byte, 4)
	io.ReadFull(connection, lenBuf)
	length := binary.BigEndian.Uint32(lenBuf)
	msgBuf := make([]byte, length)
	io.ReadFull(connection, msgBuf)

	return msgBuf
}

func writePeerMessage(connection net.Conn, bytes []byte) {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
	connection.Write(lenBuf)
	connection.Write(bytes)
}

func getPieceFromConnection(conn net.Conn, torrent TorrentFile, pieceIndex int) ([]byte, error) {
	const blockSize = 16384 // 2 ** 14 -> 16KiB

	pieceLength := torrent.Info.PieceLength
	leftToDownload := torrent.Info.Length - pieceIndex*pieceLength
	pieceLength = min(pieceLength, leftToDownload)

	var data []byte

	for i := 0; i < pieceLength; i += int(blockSize) {
		message := []byte{0x06}
		message = binary.BigEndian.AppendUint32(message, uint32(pieceIndex))
		message = binary.BigEndian.AppendUint32(message, uint32(i))
		length := int(math.Min(blockSize, float64(pieceLength-i)))
		message = binary.BigEndian.AppendUint32(message, uint32(length))
		writePeerMessage(conn, message)
		msg := readPeerMessage(conn)
		data = append(data, msg[9:]...)
		fmt.Printf("Downloaded %d/%d bytes of the piece\n", i+length, pieceLength)
	}

	expectedChecksum := []byte(torrent.Info.Pieces[pieceIndex*20 : (pieceIndex+1)*20])
	actualChecksum := sha1.Sum(data)

	if !bytes.Equal(actualChecksum[:], expectedChecksum) {
		return nil, fmt.Errorf("Invalid checksum")
	}

	return data, nil
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, err := bencode.Decode(bufio.NewReader(strings.NewReader(bencodedValue)))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	case "info", "peers", "handshake":
		torrent := parseTorrentFile(os.Args[2])
		var pieceHashes strings.Builder
		for i := 0; i < len(torrent.Info.Pieces); i += 20 {
			pieceHashes.WriteString("\n")
			pieceHashes.WriteString(hex.EncodeToString([]byte(torrent.Info.Pieces[i : i+20])))
		}

		switch command {
		case "info":
			fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %s\nPiece Length: %d\nPiece Hashes: %s\n", torrent.Announce, torrent.Info.Length, hex.EncodeToString(torrent.InfoHash), torrent.Info.PieceLength, pieceHashes.String())
		case "peers":
			fmt.Println(strings.Join(findPeers(torrent), "\n"))
		case "handshake":
			conn, _ := net.Dial("tcp", os.Args[3])
			info, _ := performHandshake(conn, torrent.InfoHash, false)
			fmt.Printf("Peer ID: %s\n", hex.EncodeToString(info.PeerId))
		}
	case "download_piece":
		outputPath := os.Args[3]
		torrent := parseTorrentFile(os.Args[4])
		pieceIndex, _ := strconv.Atoi(os.Args[5])

		peers := findPeers(torrent)
		var data []byte
		var err error = fmt.Errorf("No peers have that piece")

		for _, peer := range peers {
			conn, _ := net.Dial("tcp", peer)
			info, _ := performHandshake(conn, torrent.InfoHash, false)
			hasPiece := info.BitField[pieceIndex/8] & (1 << (7 - pieceIndex%8))
			if hasPiece == 0x01 {
				data, err = getPieceFromConnection(conn, torrent, pieceIndex)
				conn.Close()
				break
			}
			conn.Close()
		}

		if err != nil {
			fmt.Println(err)
			return
		}

		file, _ := os.Create(outputPath)
		file.Write(data)
		file.Close()
	case "download":
		outputPath := os.Args[3]
		torrent := parseTorrentFile(os.Args[4])
		peers := findPeers(torrent)
		conn, _ := net.Dial("tcp", peers[0])
		performHandshake(conn, torrent.InfoHash, false)
		file, _ := os.Create(outputPath)

		pieceCount := len(torrent.Info.Pieces) / 20
		for i := range pieceCount {
			data, err := getPieceFromConnection(conn, torrent, i)
			if err != nil {
				fmt.Println(err)
				file.Close()
				os.Remove(outputPath)
				conn.Close()
				return
			}
			file.WriteAt(data, int64(i*torrent.Info.PieceLength))
		}

		file.Close()
		conn.Close()
	case "magnet_parse":
		torrent, _ := parseMagnetLink(os.Args[2])
		fmt.Printf("Tracker URL: %s\nInfo Hash: %s\n", torrent.Announce, torrent.InfoHash)
	case "magnet_handshake":
		torrent, _ := parseMagnetLink(os.Args[2])
		peers := findPeers(torrent)
		conn, _ := net.Dial("tcp", peers[0])
		info, err := performHandshake(conn, torrent.InfoHash, true)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Peer ID: " + hex.EncodeToString(info.PeerId))
		conn.Close()
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
