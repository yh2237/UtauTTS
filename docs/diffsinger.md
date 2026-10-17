# DiffSinger

UtauTTSはOpenUtau仕様のDiffSinger音源に対応しています。DiffSinger音源では原音を切り出してつながず、音響モデルとボコーダーで声を作ります。

## 対応仕様
- OpenUtau形式の`dsconfig.yaml`
- 音素長の配分（`dsdur`）、ピッチの予測（`dspitch`）、声質の予測（`dsvariance`）
- 共有ボコーダー（`Dependencies/<ボコーダー名>`）
- 連続・離散のDiffusion

GPUでの実行と話者の混合（Speaker Mix）には対応していません。Windows x64のCPUでのみ動きます。

---

## 使い方

1. DiffSinger音源のフォルダを、UTAU音源と同じく`voice/`に置きます。
2. 共有ボコーダーを使う音源では、ボコーダーを`Dependencies/<ボコーダー名>`に置きます。
3. GUIで音源を選ぶと、Rendererが自動で`diffsinger`に切り替わります。
4. 編集画面ではUTAU音源と同じく音高と長さを調整できます。

---

## 主な調声パラメータ

| パラメータ | CLIオプション | 説明 |
| :--- | :--- | :--- |
| 拡散ステップ数 | `--diffsinger-steps` | 0は既定の値を使います。 |
| 長さ混合率 | `--diffsinger-duration-mix` | 音節の中の子音と母音の配分（0〜1）。上げると音源の予測を、下げるとUtauTTSの予測を優先します。 |
| ピッチ混合率 | `--diffsinger-pitch-mix` | ピッチの予測の配分（0〜1）。上げると音源のピッチの予測を、下げるとUtauTTSの話し声の抑揚を優先します。 |
| 表現力 | `--diffsinger-expr` | 下げるとビブラートや息遣いが弱まります。既定値は`1.0`です。 |
