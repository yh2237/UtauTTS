package aviutl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/yh2237/UtauIDX/audio"
)

const (
	exoAudioRate = 48000
	exoAudioCh   = 2
	exoWidth     = 1920
	exoHeight    = 1080
)

func WriteExo(path string, wavPaths []string, frameRate int) error {
	if len(wavPaths) == 0 {
		return fmt.Errorf("no WAV files to arrange")
	}
	if frameRate <= 0 || frameRate > 240 {
		return fmt.Errorf("invalid frame rate %d", frameRate)
	}
	type clip struct {
		path   string
		frames int
	}
	clips := make([]clip, 0, len(wavPaths))
	totalFrames := 0
	for _, wavPath := range wavPaths {
		pcm, err := audio.ReadWav(wavPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", wavPath, err)
		}
		if pcm.SampleRate <= 0 || pcm.Channels <= 0 {
			return fmt.Errorf("read %s: invalid WAV metadata", wavPath)
		}
		frames := int(float64(len(pcm.Data)/pcm.Channels) * float64(frameRate) / float64(pcm.SampleRate))
		if frames <= 0 {
			frames = 1
		}
		clips = append(clips, clip{path: filepath.Clean(wavPath), frames: frames})
		totalFrames += frames
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "[exedit]\nwidth=%d\nheight=%d\nrate=%d\nscale=1\nlength=%d\naudio_rate=%d\naudio_ch=%d\n",
		exoWidth, exoHeight, frameRate, totalFrames, exoAudioRate, exoAudioCh)
	cursor := 0
	for index, clip := range clips {
		start := cursor + 1
		end := start + clip.frames - 1
		fmt.Fprintf(&builder, "[%d]\nstart=%d\nend=%d\nlayer=1\noverlay=1\naudio=1\n", index, start, end)
		fmt.Fprintf(&builder, "[%d.0]\n_name=音声ファイル\n再生位置=0.00\n再生速度=100.0\nループ再生=0\n動画ファイルと連携=0\nfile=%s\n", index, toExoPath(clip.path))
		fmt.Fprintf(&builder, "[%d.1]\n_name=標準再生\n音量=100.0\n左右=0.0\n", index)
		cursor = end
	}

	encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(builder.String()))
	if err != nil {
		return fmt.Errorf("encode exo: %w", err)
	}
	return os.WriteFile(path, encoded, 0o644)
}

func toExoPath(path string) string {
	return strings.ReplaceAll(filepath.Clean(path), "/", "\\")
}
