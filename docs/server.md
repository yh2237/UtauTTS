# UtauTTS Server

ローカルのアプリケーションからHTTPで音声合成を使うための常駐型APIサーバーです。

## サーバーの起動

```powershell
.\utautts-server.exe --voice-dir "voice" --port 8080
```

起動後、ブラウザで `http://127.0.0.1:8080/` を開くとWebコンソールUIを使えます。

> サーバーはローカル専用です。`--host` に指定できるのは `localhost` とloopback IP（`127.0.0.1`、`::1` など）だけで、既定は `127.0.0.1` です。`localhost` を指定すると、DNSを使わずに `127.0.0.1` で待ち受けます。

| オプション | 既定値 | 内容 |
| :--- | :--- | :--- |
| `--port` | `8080` | 待ち受けポート |
| `--host` | `127.0.0.1` | 待ち受けホスト。loopbackのみ |
| `--voice-dir` | `voice` | 音源のディレクトリ |
| `--openjtalk-engine` / `--openjtalk-dictionary` | 空 | Open JTalkのライブラリと辞書 |
| `--model-dir` | 空 | 抑揚モデルのディレクトリ。繰り返し指定できる |
| `--max-concurrent-synthesis` | `4` | 同時に実行する合成と読み解析の件数 |
| `--max-concurrent-batches` | `2` | 同時に実行する一括合成の件数 |
| `--queue-timeout` | `30s` | 実行枠が空くのを待つ上限。負なら要求の切断まで待つ |
| `--write-timeout` | `10m` | 応答の書き込み期限 |

---

## 主なエンドポイント一覧

| メソッド | パス | 説明 |
| :--- | :--- | :--- |
| `GET` | `/api/health` | 稼働状態、バージョン、実行枠の使用状況 |
| `GET` | `/api/voicebanks` | 認識した音源の一覧 |
| `POST` | `/api/voicebanks/reload` | 音源ディレクトリの再スキャン |
| `GET` | `/api/models` | 使える抑揚モデルの一覧 |
| `GET` | `/api/settings` | 合成設定の一覧（既定値と範囲） |
| `POST` | `/api/analyze` | テキストの読みとモーラ |
| `POST` | `/api/synthesize/audio` | 1発話の合成音声（WAV、`audio/wav`） |
| `POST` | `/api/synthesize/label` | 1発話の音素境界ラベル（`.lab`、テキスト） |
| `POST` | `/api/synthesize/report` | 1発話の合成結果と診断の詳細（JSON） |
| `POST` | `/api/synthesize/jobs` | 1発話の合成を非同期で開始（ジョブID） |
| `GET` | `/api/synthesize/jobs/{job_id}` | ジョブの状態と進捗 |
| `GET` | `/api/synthesize/jobs/{job_id}/audio` | 完了したジョブの合成音声（WAV） |
| `POST` | `/api/synthesize/batch` | 複数発話の一括合成（ZIP） |
| `GET` | `/api/openapi.yaml` | OpenAPI 3.0 仕様書（YAML） |

`POST /api/analyze` には `language` を指定します。

---

## 共通の入力制限とエラー

- JSON本文は1 MiBまでで、単一のJSONオブジェクトを指定します。未知のJSONフィールドは400で拒否します。
- 1発話の `text` と有効な読み（`reading`、なければ `kana`）は、それぞれ500文字までです。
- `manual_pitch.points` は最大1000個、`mora_durations_ms` は最大500要素です。
- `mora_durations_ms` は休止を含む全モーラ数と同じ要素数を指定します。各値は0〜1000のミリ秒です。参照照合では、休止を飛ばすと値が別のモーラへずれるため、要素数が合わない要求はエラーになります。波形合成では部分的な配列も使えます。
- `manual_pitch.points[].position` は休止を含む全モーラ番号で、休止の位置は指定できません。
- `phone_weights` は休止を含む全モーラ番号の配列です。各要素はそのモーラの音素数と同じ長さの非負の比で、参照照合ではモーラ内の音素の長さの配分に使います。休止は空配列にします。
- `consonant_ms`と`vowel_ms`は休止を含む全モーラ数と同じ長さの配列です。子音だけまたは母音だけのモーラでは、片方だけを指定できます。休止は0にします。
- batchは最大16発話で、非圧縮WAVの合計は256 MiBまでです。
- 同時に実行する合成と読み解析は既定で4件まで、batchの要求はさらに2件までです。件数は起動オプションで変えられます。空きを待つ順番は保証しません。

文字数・要素数・発話数の上限は固定です。必要なら呼出し側で要求を分けてください。上限を超えると413を返します。

実行枠が空かないまま `--queue-timeout` を過ぎた要求は429を返します。`Retry-After` の秒数だけ待って再試行してください。現在の使用状況は `/api/health` の `synthesis` と `batch`（`limit`・`active`・`queued`）で分かります。

### エラーの形式

API処理のエラーは `{"error":"説明","code":"分類"}` と対応するHTTPステータスで返します。入力不正のときは、どの項目が悪いかを `field`、期待する形式を `expected` で添えます。

```json
{"error":"mora_duration_ms must be between 0 and 1000","code":"invalid_request","field":"mora_duration_ms","expected":"0..1000"}
```

未定義のパスへの404と、対応していないメソッドへの405も同じ形式です。405は `Allow` ヘッダーに使えるメソッドを返します。

| `code` | 内容・HTTPステータス |
| :--- | :--- |
| `invalid_request` | JSON・必須項目・範囲・ファイル名などの入力不正（400） |
| `limit_exceeded` | 本文・文字数・編集要素・一括合成の上限超過（413） |
| `unavailable` | 音源・モデル等の不足（400）、終了済みRuntime（503） |
| `synthesis_error` | 読み解析・合成の失敗（422） |
| `canceled` / `deadline_exceeded` | 要求のキャンセル・期限切れ（408） |
| `forbidden` | 非ローカルのHost、別サイトからのブラウザ操作（403） |
| `not_found` | 未定義のパス、存在しないジョブ（404） |
| `method_not_allowed` | パスはあるがメソッドが違う（405） |
| `too_many_requests` | 実行枠が空かないまま待ち時間の上限を過ぎた（429） |
| `not_ready` | ジョブがまだ完了していない（409） |
| `internal_error` | 再読込・一時ファイル・ZIP出力などの内部エラー（500） |

CLIの `--json` が返す `error.code` は語彙が違います。同じ失敗は次のように対応します。

| CLIの `code` | HTTPの `code` |
| :--- | :--- |
| `invalid_arguments` / `flag_parse_error` / `input_error` | `invalid_request` |
| `initialization_error` | `unavailable` |
| `shutdown_error` / `output_error` | `internal_error` |
| `synthesis_error` / `unavailable` / `canceled` / `deadline_exceeded` | 同名 |

---

## 合成APIの仕様（`POST /api/synthesize/audio`）

### リクエストJSONの例
```json
{
  "text": "こんにちは、今日はいい天気です。",
  "language": "ja",
  "method": "waveform-synthesis",
  "voicebank_id": "足立レイver3.5.0",
  "model_id": "frame-intonation-tcn-v12",
  "intonation_strength": 4.0,
  "mora_duration_ms": 120,
  "pause_duration_ms": 180,
  "apply_pitch": true
}
```

### 主なパラメータ
- `text` (string): 読み上げる文章（`reading` がなければ必須）。
- `reading` (string): 読み（かな、ARPAbet、Pinyin）を直接指定するときに使用。
- `language` (string): 言語（`ja`、`en`、`zh`、`es`、`pt`）。必須。
- `method` (string): 合成の方式（`waveform-synthesis` か `reference-matching`）。必須。
- `voicebank_id` (string): 音源ID。必須。
- `model_id` (string): 抑揚モデルID。
- `intonation_strength` (number): 抑揚の強さ（0〜8、既定値 `4.0`）。
- `mora_duration_ms` (number): 基本モーラ長（既定値 `120`）。
- `pause_duration_ms` (number): 句読点などの休止長（既定値 `180`）。
- `settings` (object): `/api/settings` の設定をIDで指定する（例: `{"timing_warp": false}`）。使えない値は結果の`diagnostics`で返す。

### レスポンス
- 成功すると、`200` と `Content-Type: audio/wav` でWAVを返します。
- レスポンスヘッダーには解析した読みの `X-UtauTTS-Reading`、合成の方式の `X-UtauTTS-Method`、使えなかった設定などの診断の数の `X-UtauTTS-Diagnostic-Count` が付きます。

---

## 診断の詳細（`POST /api/synthesize/report`）

`X-UtauTTS-Diagnostic-Count` は件数だけなので、内容はこのエンドポイントで取得します。要求の形式は `/api/synthesize/audio` と同じで、WAVの代わりに合成結果の情報をJSONで返します。

```json
{
  "reading": "コンニチワ",
  "method": "waveform-synthesis",
  "duration_ms": 1200,
  "sample_rate": 44100,
  "channels": 1,
  "unit_count": 5,
  "phoneme_labels": "...",
  "diagnostics": [
    {"severity": "warning", "code": "unknown_setting", "message": "..."}
  ],
  "logs": ["..."]
}
```

`diagnostics` は使えなかった設定と描画時の警告です。`logs` は索引の作成など時間のかかる処理の途中経過です。

---

## 非同期合成（`POST /api/synthesize/jobs`）

長い合成の進捗を知りたいときは、ジョブを作って状態を繰り返し問い合わせます。要求の形式は `/api/synthesize/audio` と同じです。入力不正は受け付け時に400・413で返るので、202が返れば要求は妥当です。

```json
{"job_id": "8f14e45fceea167a", "status": "queued", "status_url": "/api/synthesize/jobs/8f14e45fceea167a"}
```

`GET /api/synthesize/jobs/{job_id}` を完了するまで呼びます。

```json
{
  "job_id": "8f14e45fceea167a",
  "status": "queued",
  "queue_position": 1,
  "created_at": "2026-11-08T08:00:00.000000Z",
  "elapsed_ms": 1200
}
```

- `status` は `queued`（実行枠待ち）、`running`（合成中）、`succeeded`、`failed` のいずれかです。
- `queue_position` は先に並んでいるジョブの数です。`queued` 以外では0です。
- `succeeded` になると `result` に `/api/synthesize/report` と同じ内容が入ります。
- `failed` になると `error`・`code`・`field`・`expected` に失敗の内容が入ります。`code` は上の表と同じ語彙です。
- 合成の途中経過は段階（`queued` / `running`）までで、進捗の割合は返しません。

完了したジョブの音声は `GET /api/synthesize/jobs/{job_id}/audio` で取得します。まだ完了していなければ409です。ジョブは完了から10分で消え、同時に保持するのは32件までです。あふれた要求は429を返します。

---

## 手動ピッチ補正（`manual_pitch`）

GUIで調声したピッチ曲線を反映するときは、`manual_pitch` にcent単位の補正値を指定します。

```json
{
  "version": 1,
  "reading": "コンニチワ",
  "mode": "offset",
  "points": [
    {"position": 0, "mora": "こ", "cents": 0},
    {"position": 1, "mora": "ん", "cents": 40},
    {"position": 2, "mora": "に", "cents": 80},
    {"position": 3, "mora": "ち", "cents": 20},
    {"position": 4, "mora": "わ", "cents": -30}
  ]
}
```
- `mode`: `"offset"`（自動予測値に加算）、`"replace"`（手動値だけを使う）、`"frames"`（`frames` に並べた10msごとの補正値を自動予測値に加算。発話の長さに合わせて伸縮）。

`position` は、読みのモーラ配列での0始まりの位置です。`mora` も指定すると、その位置のモーラと一致するかを検証します。休止のモーラは編集できず、指定していないモーラの補正は0centです。手動の曲線は10ms間隔に補間し、急な変化は抑えます。

`points` と `frames` の各補正値は±4800cent以内です。`frames` モードでは `points` を併用できず、`frames` は最大20000個です。補正値は自動予測値との差なので、GUIのピッチ欄が扱う±800centより大きくなることがあります。
