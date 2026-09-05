package sidecar

import (
	"fmt"
	"path/filepath"
	"strings"
	"utautts/internal/atomicfile"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const (
	EncodingUTF8     = "utf-8"
	EncodingShiftJIS = "shift_jis"
)

type Options struct {
	WriteText bool
	WriteLab  bool
	Encoding  string
	Text      string
	Lab       string
}

// WriteはWAVと同じ場所へ同名の字幕と音素ラベルを書き出す。
func Write(wavPath string, options Options) error {
	if !options.WriteText && !options.WriteLab {
		return nil
	}
	absolute, err := filepath.Abs(wavPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".wav") {
		return fmt.Errorf("sidecar output requires a .wav path")
	}
	base := strings.TrimSuffix(absolute, filepath.Ext(absolute))
	if options.WriteText {
		data, encodeErr := TextBytes(options.Text, options.Encoding)
		if encodeErr != nil {
			return encodeErr
		}
		if err := atomicfile.WriteFile(base+".txt", data); err != nil {
			return fmt.Errorf("write text sidecar: %w", err)
		}
	}
	if options.WriteLab {
		data, labelErr := LabBytes(options.Lab)
		if labelErr != nil {
			return labelErr
		}
		if err := atomicfile.WriteFile(base+".lab", data); err != nil {
			return fmt.Errorf("write label sidecar: %w", err)
		}
	}
	return nil
}

// TextBytesは指定された文字コードの字幕ファイルを作る。
func TextBytes(value, encoding string) ([]byte, error) {
	return encodeText(ensureTrailingNewline(value), encoding)
}

// LabBytesは末尾を改行した音素ラベルを作る。
func LabBytes(value string) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("phoneme label is empty")
	}
	return []byte(ensureTrailingNewline(value)), nil
}

func encodeText(value, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", EncodingUTF8, "utf8":
		return []byte(value), nil
	case EncodingShiftJIS, "shift-jis", "sjis", "cp932":
		encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(value))
		if err != nil {
			return nil, fmt.Errorf("encode text as Shift_JIS: %w", err)
		}
		return encoded, nil
	default:
		return nil, fmt.Errorf("unsupported text encoding %q", encoding)
	}
}

func ensureTrailingNewline(value string) string {
	if strings.HasSuffix(value, "\n") {
		return value
	}
	return value + "\n"
}
