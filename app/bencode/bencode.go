package bencode

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func Decode(reader *bufio.Reader) (any, error) {
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

		buf := make([]byte, length)
		io.ReadFull(reader, buf)

		return string(buf), nil
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
			result, _ := Decode(reader)
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
			key, _ := Decode(reader)
			value, _ := Decode(reader)
			d[key.(string)] = value
		}
	} else {
		return "", fmt.Errorf("Unknown bencode type specifier: %c\n", rn)
	}
}

func Encode(value any) (string, error) {
	switch val := value.(type) {
	case string:
		return fmt.Sprintf("%d:%s", len(val), val), nil
	case int:
		return fmt.Sprintf("i%de", val), nil
	case []any:
		var sb strings.Builder
		for i := range len(val) {
			s, _ := Encode(val[i])
			sb.WriteString(s)
		}
		return fmt.Sprintf("l%se", sb.String()), nil
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for _, k := range keys {
			s, _ := Encode(k)
			sb.WriteString(s)
			s, _ = Encode(val[k])
			sb.WriteString(s)
		}
		return fmt.Sprintf("d%se", sb.String()), nil
	default:
		return "", fmt.Errorf("Unknown type: %T\n", val)
	}
}
