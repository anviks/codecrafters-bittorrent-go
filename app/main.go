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
		encoded_info, _ := bencode.Encode(data["info"])
		hash := sha1.Sum([]byte(encoded_info))
		fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %s\n", data["announce"], data["info"].(map[string]any)["length"], hex.EncodeToString(hash[:]))
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
