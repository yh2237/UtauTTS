package synth

import (
	"errors"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plugin"
)

// TestRetiredModelIDsAreReportedは、引退したモデルの読み替えを表として渡すことを確かめる。
// 読み替えるのはプロジェクトを持つGUIの役目なので、コアでは置き換えない。
func TestRetiredModelIDsAreReported(t *testing.T) {
	retired := RetiredModelIDs()
	for _, id := range []string{"frame-intonation-tcn-v9.1-t", "frame-intonation-tcn-v10", "frame-intonation-tcn-v11"} {
		if got := retired[id]; got != "frame-intonation-tcn-v12" {
			t.Fatalf("RetiredModelIDs()[%q] = %q", id, got)
		}
	}
	if _, ok := retired["frame-intonation-tcn-v12"]; ok {
		t.Fatal("a current model must not be listed as retired")
	}
	// 呼出し側が表を書き換えても、元の表は変わらない。
	retired["frame-intonation-tcn-v12"] = "broken"
	if _, ok := RetiredModelIDs()["frame-intonation-tcn-v12"]; ok {
		t.Fatal("RetiredModelIDs returned the shared map")
	}
}

// TestResolveModelRejectsRetiredIDは、引退したモデルの指定をエラーにすることを確かめる。
func TestResolveModelRejectsRetiredID(t *testing.T) {
	service := NewService(&plugin.Catalog{}, "waveform", "", "", "", nil)
	_, err := service.ResolveModel("frame-intonation-tcn-v10")
	if err == nil {
		t.Fatal("a retired model id was accepted")
	}
	if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "frame-intonation-tcn-v12") {
		t.Fatalf("error should name the replacement: %v", err)
	}
}
