package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	case "info", "peers":
		torrentPath := os.Args[2]
		file, _ := os.OpenFile(torrentPath, os.O_RDONLY, 0777)
		decoded, _ := bencode.Decode(bufio.NewReader(file))
		file.Close()
		data := decoded.(map[string]any)
		info := data["info"].(map[string]any)
		encoded_info, _ := bencode.Encode(info)
		hash := sha1.Sum([]byte(encoded_info))
		var pieceHashes strings.Builder
		for i := 0; i < len(info["pieces"].(string)); i += 20 {
			pieceHashes.WriteString("\n")
			pieceHashes.WriteString(hex.EncodeToString([]byte(info["pieces"].(string)[i : i+20])))
		}
		if command == "info" {
			fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %s\nPiece Length: %d\nPiece Hashes: %s\n", data["announce"], info["length"], hex.EncodeToString(hash[:]), info["piece length"], pieceHashes.String())
		} else {
			vals := make(url.Values)
			vals.Add("info_hash", string(hash[:]))
			vals.Add("peer_id", "idk_some_randomid_ig")
			vals.Add("port", "6881")
			vals.Add("uploaded", "0")
			vals.Add("downloaded", "0")
			vals.Add("left", strconv.Itoa(info["length"].(int)))
			vals.Add("compact", "1")
			resp, _ := http.Get(fmt.Sprintf("%s?%s", data["announce"].(string), vals.Encode()))
			result, _ := bencode.Decode(bufio.NewReader(resp.Body))
			peers := []byte(result.(map[string]any)["peers"].(string))
			var peerStr strings.Builder
			for i := 0; i < len(peers); i += 6 {
				fmt.Fprintf(&peerStr, "%d.%d.%d.%d:%d\n", peers[i], peers[i+1], peers[i+2], peers[i+3], int32(peers[i+4])<<8|int32(peers[i+5]))
			}
			fmt.Print(peerStr.String())
		}
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
