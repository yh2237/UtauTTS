# UtauTTS Server

Windows／Linux x64向けHTTPサーバーです。

サーバーは初期状態で`127.0.0.1:8080`を待ち受けます。LANや外部から接続できるアドレスで起動する場合は必ず`--auth-token`を設定してください。

Windows

```powershell
.\utautts-server.exe `
  --voice-dir "voice" `
  --renderer waveform
```

Linux

```bash
./utautts-server --voice-dir voice --renderer waveform
```

標準では実行ファイルと同じ場所の`voice`ディレクトリを読み込みます。音源はフォルダごとに配置して`voicebank_id`には`/api/voicebanks`で取得したIDを指定します。省略するとID順で最初の音源が使われます。

## コンソールUI

ブラウザで`http://127.0.0.1:8080/`を開くとAPIを試せるコンソールUIが表示されます。

- 稼働状況・音源・モデル・Rendererの一覧表示
- 文章の解析（`/api/analyze`）と読み・モーラ列の表示
- 文章・音源・モデル・Rendererなどを指定した合成とWAVダウンロード
- `--auth-token`使用時はページ内のトークン入力へ保存すると以降のAPI呼び出しに`Authorization: Bearer <token>`を付加します

コンソールUI（`/`と`/ui`）は公開されます。認証・Origin検査は`/api/*`にのみ適用されます。

## エンドポイント一覧

| Method | Path | 説明 |
|---|---|---|
| `GET` | `/api/health` | 稼働確認 |
| `GET` | `/api/voicebanks` | 音源一覧 |
| `POST` | `/api/voicebanks` | 音源の登録（既定で無効） |
| `POST` | `/api/voicebanks/reload` | 音源ディレクトリの再読込 |
| `GET` | `/api/models` | 抑揚モデル一覧 |
| `GET` | `/api/renderers` | Renderer一覧 |
| `POST` | `/api/analyze` | 文章から読み・モーラ列への変換 |
| `POST` | `/api/synthesize/audio` | 単一発話の合成（WAV） |
| `POST` | `/api/synthesize/label` | 単一発話の音素ラベル（LAB） |
| `POST` | `/api/synthesize/batch` | 複数発話の合成（ZIP） |

## 共通仕様

- エラーは`{"error":"説明"}`のJSONと対応するHTTPステータスコードで返ります。
- JSON本文は1 MiBまでです。未知のJSON fieldは入力ミスとして拒否されます（400）。
- ボディは単一のJSONオブジェクトでなければなりません。
- 1発話の `text` と `kana` はそれぞれ500文字までです（413）。
- `manual_pitch` のpointsは最大1000個です（413）。
- batchは16発話、展開前WAV合計256 MiBまでです（413）。
- 合成は最大4並行に制限されます。超過分は空きを待って順に処理されます。

### 認証

`--auth-token`を設定すると全エンドポイントで`Authorization: Bearer <token>`ヘッダーが必要になります。無い場合は401です。

GET以外のリクエストに`Origin`ヘッダーがあり待受ホストのorigin（`http://<host>` / `https://<host>`）と一致しない場合は403で拒否します。

## 各エンドポイント

### `GET /api/health`

```json
{"status":"ok","engine":"utautts-world-phrase"}
```

`engine`には既定RendererのIDが入ります。

### `GET /api/voicebanks`

ID順にソートされた音源一覧です。

```json
{
  "voicebanks": [
    {
      "id": "足立レイver3.5.0",
      "name": "足立レイ",
      "path": "C:\\...\\voice\\足立レイver3.5.0",
      "oto_file_count": 12,
      "phoneme_count": 2640,
      "diagnostic_count": 3,
      "alias_counts": {"CV": 1200, "VCV": 1400, "VC": 40, "other": 0},
      "vcv_contexts": {"-": 50, "a": 280, "i": 270},
      "vc_contexts": {"a": 8, "i": 7},
      "has_vc": true,
      "has_initial_vcv": true,
      "has_n_context_vcv": true
    }
  ]
}
```

`id`は`voicebank_id`に指定する値で音源フォルダ名です。`phoneme_count`は全`oto.ini`のエントリ数、`diagnostic_count`はoto.iniの診断で問題があるエントリ数です。
`alias_counts`、`vcv_contexts`、`vc_contexts`は音源のalias能力を表す診断情報です。実際の各モーラではaliasの存在と`oto.ini`設定が最終的な選択を決めます。

`types`には`character.yaml`の全サブバンク（カラー、接頭辞・接尾辞、音域）が宣言順で入ります。合成するときはその`color`をリクエストの`color`へ指定できます。

### `POST /api/voicebanks`

音源パスを動的に登録します。既定では無効で`--allow-voicebank-registration`を指定した場合だけ使えます。登録先は`--voice-dir`以下に制限されてシンボリックリンク解決後に範囲外なら400で拒否します。

```json
{"name": "My Bank", "path": "voice/my-bank"}
```

成功すると登録された音源オブジェクト（`GET /api/voicebanks` と同じ形式）を返します。`name` は省略できます（省略時は音源の表示名）。

### `POST /api/voicebanks/reload`

`--voice-dir`を再走査して音源一覧を置き換えます。レスポンスは`GET /api/voicebanks`と同じ形式で失敗時は500です。

### `GET /api/models`

利用可能な抑揚モデルの一覧です。

```json
{
  "models": [
    {
      "id": "frame-intonation-v8",
      "display_name": "Frame intonation TCN v8",
      "description": "JSUT frame-level learned intonation model",
      "path": "C:\\...\\models\\frame-intonation-v8.json",
      "version": 8,
      "mode": "intonation_frame_tcn_accent_bounded",
      "outputs": {"pitch": true},
      "recommended_renderers": ["utautts-world-phrase", "openutau-worldline-r-faithful"],
      "default_priority": 100,
      "requires_features": true,
      "frame_contour": true
    }
  ]
}
```

`model_id` には `id` を指定します。

### `GET /api/renderers`

```json
{
  "default_renderer": "utautts-world-phrase",
  "renderers": [
    {"id": "utautts-world-phrase", "display_name": "UtauTTS WORLD phrase", "description": "...", "backend": "utautts-world-phrase", "capabilities": {"frame_pitch": true}, "default_priority": 300}
  ]
}
```

`default_renderer`はサーバー起動時の既定Rendererです。レスポンスの`resamplers`と`wavtools`にはClassic UTAUで選択できるツールが含まれます。

### `POST /api/analyze`

文章を読みとモーラ列へ変換します。合成前の読み確認に使います。

```json
{
  "text": "v8を使います。",
  "dictionary": [{"surface": "v8", "reading": "ぶいはち"}]
}
```

```json
{
  "reading": "コンニチハ",
  "morae": [
    {"position": 0, "mora": "コ", "consonant": "k", "vowel": "o", "pause": false},
    {"position": 1, "mora": "ン", "consonant": "n", "vowel": "n", "pause": false},
    {"position": 2, "mora": "ニ", "consonant": "n", "vowel": "i", "pause": false},
    {"position": 3, "mora": "チ", "consonant": "ch", "vowel": "i", "pause": false},
    {"position": 4, "mora": "ハ", "consonant": "h", "vowel": "a", "pause": false}
  ]
}
```

`consonant`と`vowel`はCVVC候補生成に使う文脈です。`pause`が`true`のモーラは休止になります。`text`が空なら400、500文字を超えたら413、変換に失敗すると422です。

`dictionary`はGUIのユーザー辞書と同じ表記・読みの配列です。合成リクエストにも同じ形式で指定できます。

### `POST /api/synthesize/audio`

一つの発話を合成して`audio/wav`を返します。

```json
{
  "text": "こんにちは、今日はいい天気です。",
  "voicebank_id": "足立レイver3.5.0",
  "model_id": "frame-intonation-v8",
  "renderer": "utautts-world-phrase",
  "alias_policy": "auto",
  "intonation_strength": 1,
  "apply_pitch": true
}
```

レスポンスヘッダーの`X-UtauTTS-Reading`に使用した読み、`X-UtauTTS-Engine`にRenderer IDが入ります。

リクエストフィールド：

| field | 型 | 既定値 | 説明 |
|---|---|---|---|
| `text` | string | | 日本語文章。`kana` とどちらか一方が必須 |
| `kana` | string | | 読み仮名の直接指定 |
| `voicebank_id` | string | ID順先頭 | `GET /api/voicebanks` の `id` |
| `tone` | string | `C4` | `prefix.map` 使用時の音階 |
| `color` | string | なし | `character.yaml`で定義された音源タイプ／サブバンク |
| `model_id` | string | なし | `GET /api/models` の `id` |
| `renderer` | string | 既定Renderer | `GET /api/renderers` の `id`。未知のIDは既定Rendererへ解決されます |
| `resampler` | string | 自動選択 | Classic UTAUで使うresamplerの相対ID |
| `wavtool` | string | `builtin` | Classic UTAUで使うwavtoolの相対ID |
| `alias_policy` | string | `auto` | `auto`（VC/VCV収録比から自動選択）、`legacy`（v0.0.9互換）、`cvvc-enhanced`（CVVC優先・sequential timing・VC音量35%）、`vcv-prefer`、`cvvc-prefer`、`cv-only` |
| `mora_duration_ms` | number | `140` | 基本モーラ長（0〜1000） |
| `pause_duration_ms` | number | `180` | 句読点の休止長（0〜3000） |
| `leading_preutterance_ms` | number | `0`（自動） | 文頭に確保する先行発声（0〜1000）。0では先頭原音の`oto.ini`から決定 |
| `mora_durations_ms` | number[] | | モーラごとの長さ。値は0〜1000 |
| `intonation_strength` | number | `0` | 音源ピッチ安定化と句曲線の強さ（0〜4） |
| `apply_pitch` | boolean | `false` | 波形ピッチ再サンプリング |
| `manual_pitch` | object | なし | 手動ピッチ編集（[manual-pitch.md](manual-pitch.md) のJSON） |
| `acoustic_mode` | string | なし | 音響特徴による候補選択の診断。`dry-run`または`apply` |
| `dictionary` | object[] | なし | ユーザー辞書。各項目は`surface`と`reading`を持つ |

ステータスコード：

- `200`: WAVバイナリ。`X-UtauTTS-Engine` / `X-UtauTTS-Reading` ヘッダー付き
- `400`: `text`/`kana` の両方なし、範囲外のduration・`intonation_strength`、音源・モデルが未登録、指定Rendererのasset不足（`ErrUnavailable`）
- `413`: 文字数・`manual_pitch` points超過、JSON 1 MiB超過
- `422`: 合成の失敗（読み変換失敗、モデル評価失敗、未知の`alias_policy`など）

### `POST /api/synthesize/label`

`/api/synthesize/audio`と同じJSONを受け取り、合成音声に対応するHTK形式の音素ラベルを`text/plain`で返します。レスポンスには同じ`X-UtauTTS-Engine`と`X-UtauTTS-Reading`ヘッダーが付きます。

### `POST /api/synthesize/batch`

複数発話を一つのZIPとして取得します。レスポンスは`application/zip`で`Content-Disposition: attachment; filename="utautts-audio.zip"`が付きます。

```json
{
  "write_text": true,
  "write_lab": true,
  "text_encoding": "utf-8",
  "items": [
    {
      "name": "001.wav",
      "request": {"text": "こんにちは", "voicebank_id": "足立レイver3.5.0"}
    },
    {
      "name": "002.wav",
      "request": {"kana": "オハヨウ", "voicebank_id": "足立レイver3.5.0"}
    }
  ]
}
```

`write_text`と`write_lab`を有効にすると、各WAVと同名のTXT／LABもZIPへ入ります。`text_encoding`は`utf-8`または`shift_jis`です。`name`はパス部分が除去され、`.wav`以外なら`.wav`が補われます。空や`..`なら`utterance-N.wav`に置き換えられます。重複するファイル名は400です。途中のアイテムで合成に失敗すると`item N: <error>`形式でエラーを返し、それより後ろは合成しません。

## 起動オプション

- `--voice-dir`: ボイスバンクを格納したディレクトリ
- `--renderer`: Renderer ID。省略時は設定された優先度が最も高いものを使う
- `--renderer-dir`: Renderer pluginの検索directory。複数回指定できる
- `--model-dir`: 自己記述モデルJSONの検索directory。複数回指定できてリクエストの`model_id`で選択する
- `--host`: 待受アドレス。初期値は`127.0.0.1`
- `--port`: ポート。初期値は`8080`
- `--auth-token`: 認証トークン。設定すると`Authorization: Bearer <token>`が必須になる
- `--allow-voicebank-registration`: `POST /api/voicebanks` による音源パス登録を許可する（登録先は`--voice-dir`以下に制限）
- `--worldline` / `--worldline-bridge`: worldlineライブラリとbridge実行ファイルを明示する
- `--openjtalk-features` / `--openjtalk-dictionary`: 自動検出を使わずhelperまたは辞書を明示する開発用オプション
