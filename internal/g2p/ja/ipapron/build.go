package ipapron

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/ikawaha/kagome-dict/dict"
)

// Buildは、同じ発音を共有し各文字を1byteの符号にした表を作る。
func Build(d *dict.Dict) ([]byte, error) {
	column, ok := d.ContentsMeta[dict.PronunciationIndex]
	if !ok || len(d.Contents) != len(d.Morphs) {
		return nil, fmt.Errorf("dictionary has no pronunciation contents")
	}
	pronunciations := make([]string, len(d.Morphs))
	codes := map[rune]int{}
	for id, row := range d.Contents {
		i := int(column) - len(d.POSTable.POSs[id])
		if i < 0 || i >= len(row) {
			return nil, fmt.Errorf("morph %d has no pronunciation", id)
		}
		pronunciations[id] = row[i]
		for _, r := range row[i] {
			codes[r] = 0
		}
	}
	runes := make([]rune, 0, len(codes))
	for r := range codes {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	if len(runes) > 256 {
		return nil, fmt.Errorf("%d distinct characters do not fit one byte", len(runes))
	}
	for i, r := range runes {
		codes[r] = i
	}

	var blob []byte
	offsets := map[string]int{}
	index := make([]byte, len(pronunciations)*4)
	for id, pronunciation := range pronunciations {
		offset, seen := offsets[pronunciation]
		length := len([]rune(pronunciation))
		if !seen {
			offset = len(blob)
			offsets[pronunciation] = offset
			for _, r := range pronunciation {
				blob = append(blob, byte(codes[r]))
			}
		}
		if length > 0xff || offset > 0xffffff {
			return nil, fmt.Errorf("morph %d does not fit the table layout", id)
		}
		binary.LittleEndian.PutUint32(index[id*4:], uint32(offset)<<8|uint32(length))
	}

	var out bytes.Buffer
	out.WriteString(magic)
	binary.Write(&out, binary.LittleEndian, DictHash(d))
	binary.Write(&out, binary.LittleEndian, uint32(len(pronunciations)))
	binary.Write(&out, binary.LittleEndian, uint32(len(runes)))
	for _, r := range runes {
		binary.Write(&out, binary.LittleEndian, uint32(r))
	}
	out.Write(index)
	out.Write(blob)
	return out.Bytes(), nil
}
