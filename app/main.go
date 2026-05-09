package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(reader *bufio.Reader) (any, error) {
	rn, _, _ := reader.ReadRune()

	if unicode.IsDigit(rn) {
		var lengthStr strings.Builder

		for ; rn != ':'; rn, _, _ = reader.ReadRune() {
			lengthStr.WriteRune((rn))
		}

		length, err := strconv.Atoi(lengthStr.String())
		if err != nil {
			return "", err
		}

		var result strings.Builder

		for range length {
			rn, _, _ = reader.ReadRune()
			result.WriteRune((rn))
		}

		return result.String(), nil
	} else if rn == 'i' {
		var numStr strings.Builder

		rn, _, _ = reader.ReadRune()
		for ; rn != 'e'; rn, _, _ = reader.ReadRune() {
			numStr.WriteRune(rn)
		}

		num, err := strconv.Atoi(numStr.String())
		if err != nil {
			return "", err
		}

		return num, nil
	} else if rn == 'l' {
		var ls []any

		for {
			b, _ := reader.Peek(1)
			rn = rune(b[0])
			if rn == 'e' {
				return ls, nil
			}
			result, _ := decodeBencode(reader)
			ls = append(ls, result)
		}
	} else if rn == 'd' {
		d := make(map[string]any)

		for {
			b, _ := reader.Peek(1)
			rn = rune(b[0])
			if rn == 'e' {
				return d, nil
			}
			key, _ := decodeBencode(reader)
			value, _ := decodeBencode(reader)
			d[key.(string)] = value
		}
	} else {
		return "", fmt.Errorf("Only strings are supported at the moment")
	}
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bufio.NewReader(strings.NewReader(bencodedValue)))
		// decoded, err := bencode.Decode(strings.NewReader(bencodedValue))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
