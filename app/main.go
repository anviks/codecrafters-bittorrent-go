package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

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
	case "info":
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
		fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %s\nPiece Length: %d\nPiece Hashes: %s\n", data["announce"], info["length"], hex.EncodeToString(hash[:]), info["piece length"], pieceHashes.String())
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
