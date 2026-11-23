// Package allは標準対応している全言語のG2P実装を既定Registryへ登録する。
package all

import (
	_ "github.com/yh2237/UtauTTS/internal/g2p/en"
	_ "github.com/yh2237/UtauTTS/internal/g2p/es"
	_ "github.com/yh2237/UtauTTS/internal/g2p/ja"
	_ "github.com/yh2237/UtauTTS/internal/g2p/pt"
	_ "github.com/yh2237/UtauTTS/internal/g2p/zh"
)
