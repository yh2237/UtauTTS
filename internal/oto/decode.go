package oto

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func Decode(data []byte) (string, string, error) {
	if len(data) >= 2 && ((data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff)) {
		var order binary.ByteOrder = binary.LittleEndian
		encoding := "UTF-16LE"
		if data[0] == 0xfe {
			order = binary.BigEndian
			encoding = "UTF-16BE"
		}
		body := data[2:]
		if len(body)%2 != 0 {
			return "", "", fmt.Errorf("odd-length %s data", encoding)
		}
		units := make([]uint16, len(body)/2)
		for index := range units {
			units[index] = order.Uint16(body[index*2:])
		}
		return string(utf16.Decode(units)), encoding, nil
	}
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	if utf8.Valid(data) {
		return string(data), "UTF-8", nil
	}
	decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	if err != nil {
		return "", "", err
	}
	return string(decoded), "Shift_JIS", nil
}
