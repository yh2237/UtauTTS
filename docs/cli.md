# コマンドライン（CLI）

`tools/utautts-cli`（Windowsでは `utautts-cli.exe`）で、ターミナルや外部プログラムから音声合成を実行します。

## 基本コマンド

### 最小構成での合成
```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --method waveform-synthesis `
  --out ".\out.wav"
```

### 抑揚モデルを指定した合成
```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --method waveform-synthesis `
  --prosody frame-intonation-tcn-v12 `
  --out ".\out.wav"
```

- `--out -` を指定すると、合成が終わってからWAV全体を標準出力へ書き出します。完了表示は標準エラーへ出します。
- テキストは `--text-file <path>` でファイルから、`--text-file -` で標準入力から読み込めます。

`--out -` は `--json`、`--write-text`、`--write-lab` と併用できません。Windows PowerShell 5.1など、外部コマンドの出力を文字列に変換する環境ではWAVをリダイレクトせず、`--out out.wav` で直接保存してください。リダイレクトにはバイナリ出力を保つPowerShell 7.4以降などを使います。

---

## 主なオプション一覧

### 基本オプション
| オプション | 既定値 | 説明 |
| :--- | :--- | :--- |
| `--voicebank <dir>` | (必須) | 音源のディレクトリ |
| `--out <path>` | (必須) | 出力するWAVのパス（`-` で標準出力） |
| `--method <id>` | (必須) | 合成の方式（`waveform-synthesis` で波形合成、`reference-matching` で参照照合） |
| `--text <文字列>` | | 読み上げる文章 |
| `--reading <読み>` | | 読み（かな、ARPAbet、Pinyin）を直接指定（`--kana` も可） |
| `--text-file <path>` | | 入力テキストファイル（`-` で標準入力） |
| `--prosody <id>` | | 抑揚モデルID（例: `frame-intonation-tcn-v12`） |
| `--language <id>` | `ja` | 言語（`ja`, `en`, `zh`, `es`, `pt`） |
| `--phonemizer <id>` | 自動 | 発音形式（`ja-kana`, `en-arpasing`, `zh-cvvc` 等） |
| `--dictionary <path>` | | ユーザー辞書（JSON） |
| `--json` | `false` | 結果（読み、音声の長さ、合成単位の数など）やエラーをJSONで標準出力に出力 |

### パラメータ調整
| オプション | 既定値 | 説明 |
| :--- | :--- | :--- |
| `--mora-ms <ms>` | `120` | 基本モーラ長（ミリ秒） |
| `--pause-ms <ms>` | `180` | 句読点の休止時間（ミリ秒） |
| `--intonation-strength` | `4` | 自動抑揚の強さ（0〜8） |
| `--apply-pitch` | `true` | ピッチ補正・抑揚曲線を波形に適用するか |
| `--boundary-tone` | `true` | 文末音調（疑問文の語尾上げ等）の適用 |
| `--microprosody` | `true` | 子音前後の微小ピッチ変動の再現（日本語WORLD合成時） |
| `--timing-warp` | `true` | 日本語WORLD合成の時間を、学習した発話の動きに合わせてモーラ内で伸縮 |
| `--alias-policy` | `auto` | 原音選択ポリシー（`auto`, `cvvc-enhanced`, `vcv-prefer` 等） |

### 追加出力
| オプション | 説明 |
| :--- | :--- |
| `--plan-out <path>` | 原音の選択と配置の時刻を記録した合成計画JSONを出力 |
| `--ustx-out <path>` | 合成パラメータをOpenUtauのプロジェクト形式（`.ustx`）で書き出し |
| `--write-text` | WAVと同名のテキストファイル（`.txt`）を出力 |
| `--write-lab` | WAVと同名の音素ラベルファイル（`.lab`）を出力 |

---

## 診断・一覧表示コマンド

```bash
# 音源・モデルの一覧
utautts-cli --list voicebanks
utautts-cli --list models --json

# 指定した資源と依存ファイルの確認（ヘルパーの起動と合成はしない）
utautts-cli --doctor --voicebank "voice/足立レイver3.5.0" --prosody frame-intonation-tcn-v12
```

---

## JSONリクエストファイルからの合成

HTTP APIと同じ構造のリクエストJSONを渡して合成します。音源は `voicebank_id` ではなく `--voicebank` で指定します。

```bash
utautts-cli --voicebank "voice/足立レイver3.5.0" --request request.json --out out.wav
```
