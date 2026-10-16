package synth

import "testing"

// 同梱から外したモデルのIDは、保存済みのプロジェクトや設定のために代わりのモデルへ置き換える。
func TestRetiredModelIDsUseCurrentModel(t *testing.T) {
	for _, id := range []string{"frame-intonation-tcn-v9.1-t", "frame-intonation-tcn-v10", "frame-intonation-tcn-v11"} {
		if got := currentModelID(id); got != "frame-intonation-tcn-v12" {
			t.Fatalf("currentModelID(%q) = %q", id, got)
		}
	}
	for _, id := range []string{"frame-intonation-tcn-v12", "tone-intonation-zh-v1", "none", ""} {
		if got := currentModelID(id); got != id {
			t.Fatalf("currentModelID(%q) = %q, want unchanged", id, got)
		}
	}
}
