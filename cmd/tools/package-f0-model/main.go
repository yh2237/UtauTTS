// 学習した統合韻律モデル（F0ヘッド付きsafetensors）を、models/へ置けるモデルJSONにまとめる。
// 基準の抑揚モデルがアクセント特徴とモーラの予測を担い、このモデルが自動ピッチ曲線とモーラの音量を担う。
// ライセンス・通知・出典は基準モデルから引き継ぐ（--licenseなどで上書きできる）。
// 使い方: go run ./cmd/tools/package-f0-model --weights out/speech-timing-target/ja-mh.safetensors --base models/frame-intonation-tcn-v10.json --id my-f0-v1 --display-name "My F0 v1" --out out/my-f0-v1.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/prosody"
	"utautts/internal/speechtiming"
)

type options struct {
	Weights, Base, ID, DisplayName, Description, License, Language, Out string
	Priority                                                             int
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
	if err := toolutil.RequireUnderOut(o.Out, "output", false); err != nil {
		return err
	}
	weights, err := os.ReadFile(o.Weights)
	if err != nil {
		return err
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
