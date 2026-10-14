# UtauTTS Server

ローカルアプリケーションからHTTP経由で音声合成機能を利用できる常駐型APIサーバーです。Windows、Linux、macOSに対応しています。

## サーバーの起動

```powershell
.\utautts-server.exe --voice-dir "voice" --port 8080
```

起動後、ブラウザで `http://127.0.0.1:8080/` にアクセスすると、WebコンソールUIからパラメータの確認や試聴、バッチダウンロードを行えます。

> **セキュリティについて**: サーバーはローカル専用です。`--host` は `localhost` またはloopback IP（`127.0.0.1`、`::1` など）だけを受け付け、既定は `127.0.0.1` です。`localhost` はDNSを使わず `127.0.0.1` で待ち受けます。

---

## 主なエンドポイント一覧

| メソッド | パス | 説明 |
| :--- | :--- | :--- |
| `GET` | `/api/health` | サーバー稼働状態とバージョンの確認 |
| `GET` | `/api/voicebanks` | 認識されているボイスバンク一覧の取得 |
| `POST` | `/api/voicebanks/reload` | 音源ディレクトリの再スキャン |
| `GET` | `/api/models` | 利用可能な抑揚モデル一覧の取得 |
| `GET` | `/api/renderers` | 利用可能なRenderer一覧の取得 |
| `POST` | `/api/analyze` | テキストの読み・モーラ分解結果の取得 |
| `POST` | `/api/synthesize/audio` | 1発話の合成音声をWAVバイナリ（`audio/wav`）で取得 |
| `POST` | `/api/synthesize/label` | 1発話の音素境界ラベル（`.lab`）をテキスト形式で取得 |
| `POST` | `/api/synthesize/batch` | 複数発話の一括合成（ZIP形式でダウンロード） |
| `GET` | `/api/openapi.yaml` | OpenAPI 3.0 仕様書（YAML）の取得 |

`POST /api/analyze` は日本語の読み・かなモーラ解析用です。英語・中国語は `/api/synthesize/*` の `language` / `phonemizer` を指定し、必要なら `reading` でARPAbetまたはPinyinを直接渡してください。

---

## 共通の入力制限とエラー

- JSON本文は1 MiBまでで、単一のJSONオブジェクトを指定します。未知のJSONフィールドは400で拒否します。
- 1発話の `text` と有効な読み（`reading`、なければ `kana`）は、それぞれ500文字までです。
- `manual_pitch.points` は最大1000個、`mora_durations_ms` と `resampler_expressions` は最大500要素です。
- batchは最大16発話、非圧縮WAVの合計256 MiBまでです。
- 合成と読み解析は合計4並行、batchはさらに最大2並行です。超過分は空きを待ち、待機順の保証はありません。

これらの上限は固定です。必要なら呼出し側で要求を分けてください。上限超過は413を返します。

各API処理のエラーは `{"error":"説明","code":"分類"}` と対応するHTTPステータスで返します。未定義パス・未対応メソッドの404・405はHTTPルーターの応答です。

| `code` | 内容・HTTPステータス |
| :--- | :--- |
| `invalid_request` | JSON・必須項目・範囲・ファイル名などの入力不正（400） |
| `limit_exceeded` | 本文・文字数・編集要素・一括合成の上限超過（413） |
| `unavailable` | 音源・モデル・Renderer等の不足（400）、終了済みRuntime（503） |
| `synthesis_error` | 読み解析・合成の失敗（422） |
| `canceled` / `deadline_exceeded` | 要求のキャンセル・期限切れ（408） |
| `forbidden` | 非ローカルのHost、別サイトからのブラウザ操作（403） |
| `internal_error` | 再読込・一時ファイル・ZIP出力などの内部エラー（500） |
---

## 合成APIの仕様（`POST /api/synthesize/audio`）

### リクエストJSONの例
```json
{
  "text": "こんにちは、今日はいい天気です。",
  "voicebank_id": "足立レイver3.5.0",
  "model_id": "frame-intonation-tcn-v12",
  "renderer": "utautts-world-phrase",
  "intonation_strength": 4.0,
  "mora_duration_ms": 120,
  "pause_duration_ms": 180,
  "apply_pitch": true
}
```

### 主なパラメータ
- `text` (string): 読み上げる文章（`reading` 未指定時は必須）。
- `reading` (string): 読み（かな、ARPAbet、Pinyin）を直接指定する場合に使用。
- `voicebank_id` (string): 使用する音源ID（省略時は一覧先頭の音源）。
- `model_id` (string): 使用する抑揚モデルID。
- `renderer` (string): 合成エンジンID（既定値: `utautts-world-phrase`）。
- `intonation_strength` (number): 抑揚の強さ（0〜8、既定値 `4.0`）。
- `mora_duration_ms` (number): 基本モーラ長（既定値 `120`）。
- `pause_duration_ms` (number): 句読点等の休止長（既定値 `180`）。
- `renderer_settings` (object): Renderer固有のオプション（例: `{"timing_warp": false}`）。

### レスポンス
- 成功時はステータスコード `200` とともに `Content-Type: audio/wav` でWAVバイナリを返却します。
- レスポンスヘッダーには `X-UtauTTS-Reading`（解析された読み文字列）や `X-UtauTTS-Engine`（使用エンジンID）が付与されます。

---

## 手動ピッチ補正（`manual_pitch`）

GUIで調声したピッチ曲線を反映させたい場合、リクエスト内の `manual_pitch` フィールドでcent単位の補正値を指定できます。

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
- `mode`: `"offset"`（自動予測値への加算）、`"replace"`（手動値のみ使用）、`"frames"`（10msごとの直接指定）。

`position` は読みのモーラ配列に対する0始まりの位置です。`mora` も指定した場合は、その位置のモーラと一致するか検証します。休止モーラは編集できず、指定していないモーラの補正は0centです。手動の曲線は10ms間隔へ補間し、急な変化は安全な範囲に抑えます。

`points` と `frames` の各補正値は±1200cent以内です。`frames` モードでは `points` を併用できず、`frames` は最大20000個です。
