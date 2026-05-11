package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func parseTorrentFile(torrentPath string) TorrentFile {
	file, _ := os.OpenFile(torrentPath, os.O_RDONLY, 0777)
	decoded, _ := bencode.Decode(bufio.NewReader(file))
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
		peerStr = append(peerStr, fmt.Sprintf("%d.%d.%d.%d:%d", peers[i], peers[i+1], peers[i+2], peers[i+3], int32(peers[i+4])<<8|int32(peers[i+5])))
	}

	return peerStr
}

func performHandshake(connection net.Conn, infoHash []byte) {
	message := make([]byte, 68)
	message[0] = 19
	copy(message[1:20], []byte("BitTorrent protocol"))
	copy(message[20:28], []byte{0, 0, 0, 0, 0, 0, 0, 0})
	copy(message[28:48], infoHash)
	copy(message[48:68], []byte("idk_some_randomid_ig"))
	connection.Write(message)
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
			performHandshake(conn, torrent.InfoHash)
			buf := make([]byte, 68)
			conn.Read(buf)
			peerId := buf[48:68]
			fmt.Printf("Peer ID: %s\n", hex.EncodeToString(peerId))
		}
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
