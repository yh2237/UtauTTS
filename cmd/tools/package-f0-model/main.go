// 学習した統合韻律モデル（F0ヘッド付きsafetensors）を、models/へ置けるモデルJSONにまとめる。
// 基準の抑揚モデルがアクセント特徴とモーラの予測を担い、このモデルが自動ピッチ曲線とモーラの音量を担う。
// ライセンス・通知・出典は基準モデルから引き継ぐ（--licenseなどで上書きできる）。
// 使い方: go run ./cmd/tools/package-f0-model --weights out/speech-timing-target/ja-mh.safetensors --base models/frame-intonation-tcn-v10.json --id my-f0-v1 --display-name "My F0 v1" --out out/my-f0-v1.json
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/prosody"
	"utautts/internal/speechtiming"
)

type options struct {
	Weights, Base, ID, DisplayName, Description, License, Language, Out string
	Priority                                                            int
	BaseBlend, PitchOffsetCents                                         float64
	UseEnergy, NaturalScale                                             bool
}

func main() {
	var o options
	flag.StringVar(&o.Weights, "weights", "", "F0ヘッド付きsafetensors")
	flag.StringVar(&o.Base, "base", "", "基準の抑揚モデルJSON（出力と同じディレクトリに置くファイル名で参照する）")
	flag.StringVar(&o.ID, "id", "", "モデルID")
	flag.StringVar(&o.DisplayName, "display-name", "", "表示名")
	flag.StringVar(&o.Description, "description", "", "説明")
	flag.StringVar(&o.License, "license", "", "モデルのライセンス（空は基準モデルと同じ）")
	flag.StringVar(&o.Language, "language", "ja", "対象言語")
	flag.IntVar(&o.Priority, "priority", 0, "default_priority（基準より小さくすると既定にならない）")
	flag.Float64Var(&o.BaseBlend, "base-blend", 0, "基準モデルの抑揚曲線を混ぜる重み（0〜1）")
	flag.BoolVar(&o.UseEnergy, "use-energy", true, "エネルギーヘッドでモーラの音量を変える")
	flag.Float64Var(&o.PitchOffsetCents, "pitch-offset-cents", 0, "自動ピッチ曲線全体へ足す高さ（セント）")
	flag.BoolVar(&o.NaturalScale, "natural-scale", false, "メタデータのf0_scaleを外す（--f0-teacherに自然スケールの単位で渡した場合）")
	flag.StringVar(&o.Out, "out", "", "出力JSON（out/以下）")
	flag.Parse()
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(o options) error {
	if o.Weights == "" || o.Base == "" || o.ID == "" || o.DisplayName == "" || o.Out == "" {
		return fmt.Errorf("weights, base, id, display-name and out are required")
	}
	if o.BaseBlend < 0 || o.BaseBlend > 1 {
		return fmt.Errorf("base-blend must be between 0 and 1")
	}
	if math.Abs(o.PitchOffsetCents) > 1200 {
		return fmt.Errorf("pitch-offset-cents must be between -1200 and 1200")
	}
	if err := toolutil.RequireUnderOut(o.Out, "output", false); err != nil {
		return err
	}
	weights, err := os.ReadFile(o.Weights)
	if err != nil {
		return err
	}
	if o.NaturalScale {
		if weights, err = withoutF0Scale(weights); err != nil {
			return err
		}
	}
	head, err := speechtiming.LoadTCN(weights)
	if err != nil {
		return fmt.Errorf("load weights: %w", err)
	}
	if !head.HasF0Head() {
		return fmt.Errorf("%s has no F0 head", o.Weights)
	}
	base, err := prosody.LoadModel(o.Base)
	if err != nil {
		return fmt.Errorf("load base model: %w", err)
	}
	if base.F0Head != nil {
		return fmt.Errorf("base model must be a plain intonation model")
	}
	license := o.License
	if license == "" {
		license = base.License
	}
	manifest := map[string]any{
		"id": o.ID, "display_name": o.DisplayName, "description": o.Description,
		"license": license, "license_notices": base.LicenseNotices, "provenance": base.Provenance,
		"language": o.Language, "default_priority": o.Priority,
		"base_model": filepath.Base(o.Base), "f0_head": weights,
		"base_blend": o.BaseBlend, "use_energy": o.UseEnergy,
	}
	if o.PitchOffsetCents != 0 {
		manifest["pitch_offset_cents"] = o.PitchOffsetCents
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(o.Out), 0o755); err != nil {
		return err
	}
	file, err := toolutil.CreateExclusive(o.Out)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Printf("wrote %s (base %s); place it next to the base model in models/\n", o.Out, filepath.Base(o.Base))
	return nil
}

// withoutF0Scaleはsafetensorsのメタデータからf0_scaleを外す（推論は自然スケールの経路になる）。
func withoutF0Scale(weights []byte) ([]byte, error) {
	if len(weights) < 8 {
		return nil, fmt.Errorf("weights too short")
	}
	size := binary.LittleEndian.Uint64(weights[:8])
	if size > uint64(len(weights)-8) {
		return nil, fmt.Errorf("invalid safetensors header")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(weights[8:8+size], &header); err != nil {
		return nil, fmt.Errorf("read safetensors header: %w", err)
	}
	var metadata map[string]string
	if raw, ok := header["__metadata__"]; ok {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, fmt.Errorf("read safetensors metadata: %w", err)
		}
	}
	delete(metadata, "f0_scale")
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	header["__metadata__"] = raw
	encoded, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	if pad := (8 - len(encoded)%8) % 8; pad > 0 {
		encoded = append(encoded, bytes.Repeat([]byte(" "), pad)...)
	}
	result := binary.LittleEndian.AppendUint64(nil, uint64(len(encoded)))
	result = append(result, encoded...)
	return append(result, weights[8+size:]...), nil
}
