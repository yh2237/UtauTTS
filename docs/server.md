# UtauTTS Server

同じ端末のアプリからUtauTTSの合成機能をHTTP APIで利用するためのサーバーです。Windows、Linux x64、macOS arm64に対応しています。

既定では`127.0.0.1:8080`を待ち受けます。ローカル専用で、LANや外部アドレスへの待受はできません。外部公開が必要な場合は、本体とは別のアプリで認証・アクセス制御を担当してください。

Windows

```powershell
.\utautts-server.exe `
  --voice-dir "voice" `
  --renderer utautts-world-phrase
```

Linux

```bash
./utautts-server --voice-dir voice --renderer utautts-world-phrase
```

macOS

```bash
xattr -rc "utautts-server" runtime
./utautts-server --voice-dir voice --renderer utautts-world-phrase
```

標準では実行ファイルと同じ場所の`voice`ディレクトリを読み込みます。音源はフォルダごとに配置し、`voicebank_id`には`/api/voicebanks`で取得したIDを指定します。省略するとID順で最初の音源が使われます。

## コンソールUI

ブラウザで`http://127.0.0.1:8080/`を開くとAPIを試せるコンソールUIが表示されます。

- 稼働状況・音源・モデル・Rendererの一覧表示
- 文章の解析（`/api/analyze`）と読み・モーラ列の表示
- 文章・音源・モデル・Rendererなどを指定した合成とWAVダウンロード
- 言語・発音形式・読みの指定、音素ラベルの表示・ダウンロード、一括合成ZIPのダウンロード
- 合成リクエストの詳細JSON指定、許可されたサーバーでの音源登録

コンソールUIは`/`で提供されます（`/ui`は`/`への307リダイレクトです）。トークンの入力は不要です。

## エンドポイント一覧

| Method | Path | 説明 |
|---|---|---|
| `GET` | `/api/health` | 稼働確認 |
| `GET` | `/api/limits` | 適用中の運用制限 |
| `GET` | `/api/openapi.yaml` | OpenAPI仕様（YAML） |
| `GET` | `/api/voicebanks` | 音源一覧 |
| `POST` | `/api/voicebanks` | 音源の登録（既定で無効） |
| `POST` | `/api/voicebanks/reload` | 音源ディレクトリの再読込 |
| `GET` | `/api/models` | 抑揚モデル一覧 |
| `GET` | `/api/renderers` | Renderer一覧 |
| `POST` | `/api/analyze` | 文章から読み・モーラ列への変換 |
| `POST` | `/api/synthesize/audio` | 単一発話の合成（WAV） |
| `POST` | `/api/synthesize/result` | 単一発話の音声と結果・診断（JSON） |
| `POST` | `/api/synthesize/label` | 単一発話の音素ラベル（LAB） |
| `POST` | `/api/synthesize/batch` | 複数発話の合成（ZIP） |

## 共通仕様

- 各API処理のエラーは`{"error":"説明","code":"分類"}`のJSONと対応するHTTPステータスで返ります。既存の`error`は文字列のままです。未定義パス・未対応メソッドの404・405はHTTPルーターの応答です。
- JSON本文は既定で1 MiBまでです。未知のJSON fieldは入力ミスとして拒否されます（400）。
- ボディは単一のJSONオブジェクトでなければなりません。
- 1発話の`text`と有効な読み（`reading`、無ければ`kana`）は既定でそれぞれ500文字までです（413）。
- `manual_pitch`のpointsは既定で最大1000個です（413）。
- `mora_durations_ms`と`resampler_expressions`は既定で最大500要素です（413）。
- batchは既定で16発話、非圧縮WAV合計256 MiBまでです（413）。
- 合成・読み解析は既定で合計4並行、batchはさらに最大2並行です。超過分は空きを待ちます。待機順の保証はありません。

これらの上限は[起動オプション](#起動オプション)で変更でき、適用中の値は`/api/limits`で取得できます。

### OpenAPI

[OpenAPI 3.0.3のYAML](../internal/api/openapi.yaml)に、HTTPの入力・既定値・応答・エラーを記録しています。サーバーからは`GET /api/openapi.yaml`で同じ内容を取得できます。

```sh
curl http://127.0.0.1:8080/api/openapi.yaml -o openapi.yaml
```

仕様の`info.version`は`development`で、アプリケーションのバージョンは`/api/health`で取得します。上限を表す`x-default-*`は既定の運用設定です。固定の`maxLength`・`maxItems`制約にはせず、現在の上限は`/api/limits`を参照します。

入力の`default`はJSONデコード時の値です。合成時に空・ゼロから補う値とは区別しています。例えば`tone`の空文字は合成時にC4となり、`release_set`が偽なら`release_ms`のデコード値0を使わず既定の20 msになります。通常入力・編集・互換入力を同じ要求スキーマに含め、任意の`renderer_settings`も受け取ります。

OpenAPIの追加によって入力の検証規則は変えていません。音源・資源の不足やモーラ位置などは実際の要求処理でも検証します。

### ローカル接続

`--host`には`localhost`かloopback IP（`127.0.0.1`・`::1`など）だけを指定できます。`localhost`はDNSを使わず`127.0.0.1`で待ち受けます。空文字・`0.0.0.0`・LAN IP・外部ホスト名は起動時に拒否します。

UIとAPIの`Host`も`localhost`かloopback IPに限定します。別サイトからのPOSTなどの操作要求は、Go標準の`CrossOriginProtection`による`Sec-Fetch-Site`・`Origin`検査で拒否します。同じサーバーのUI操作と、これらのヘッダーを付けないローカルアプリの要求は受け付けます。拒否時は403の`forbidden`を返します。

これは別サイトからの意図しない操作への保護で、同じ端末のアプリ間の認証ではありません。`--auth-token`とトークン管理UIは削除しました。以前の起動設定にこのフラグがある場合は取り除いてください。

### エラー分類

| `code` | 内容・HTTPステータス |
| --- | --- |
| `invalid_request` | JSON・必須項目・範囲・ファイル名などの入力不正（400） |
| `limit_exceeded` | 本文・文字数・編集要素・一括合成の上限超過（413） |
| `unavailable` | 音源・モデル・Renderer等の不足（従来どおり400）、終了済みRuntime（503） |
| `synthesis_error` | 読み解析・合成の失敗（422） |
| `canceled`・`deadline_exceeded` | 要求のキャンセル・期限切れ（408） |
| `queue_timeout` | 処理枠の待機時間超過（503） |
| `forbidden` | 非ローカルのHost、別サイトからのブラウザ操作、登録許可のエラー（403） |
| `internal_error` | 再読込・一時ファイル・ZIP出力などの内部エラー（500） |

一括合成で発話が失敗した場合も同じ分類を使い、`error`には`item N: ...`を残します。応答本文の書込が始まった後の通信エラーはJSONへ切り替えず、ログへ記録します。クライアントが切断した場合はエラーJSONを受け取れないことがあります。

## 各エンドポイント

### `GET /api/health`

```json
{"status":"ok","engine":"utautts-world-phrase","version":"1.5.1"}
```

`engine`には既定RendererのID、`version`にはサーバーのバージョンが入ります。これは稼働確認で、合成資源がなくても200を返します。既定Rendererの資源の利用可否は`/api/renderers`、音源の登録状況は`/api/voicebanks`で確認できます。

### `GET /api/limits`

適用中の制限を返します。バイト数・件数・並列数は整数、時間は`30s`などの文字列です。

```json
{
  "max_json_request_bytes": 1048576,
  "max_text_runes": 500,
  "max_batch_items": 16,
  "max_batch_wav_bytes": 268435456,
  "max_manual_pitch_points": 1000,
  "max_concurrent_synthesis": 4,
  "max_concurrent_batches": 2,
  "queue_timeout": "0s",
  "synthesis_timeout": "0s"
}
```

`queue_timeout`は処理枠ごとの最大待機時間です。`synthesis_timeout`は要求のJSON読込後の解析・合成に適用し、処理枠の待機時間も含みます。一括合成では全発話を通して一つの期限を使い、発話ごとにはリセットしません。時間の`0s`は制限なしです。

期限切れ後も要求と資源の終了を待ちます。本文読込や同期I/O、Runtime再読込の待機、応答書込を強制的に打ち切る設定ではありません。ネットワーク側には既存のread timeout 30秒・write timeout 10分が別途適用されます。並列数やWAV合計の制限はプロセスのメモリ使用量の上限を保証しません。

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

`id`は`voicebank_id`に指定する値で、`--voice-dir`からの相対パスです（入れ子の音源では`a/b`のようにスラッシュ区切りになります）。`phoneme_count`は全`oto.ini`のエントリ数、`diagnostic_count`はoto.iniの診断で問題があるエントリ数です。
`alias_counts`、`vcv_contexts`、`vc_contexts`は音源のalias能力を表す診断情報です。実際の各モーラではaliasの存在と`oto.ini`設定が最終的な選択を決めます。

`types`には`character.yaml`の全サブバンク（カラー、接頭辞・接尾辞、音域）が宣言順で入ります。合成するときはその`color`をリクエストの`color`へ指定できます。

### `POST /api/voicebanks`

音源パスを動的に登録します。既定では無効で、`--allow-voicebank-registration`を指定した場合だけ使えます。登録先は`--voice-dir`以下に制限され、シンボリックリンクを解決した後に範囲外になるパスは400で拒否します。

```json
{"name": "My Bank", "path": "voice/my-bank"}
```

成功すると登録された音源オブジェクト（`GET /api/voicebanks` と同じ形式）を返します。`name` は省略できます（省略時は音源の表示名）。

### `POST /api/voicebanks/reload`

`--voice-dir`を再走査して音源一覧を置き換えます。レスポンスは`GET /api/voicebanks`と同じ形式で、失敗時は500です。

### `GET /api/models`

利用可能な抑揚モデルの一覧です。

```json
{
  "models": [
    {
      "id": "frame-intonation-tcn-v12",
      "display_name": "Frame Intonation TCN v12",
      "description": "...",
      "language": "ja",
      "path": "C:\\...\\models\\frame-intonation-tcn-v12.json",
      "version": 8,
      "feature_version": 1,
      "mode": "intonation_frame_tcn_accent_bounded",
      "sha256": "<モデルファイルのSHA-256>",
      "recommended_renderers": ["utautts-world-phrase"],
      "default_priority": 130,
      "requires_features": true,
      "frame_contour": true
    }
  ]
}
```

`model_id` には `id` を指定します。`language`は対象の言語で、未指定の古いモデルは日本語として扱います。モデル固有の`outputs`は、値が無い場合に省略されます。

### `GET /api/renderers`

```json
{
  "default_renderer": "utautts-world-phrase",
  "renderers": [
    {"manifest_version": 2, "kind": "synthesis-engine", "id": "utautts-world-phrase", "display_name": "UtauTTS WORLD phrase", "description": "...", "contract": "unit-renderer", "provider": "utautts-world-phrase", "provider_version": "1", "capabilities": {"frame_pitch": true}, "default_priority": 300}
  ],
  "availability": {"utautts-world-phrase": {"available": true}},
  "problems": null,
  "resamplers": null,
  "wavtools": [{"id": "builtin", "display_name": "UtauTTS built-in", "built_in": true}]
}
```

`default_renderer`はサーバー起動時の既定Rendererです。`availability`は各Rendererの実行時資源の事前検査結果（`available`と、不足時の`issues`）です。`problems`には読み込めなかったmanifestなどの診断が入ります。`resamplers`と`wavtools`にはClassic UTAUで選択できるツールが含まれ、`wavtools`には常に`builtin`が含まれます。要素が無い配列は`null`として返ります。リクエストで未知のRenderer IDを明示すると、既定Rendererへ切り替えずエラーになります。

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

このエンドポイントは日本語の読み・かなモーラ解析用です。英語・中国語の合成では、`/api/synthesize/*`の`language`／`phonemizer`を指定し、必要なら`reading`でARPAbetまたはPinyinを直接渡してください。

### `POST /api/synthesize/audio`

一つの発話を合成して`audio/wav`を返します。

```json
{
  "text": "こんにちは、今日はいい天気です。",
  "voicebank_id": "足立レイver3.5.0",
  "model_id": "frame-intonation-tcn-v12",
  "renderer": "utautts-world-phrase",
  "alias_policy": "auto",
  "intonation_strength": 1,
  "apply_pitch": true
}
```

レスポンスヘッダーの`X-UtauTTS-Reading`に使用した読み、`X-UtauTTS-Engine`にRenderer IDが入ります。

`X-UtauTTS-Diagnostic-Count`には診断の件数が入ります。診断の内容も必要な場合は`/api/synthesize/result`を使います。

#### リクエスト項目

| 項目 | 型 | 既定値 | 説明 |
|---|---|---|---|
| `text` | string | | 合成する文章。`reading` とどちらか一方が必須 |
| `reading` | string | | かな、ARPAbet、またはPinyinを直接指定 |
| `kana` | string | | `reading`と同じ入力を受け付ける互換用の別名 |
| `language` | string | 自動選択 | `ja`、`en`、`zh`。空欄なら音源・phonemizerから選び、決まらなければ日本語 |
| `phonemizer` | string | 言語から自動選択 | `ja-kana`、`en-arpasing`、`en-delta`、`en-vccv`、`en-cv`、`zh-cvvc`。言語に対応しない組み合わせはエラー |
| `voicebank_id` | string | ID順先頭 | `GET /api/voicebanks` の `id` |
| `tone` | string | `C4` | `prefix.map` 使用時の音階 |
| `color` | string | なし | `character.yaml`で定義された音源タイプ／サブバンク |
| `model_id` | string | なし | `GET /api/models` の `id` |
| `model_path` | string | なし | モデルJSONのローカルパス。指定時は`model_id`より優先し、`--model-dir`を探索せず直接読み込みます |
| `renderer` | string | 既定Renderer | `GET /api/renderers` の `id`。省略時だけ既定Rendererを使い、未知の明示IDはエラーになります |
| `resampler` | string | 自動選択 | Classic UTAUで使うresamplerの相対ID |
| `wavtool` | string | `builtin` | Classic UTAUで使うwavtoolの相対ID |
| `resampler_expressions` | object[] | なし | unit単位のresampler設定。形式は[Classic UTAU互換仕様](plugins.md#classic-utau互換仕様)を参照 |
| `alias_policy` | string | `auto` | `auto`（VC/VCV収録比から自動選択）、`cvvc-enhanced`（CVVC優先・sequential timing・VC音量35%、英語では55%）、`vcv-prefer`、`cvvc-prefer`、`cv-only` |
| `mora_duration_ms` | number | `120` | 基本モーラ長（0〜1000） |
| `pause_duration_ms` | number | `180` | 句読点の休止長（0〜3000） |
| `leading_preutterance_ms` | number | `0`（自動） | 文頭に確保する先行発声（0〜1000）。0では先頭原音の`oto.ini`から決定 |
| `release_ms` | number | `20` | 原音のリリース（余韻）長（ms）。`release_set`が真のときだけ明示値として適用 |
| `release_set` | boolean | `false` | `release_ms`を明示指定として適用する |
| `mora_durations_ms` | number[] | | モーラごとの長さ。値は0〜1000 |
| `unit_overrides` | object[] | なし | unit単位の候補・原音の明示指定（計画の上書き） |
| `intonation_strength` | number | `4` | 自動抑揚の強さ（0〜8）。v12では曲線に「強さ/4」を掛ける。詳細は[抑揚の強さ](technical-design.md#抑揚の強さ) |
| `apply_pitch` | boolean | `true` | Rendererのピッチ処理を有効にする |
| `context_duration` | boolean | `false` | 日本語のモーラ長を文脈（助詞・句末など）に応じて変える |
| `context_duration_strength` | number | `1` | 文脈連動の強度（0〜2）。0は既定1.0として扱う |
| `boundary_tone` | boolean | `true` | 日本語の文末の音調（疑問文の上昇など）を付ける |
| `boundary_tone_strength` | number | `1` | 境界音調の強度（0〜2）。0は既定1.0として扱う |
| `stretch_adapt` | boolean | `true` | 原音より大きく伸ばす日本語の長いモーラで、母音側の伸びを音源の実測に合わせて抑える |
| `stretch_adapt_strength` | number | `1` | 伸縮補正の強度（0〜2）。0は既定1.0として扱う |
| `microprosody` | boolean | `true` | 日本語のWORLD合成で、子音の前後の微細な音高変化を補う |
| `timing_warp` | boolean | `true` | 日本語・英語・中国語のWORLD合成で、学習した時間配分に合わせてモーラ内を伸縮する。包絡・開始時刻・長さ・音高は保ち、句末モーラは伸縮しない |
| `pause_context` | boolean | `true` | 句読点の種類と発話末で休止長を変える |
| `pause_context_strength` | number | `1` | 休止長補正の強度（0〜2）。0は既定1.0として扱う |
| `english_weak_form` | boolean | `true` | 英語の機能語を句中では弱形で読む。明示の読みと辞書を優先する |
| `manual_pitch` | object | なし | [手動ピッチ編集JSON](#手動ピッチ編集json) |
| `pitch_curve` | object | なし | コーパス指定の固定ピッチ曲線。指定時は自動予測の輪郭より優先 |
| `dictionary` | object[] | なし | ユーザー辞書。各項目は`surface`と`reading`を持つ |
| `diffsinger_steps` | number | `0`（既定値） | DiffSingerの拡散ステップ数。0で既定値 |
| `diffsinger_duration_mix` | number | `0`（既定値） | DiffSingerの長さ予測の混合率（0〜1）。0で既定値 |
| `diffsinger_pitch_mix` | number | `0`（既定値） | DiffSingerのピッチ予測の混合率（0〜1）。0で既定値 |
| `diffsinger_expr` | number | `0`（既定値） | DiffSingerの表現力（0〜2）。0で既定値1.0 |
| `worldline` | object | なし | WORLD providerのホスト制御（`timing_warp`／`microprosody`／`source_phone_mapping`）。空で既定 |
| `renderer_settings` | object | なし | Rendererの設定値（`/api/renderers`の`settings`）をまとめて渡すmap。未知のidもエラーにせずprovider固有値として渡します |

廃止した`speech_timing`、`word_boundary_envelope`、`prosody_experiment`は、古いクライアントのために受け取って無視します。

#### ステータスコード

- `200`: WAVバイナリ。`X-UtauTTS-Engine` / `X-UtauTTS-Reading` ヘッダー付き
- `400`: `text`／`reading`（`kana`を含む）の両方なし、範囲外のduration・`intonation_strength`、音源・モデルが未登録、指定Rendererの必要なファイル不足（`ErrUnavailable`）
- `413`: 文字数・`manual_pitch` points超過、JSON 1 MiB超過
- `422`: 合成の失敗（読み変換失敗、モデル評価失敗、未知の`alias_policy`など）

### 手動ピッチ編集JSON

`manual_pitch`（CLIでは`--manual-pitch`のファイル）は、読みの各モーラに対する音高補正をcent単位で指定します。補正は自動イントネーションに加算されます。

```json
{
  "version": 1,
  "reading": "こんにちは",
  "mode": "offset",
  "points": [
    {"position": 0, "mora": "こ", "cents": 0},
    {"position": 1, "mora": "ん", "cents": 40},
    {"position": 2, "mora": "に", "cents": 80},
    {"position": 3, "mora": "ち", "cents": 20},
    {"position": 4, "mora": "は", "cents": -30}
  ]
}
```

`position`は読みのモーラ配列に対する0始まりの位置です。`mora`も書いた場合はその位置のモーラと一致するか検証します。休止モーラは編集できず、指定していないモーラの補正は0 centです。

| `mode` | 内容 |
| --- | --- |
| `offset` | 自動イントネーションへ補正値を加算する（通常） |
| `replace` | 自動イントネーションを使わず、手動の曲線だけを使う |
| `frames` | 10 ms単位の補正値の配列`frames`を使う。GUIの拡張編集で抑揚線を描くとこの形式になる |

手動の曲線は10 ms間隔へ補間し、急な変化は安全な範囲に抑えます。

### `POST /api/synthesize/label`

`/api/synthesize/audio`と同じJSONを受け取り、合成音声に対応するHTK形式の音素ラベルを`text/plain`で返します。レスポンスには同じ`X-UtauTTS-Engine`と`X-UtauTTS-Reading`ヘッダーが付きます。

`X-UtauTTS-Diagnostic-Count`も返します。ラベルは合成計画由来の100 ns単位の時刻で、波形の強制アライメント結果ではありません。

### `POST /api/synthesize/result`

`/api/synthesize/audio`と同じJSONを受け取り、一度の合成でWAVと結果メタデータ・診断を返します。既存のWAV・LABエンドポイントの応答形式は変わりません。

```json
{
  "audio": {"format": "wav_pcm_s16le", "data": "<WAV全体のBase64>"},
  "result": {
    "reading": "ア",
    "renderer_id": "utautts-world-phrase",
    "duration_ms": 120,
    "sample_rate": 44100,
    "channels": 1,
    "unit_count": 1,
    "phoneme_labels": "0 1200000 a\n",
    "diagnostics": []
  }
}
```

`audio.data`をBase64で復号すると、WAVエンドポイントと同じバイト列になります。`result`にはPCMやサーバー上の保存先を含めません。`diagnostics`はRendererの診断と不足音素の警告の配列で、各項目は`severity`・`code`・`message`を持ちます。警告があっても合成成功なら200です。

`phoneme_labels`はLABエンドポイントと同じ計画由来のラベルです。Base64はバイナリより転送量が約3分の1増えるため、音声だけが必要ならWAVエンドポイントを使ってください。

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

- `--version`: バージョンを表示して終了する
- `--voice-dir`: ボイスバンクを格納したディレクトリ
- `--renderer`: Renderer ID。省略時は設定された優先度が最も高いものを使う
- `--renderer-dir`: Rendererプラグインの検索先。複数回指定できる
- `--model-dir`: 自己記述モデルJSONの検索先。複数回指定でき、リクエストの`model_id`で選択する
- `--host`: ローカルの待受アドレス。既定は`127.0.0.1`。`localhost`かloopback IPだけを指定できる
- `--port`: ポート。初期値は`8080`
- `--allow-voicebank-registration`: `POST /api/voicebanks` による音源パス登録を許可する（登録先は`--voice-dir`以下に制限）
- `--worldline-bridge`: worldline bridge実行ファイルを明示する
- `--openjtalk-features` / `--openjtalk-dictionary`: 自動検出を使わずhelperまたは辞書を明示する開発用オプション

### 運用制限

| オプション | 既定値 | 対象 |
| --- | --- | --- |
| `--max-json-bytes` | `1048576` | JSON本文のバイト数 |
| `--max-text-runes` | `500` | 文章・有効な読みの文字数、モーラ長・resampler編集の要素数 |
| `--max-batch-items` | `16` | 一括合成の発話数 |
| `--max-batch-wav-bytes` | `268435456` | 一括合成の非圧縮WAV合計バイト数 |
| `--max-manual-pitch-points` | `1000` | 手動ピッチのpoints数 |
| `--max-concurrent-synthesis` | `4` | 合成・解析の合計並列数 |
| `--max-concurrent-batches` | `2` | 一括合成の並列数 |
| `--queue-timeout` | `0` | 処理枠ごとの待機時間。例: `30s` |
| `--synthesis-timeout` | `0` | 解析・合成要求全体の期限。待機を含む。例: `2m` |

サイズ・件数・並列数の`0`は既定値を使い、時間の`0`は無制限です。負の値は起動時に拒否します。
