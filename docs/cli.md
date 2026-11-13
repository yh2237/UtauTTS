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

### ヘルプ

`--help` は用途別にまとめたフラグ一覧と代表例を標準出力へ出します。

```powershell
.\tools\utautts-cli.exe --help
```

---

## 入力

- `--text` は読み上げる文章、`--reading` は読み（かな、ARPAbet、Pinyin）を直接指定します。両方を指定すると読みが優先され、文章は `--write-text` のテキストファイルにだけ使われます。このとき標準エラーへ警告を出します。
- 本文と読みは500文字までです。HTTP APIと同じ上限で、超えると引数の誤りになります。
- `--request` は合成設定をJSONで受け取ります。`--text` などの発話設定とは併用できません。設定はJSON側のフィールドで指定します（`--prosody` なら `model_id`）。

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
| `--tone <音高>` | | 基準の音程（`C4` など）。音程ごとの原音がある音源では、近い音程の原音を使う |
| `--color <声色>` | | 音源の声色（`character.yaml` のサブバンク） |
| `--reference-blend <on\|off>` | `on` | 参照照合で近い録音の音を混ぜるか。声がこもって聞こえる音源では `off` にする |
| `--dictionary <path>` | | ユーザー辞書（JSON） |
| `--json` | `false` | 結果（読み、音声の長さ、合成単位の数など）やエラーをJSONで標準出力に出力 |

### パラメータ調整
| オプション | 既定値 | 説明 |
| :--- | :--- | :--- |
| `--mora-ms <ms>` | `120` | 基本モーラ長（ミリ秒、20〜1000） |
| `--pause-ms <ms>` | `180` | 句読点の休止時間（ミリ秒、0〜3000） |
| `--leading-preutterance-ms <ms>` | `0` | 語頭の子音の先行長（ミリ秒、0〜300。0は `oto.ini` に従う） |
| `--mora-durations <path>` | | 手動のモーラ長JSON。参照照合では、休止を含む全モーラ数と同じ要素数を指定する。休止を飛ばすと値がずれるため、要素数が合わなければエラーになる。波形合成では部分的な配列も使える |
| `--manual-pitch <path>` | | 手動ピッチJSON。`points[].position`は休止を含む全モーラ番号で、休止の位置は指定できない |
| `--phone-weights <path>` | | モーラ内の音素時間比JSON。休止を含む全モーラ番号の配列で、各要素はそのモーラの音素数と同じ長さの比を指定する |
| `--phone-lengths <path>` | | モーラごとの子音長・母音長JSON。`consonant_ms`と`vowel_ms`は休止を含む全モーラ数と同じ長さで、休止は0にする。片方だけ0にすると、その側は参照の長さを保つ |
| `--intonation-strength` | `4` | 自動抑揚の強さ（0〜8） |
| `--apply-pitch` | `true` | ピッチ補正・抑揚曲線を波形に適用するか |
| `--boundary-tone` | `true` | 文末音調（疑問文の語尾上げ等）の適用 |
| `--microprosody` | `true` | 子音前後の微小ピッチ変動の再現（日本語WORLD合成時） |
| `--timing-warp` | `true` | 日本語WORLD合成の時間を、学習した発話の動きに合わせてモーラ内で伸縮 |
| `--alias-policy` | `auto` | 原音選択ポリシー（`auto`, `cvvc-enhanced`, `vcv-prefer` 等） |

範囲のある値は設定画面と同じ表（`internal/settings`）で検証します。範囲外は引数の誤りになります。

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

# 指定した資源と依存ファイルの確認（合成はしない）
utautts-cli --doctor --voicebank "voice/足立レイver3.5.0" --prosody frame-intonation-tcn-v12
```

`--list` の種別は `voicebanks` と `models` です。`--doctor` は確認した項目を1行ずつ出し、最後に件数をまとめます。

---

## JSONリクエストファイルからの合成

HTTP APIと同じ構造のリクエストJSONを渡して合成します。音源は `voicebank_id` ではなく `--voicebank` で指定します。

```bash
utautts-cli --voicebank "voice/足立レイver3.5.0" --request request.json --out out.wav
```

---

## 終了コード

| コード | 意味 | 主なエラーコード |
| :--- | :--- | :--- |
| `0` | 成功 | |
| `1` | 内部エラー（分類できない失敗、終了処理の失敗） | `shutdown_error` |
| `2` | 引数の誤り（不明なフラグ、必須引数の不足、範囲外の値、併用できないフラグ） | `flag_parse_error`, `invalid_arguments` |
| `3` | 入力の誤り（入力ファイルが読めない、UTF-8でない、JSONが不正） | `input_error` |
| `4` | 資源の問題（音源・抑揚モデル・Open JTalkが見つからない、初期化の失敗） | `unavailable`, `initialization_error` |
| `5` | 合成の失敗（合成・タイムアウト・中断） | `synthesis_error`, `deadline_exceeded`, `canceled` |
| `6` | 出力の失敗（WAV・計画JSON・USTX・サイドカーの書き込み） | `output_error` |

`--json` を指定したときは、終了コードに加えて標準出力のJSONでも失敗を返します。

---

## `--json` の出力

`--json` を指定すると、結果・エラーをJSONで標準出力へ出します。エラーは標準エラーへ出しません。

### 成功時

```json
{
  "ok": true,
  "result": {
    "path": "out.wav",
    "reading": "あらゆるげんじつを",
    "method": "waveform-synthesis",
    "duration_ms": 1240,
    "sample_rate": 44100,
    "channels": 1,
    "unit_count": 12,
    "phoneme_labels": "0 400000 a\n...",
    "diagnostics": []
  }
}
```

| フィールド | 型 | 説明 |
| :--- | :--- | :--- |
| `ok` | bool | 成功したか |
| `result.path` | string | 出力したWAVのパス |
| `result.reading` | string | 合成に使った読み |
| `result.method` | string | 合成の方式 |
| `result.duration_ms` | number | 音声の長さ（ミリ秒） |
| `result.sample_rate` | number | サンプリング周波数 |
| `result.channels` | number | チャンネル数 |
| `result.unit_count` | number | 合成単位の数 |
| `result.phoneme_labels` | string | 音素ラベル（HTK形式） |
| `result.diagnostics` | array | 使わなかった設定などの診断 |
| `result.logs` | array | 途中経過（索引の作成など。無い場合は省略） |

`--version` は `version`、`--list` は `catalog`、`--doctor` は `doctor` を `result` の代わりに返します。

```json
{"ok": true, "version": {"name": "UtauTTS", "version": "v1.7.2"}}
{"ok": true, "catalog": {"kind": "models", "default_id": "none", "items": [], "problems": []}}
{"ok": true, "doctor": {"version": "v1.7.2", "os": "windows", "arch": "amd64", "checks": []}}
```

### 失敗時

```json
{
  "ok": false,
  "error": {
    "code": "invalid_arguments",
    "message": "--voicebank is required",
    "hint": "--voicebank <string>  path to a UTAU voicebank directory"
  }
}
```

| フィールド | 型 | 説明 |
| :--- | :--- | :--- |
| `ok` | bool | 常に `false` |
| `error.code` | string | エラーコード（終了コード表を参照） |
| `error.message` | string | 何が起きたか |
| `error.hint` | string | 次に取る手順（無い場合は省略） |

`--doctor` は失敗しても `doctor` を返します。確認した項目は残ります。
