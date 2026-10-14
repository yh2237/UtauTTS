# DiffSinger

UtauTTSは、OpenUtau仕様のDiffSinger音源パッケージに対応しています。原音の切り出し・接続を行わず、音響モデルとボコーダーを用いたニューラル音声合成を行います。

## 対応仕様
- OpenUtau形式の `dsconfig.yaml`
- 音素長配分（`dsdur`）、ピッチ予測（`dspitch`）、声質予測（`dsvariance`）
- 共有ボコーダー（`Dependencies/<ボコーダー名>`）
- 連続・離散 Diffusion

※現在の制約: GPUアクセラレーションおよび話者混合（Speaker Mix）には対応していません（Windows x64でのCPU実行のみ）。

---

## 使い方

1. 通常のUTAU音源と同様に、`voice/` 配下にDiffSinger音源フォルダを配置します。
2. 共有ボコーダーが必要な音源の場合は、`Dependencies/<ボコーダー名>` に配置します。
3. GUIで音源を選択すると、Rendererが自動的に `diffsinger` に切り替わります。
4. タイムライン上のエディタで、通常の音源と同様に音高や長さを調整できます。

---

## 主な調声パラメータ

| パラメータ | CLIオプション | 説明 |
| :--- | :--- | :--- |
| **拡散ステップ数** | `--diffsinger-steps` | 拡散ステップ数を変更します。 |
| **長さ混合率** | `--diffsinger-duration-mix` | 音節内の子音・母音の配分（0〜1）。上げると音源側、下げるとUtauTTSの予測を優先します。 |
| **ピッチ混合率** | `--diffsinger-pitch-mix` | ピッチの予測配分（0〜1）。上げると音源側のピッチ予測、下げるとUtauTTSの話声抑揚を優先します。 |
| **表現力** | `--diffsinger-expr` | 下げるとビブラートや息遣いが弱まります。既定値は `1.0` です。 |
