// Package wavfileは、合成結果のWAVを書き出す。
package wavfile

import (
	"errors"
	"io"

	"github.com/yh2237/audiodsp/wav"

	"github.com/yh2237/UtauTTS/internal/atomicfile"
)

// Writeは、書き終えるまで既存のファイルを残してWAVを書く。空のPCMは書かない。
func Write(path string, pcm *wav.PCM) error {
	if err := validate(pcm); err != nil {
		return err
	}
	if len(pcm.Data) == 0 {
		return errors.New("empty pcm data")
	}
	return atomicfile.Write(path, func(w io.Writer) error {
		return wav.Encode(w, pcm)
	})
}

// Bytesは、サンプルレートかチャンネル数が不正ならnilを返す。
func Bytes(pcm *wav.PCM) []byte {
	if err := validate(pcm); err != nil {
		return nil
	}
	data, err := wav.Bytes(pcm)
	if err != nil {
		return nil
	}
	return data
}

// validateは、チャンネル数で割り切れないサンプル数を拒む。書き出すと読み直せないWAVになるため。
func validate(pcm *wav.PCM) error {
	if pcm == nil {
		return errors.New("nil pcm")
	}
	if pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return errors.New("invalid pcm metadata")
	}
	if len(pcm.Data)%pcm.Channels != 0 {
		return errors.New("pcm data is not aligned to channels")
	}
	return nil
}
