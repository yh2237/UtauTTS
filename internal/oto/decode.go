package oto

import "github.com/yh2237/utauio/textdecode"

func Decode(data []byte) (string, string, error) {
	return textdecode.Decode(data)
}
