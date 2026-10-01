// Package ipapronは、Kagome IPA辞書の既知語の発音だけを小さな表で引く。
// 辞書本文（全素性）を読まずに読み変換するために使う。表はmecab-ipadic由来の生成物で、
// ライセンスはkagome-dict/ipaと同じ（THIRD_PARTY_NOTICES.txt参照）。
package ipapron

//go:generate go run ./gen -out pron.bin

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"strings"

	"github.com/ikawaha/kagome-dict/dict"
)

const magic = "UTPRON01"

//go:embed pron.bin
var data []byte

// Tableは形態素IDから発音を返す。埋め込みデータを複製せずに参照する。
type Table struct {
	runes []rune
	index []byte
	blob  []byte
}

// Loadは埋め込み表を読み、渡された辞書の形態素表と一致する場合だけ返す。
func Load(d *dict.Dict) (*Table, error) {
	return parse(data, DictHash(d), len(d.Morphs))
}

func parse(data []byte, hash uint64, morphs int) (*Table, error) {
	if len(data) < 24 || string(data[:8]) != magic {
		return nil, errors.New("ipapron: invalid table header")
	}
	if binary.LittleEndian.Uint64(data[8:]) != hash {
		return nil, errors.New("ipapron: table does not match the dictionary")
	}
	count := int(binary.LittleEndian.Uint32(data[16:]))
	runeCount := int(binary.LittleEndian.Uint32(data[20:]))
	if count != morphs || runeCount > 256 {
		return nil, errors.New("ipapron: table does not match the dictionary")
	}
	rest := data[24:]
	if len(rest) < runeCount*4+count*4 {
		return nil, errors.New("ipapron: truncated table")
	}
	table := &Table{runes: make([]rune, runeCount)}
	for i := range table.runes {
		table.runes[i] = rune(binary.LittleEndian.Uint32(rest[i*4:]))
	}
	rest = rest[runeCount*4:]
	table.index, table.blob = rest[:count*4], rest[count*4:]
	for id := 0; id < count; id++ {
		offset, length := table.span(id)
		if offset+length > len(table.blob) {
			return nil, errors.New("ipapron: truncated table")
		}
	}
	for _, code := range table.blob {
		if int(code) >= runeCount {
			return nil, errors.New("ipapron: invalid character code")
		}
	}
	return table, nil
}

func (t *Table) span(id int) (offset, length int) {
	value := binary.LittleEndian.Uint32(t.index[id*4:])
	return int(value >> 8), int(value & 0xff)
}

// Pronunciationは既知語IDの発音を返す。範囲外のIDはfalse。
func (t *Table) Pronunciation(id int) (string, bool) {
	if id < 0 || id*4 >= len(t.index) {
		return "", false
	}
	offset, length := t.span(id)
	var builder strings.Builder
	builder.Grow(length * 3)
	for _, code := range t.blob[offset : offset+length] {
		builder.WriteRune(t.runes[code])
	}
	return builder.String(), true
}

// DictHashは、本文を除いても読み込まれる形態素表と品詞表から辞書を識別する。
func DictHash(d *dict.Dict) uint64 {
	hash := fnv.New64a()
	var buf [8]byte
	write := func(values ...uint16) {
		for _, value := range values {
			binary.LittleEndian.PutUint16(buf[:], value)
			hash.Write(buf[:2])
		}
	}
	binary.LittleEndian.PutUint64(buf[:], uint64(len(d.Morphs)))
	hash.Write(buf[:])
	for _, morph := range d.Morphs {
		write(uint16(morph.LeftID), uint16(morph.RightID), uint16(morph.Weight))
	}
	for _, pos := range d.POSTable.POSs {
		write(uint16(len(pos)))
		for _, id := range pos {
			write(uint16(id))
		}
	}
	for _, name := range d.POSTable.NameList {
		hash.Write([]byte(name))
		hash.Write([]byte{0})
	}
	return hash.Sum64()
}
