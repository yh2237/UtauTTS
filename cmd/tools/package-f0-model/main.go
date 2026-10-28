// 学習済みF0ヘッド（safetensors）を基準モデルと合わせ、配布用JSONを作る。
// 基準モデルがアクセント特徴とモーラ予測を、F0ヘッドが自動ピッチ曲線とモーラ音量を担う。
// ライセンス・通知・出典は基準モデルから引き継ぐ（--licenseなどで上書きできる）。
// --baseは基準モデルJSONか、それを埋め込んだモデルJSON。後者は埋込みの基準モデルを使う。
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

	"github.com/yh2237/UtauTTS/cmd/tools/internal/toolutil"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauWLD/speechtiming"
)

type options struct {
	Weights, Base, ID, DisplayName, Description, License, Language, Out string
	Priority                                                            int
	BaseBlend, PitchOffsetCents                                         float64
	UseEnergy, NaturalScale, EmbedBase                                  bool
}

func main() {
	var o options
	flag.StringVar(&o.Weights, "weights", "", "F0ヘッド付きsafetensors")
	flag.StringVar(&o.Base, "base", "", "基準の抑揚モデルJSON、または基準モデルを埋め込んだモデルJSON")
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
	flag.BoolVar(&o.EmbedBase, "embed-base", false, "基準モデルを出力JSONへ埋め込む（基準モデルを別に配布しない場合）。--baseが埋め込みのモデルなら常に埋め込む")
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
	baseData, embedded, err := readBaseModel(o.Base)
	if err != nil {
		return err
	}
	base, err := prosody.ParseModel(baseData)
	if err != nil {
		return fmt.Errorf("load base model: %w", err)
	}
	license := o.License
	if license == "" {
		license = base.License
	}
	manifest := map[string]any{
		"id": o.ID, "display_name": o.DisplayName, "description": o.Description,
		"license": license, "license_notices": base.LicenseNotices, "provenance": base.Provenance,
		"language": o.Language, "default_priority": o.Priority,
		"f0_head": weights, "base_blend": o.BaseBlend, "use_energy": o.UseEnergy,
	}
	if o.EmbedBase || embedded {
		manifest["base_model_data"] = json.RawMessage(baseData)
	} else {
		manifest["base_model"] = filepath.Base(o.Base)
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
	if o.EmbedBase || embedded {
		fmt.Printf("wrote %s (base model embedded)\n", o.Out)
	} else {
		fmt.Printf("wrote %s (base %s); place it next to the base model in models/\n", o.Out, filepath.Base(o.Base))
	}
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

// readBaseModelは基準モデルのJSONを返す。基準モデルを埋め込んだモデルJSONなら、埋め込んだ基準モデルを返す。
func readBaseModel(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var manifest struct {
		BaseModel     string          `json:"base_model"`
		BaseModelData json.RawMessage `json:"base_model_data"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false, fmt.Errorf("read base model: %w", err)
	}
	if len(manifest.BaseModelData) > 0 {
		return manifest.BaseModelData, true, nil
	}
	if manifest.BaseModel != "" {
		return nil, false, fmt.Errorf("base model must be a plain intonation model or embed its base model")
	}
	return data, false, nil
}
