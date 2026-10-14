# コマンドライン（CLI）

`tools/utautts-cli`（Windows環境では `utautts-cli.exe`）を使用することで、ターミナルや外部プログラムから音声合成を実行できます。

## 基本コマンド

### 最小構成での合成
```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --out ".\out.wav"
```

### 抑揚モデルを指定した合成
```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --prosody frame-intonation-tcn-v12 `
  --out ".\out.wav"
```

- `--out -` と指定すると、合成完了後にWAV全体を標準出力（stdout）へ出力します。合成途中のストリーミングではありません。完了表示は標準エラーへ出します。
- テキストは `--text-file <path>` や標準入力（`-`）からも読み込めます。

`--out -` と `--json`、`--write-text`、`--write-lab` は併用できません。Windows PowerShell 5.1など、外部コマンドの出力を文字列へ変換する環境ではWAVのリダイレクトを避け、`--out out.wav` で直接保存してください。リダイレクトには、バイナリ出力を保つPowerShell 7.4以降などを使います。

---

## 主なオプション一覧

### 基本オプション
| オプション | 既定値 | 説明 |
| :--- | :--- | :--- |
| `--voicebank <dir>` | (必須) | 使用するボイスバンクのディレクトリパス |
| `--out <path>` | (必須) | 出力先WAVファイルパス（`-` で標準出力） |
| `--text <文字列>` | | 読み上げる文章（テキスト解析により自動読み上げ） |
| `--reading <読み>` | | 読み（かな、ARPAbet、Pinyin）を直接指定（`--kana` も可） |
| `--text-file <path>` | | 入力テキストファイルパス（`-` で標準入力） |
| `--prosody <id>` | | 抑揚モデルID（例: `frame-intonation-tcn-v12`） |
| `--renderer <id>` | カタログの既定Renderer | 使用する合成エンジンID。省略時は `default_priority` が最大のRendererを選択。|
| `--language <id>` | `ja` | 言語指定（`ja`, `en`, `zh`） |
| `--phonemizer <id>` | 自動 | 発音形式（`ja-kana`, `en-arpasing`, `zh-cvvc` 等） |
| `--dictionary <path>` | | ユーザー辞書JSONファイルパス |
| `--json` | `false` | 結果詳細（実行時間、音素数等）やエラーをJSONで標準出力に出力 |

### パラメータ調整
| オプション | 既定値 | 説明 |
| :--- | :--- | :--- |
| `--mora-ms <ms>` | `120` | 基本モーラ長（ミリ秒） |
| `--pause-ms <ms>` | `180` | 句読点による休止時間（ミリ秒） |
| `--intonation-strength` | `4` | 自動抑揚の強さ（0〜8、4が標準） |
| `--apply-pitch` | `true` | ピッチ補正・抑揚曲線を波形に適用するか |
| `--boundary-tone` | `true` | 文末音調（疑問文の語尾上げ等）の適用 |
| `--microprosody` | `true` | 子音前後の微小ピッチ変動の再現（日本語WORLD合成時） |
| `--timing-warp` | `true` | モーラ内音素長バランスの最適化 |
| `--alias-policy` | `auto` | 原音選択ポリシー（`auto`, `cvvc-enhanced`, `vcv-prefer` 等） |

### 追加出力
| オプション | 説明 |
| :--- | :--- |
| `--plan-out <path>` | 原音選択・配置タイミング情報を記録した中間計画JSONを出力 |
| `--ustx-out <path>` | 合成パラメータをOpenUtauのプロジェクト形式（`.ustx`）で書き出し |
| `--write-text` | WAVと同名のテキストファイル（`.txt`）を出力 |
| `--write-lab` | WAVと同名の音素ラベルファイル（`.lab`）を出力 |

---

## 診断・一覧表示コマンド

```bash
# 認識されている音源・モデル・Rendererを一覧表示
utautts-cli --list voicebanks
utautts-cli --list models --json
utautts-cli --list renderers

# 実行環境や依存バイナリの整合性チェック
utautts-cli --doctor --voicebank "voice/足立レイver3.5.0" --prosody frame-intonation-tcn-v12
```

---

## JSONリクエストファイルからの合成

HTTP APIと同じリクエスト構造を持つJSONファイルを渡して合成できます。

```bash
utautts-cli --voicebank "voice/足立レイver3.5.0" --request request.json --out out.wav
```
