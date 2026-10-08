package synth

import (
	"testing"

	"utautts/internal/plan"
	"utautts/internal/settings"
)

// 設定表と適用テーブルが同じ設定を持ち、既定値が一致することを確認する。
func TestSettingsTableMatchesSpecs(t *testing.T) {
	for _, setting := range settings.Table {
		spec, found := rendererSettingSpecFor(rendererSettingSpecs, setting.ID)
		if !found {
			t.Errorf("setting %q has no spec", setting.ID)
			continue
		}
		if spec.defaultValue != setting.Default {
			t.Errorf("setting %q default = %v in specs, %v in table", setting.ID, spec.defaultValue, setting.Default)
		}
	}
	for _, spec := range rendererSettingSpecs {
		if _, found := settings.Lookup(spec.id); !found {
			t.Errorf("spec %q is not in the settings table", spec.id)
		}
	}
}

// planの内部既定（設定が無い場合の補完）が設定表と同じであることを確認する。
func TestPlanDefaultsMatchSettingsTable(t *testing.T) {
	if plan.DefaultMoraDurationMS != settings.Number("mora_duration_ms") || plan.DefaultPauseDurationMS != settings.Number("pause_duration_ms") {
		t.Fatalf("plan defaults %v/%v differ from the settings table", plan.DefaultMoraDurationMS, plan.DefaultPauseDurationMS)
	}
}
