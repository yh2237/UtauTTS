package synth

import (
	"testing"

	"utautts/internal/render/base"
)

// 補正の設定キーがrenderer設定テーブルに存在することを確認する。
func TestCorrectionSettingsExist(t *testing.T) {
	for _, correction := range base.Corrections {
		if correction.Setting == "" {
			continue
		}
		if _, found := rendererSettingSpecFor(rendererSettingSpecs, correction.Setting); !found {
			t.Errorf("correction %q setting %q is not in rendererSettingSpecs", correction.ID, correction.Setting)
		}
	}
}
