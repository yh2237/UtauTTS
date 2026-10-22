# UtauTTS Server

ローカルのアプリケーションからHTTPで音声合成を使うための常駐型APIサーバーです。

## サーバーの起動

```powershell
.\utautts-server.exe --voice-dir "voice" --port 8080
```

起動後、ブラウザで `http://127.0.0.1:8080/` を開くとWebコンソールUIを使えます。

> サーバーはローカル専用です。`--host` に指定できるのは `localhost` とloopback IP（`127.0.0.1`、`::1` など）だけで、既定は `127.0.0.1` です。`localhost` を指定すると、DNSを使わずに `127.0.0.1` で待ち受けます。

---

## 主なエンドポイント一覧

| メソッド | パス | 説明 |
| :--- | :--- | :--- |
| `GET` | `/api/health` | 稼働状態とバージョン |
| `GET` | `/api/voicebanks` | 認識した音源の一覧 |
| `POST` | `/api/voicebanks/reload` | 音源ディレクトリの再スキャン |
| `GET` | `/api/models` | 使える抑揚モデルの一覧 |
| `GET` | `/api/renderers` | 使えるRendererの一覧 |
| `POST` | `/api/analyze` | テキストの読みとモーラ |
| `POST` | `/api/synthesize/audio` | 1発話の合成音声（WAV、`audio/wav`） |
| `POST` | `/api/synthesize/label` | 1発話の音素境界ラベル（`.lab`、テキスト） |
| `POST` | `/api/synthesize/batch` | 複数発話の一括合成（ZIP） |
| `GET` | `/api/openapi.yaml` | OpenAPI 3.0 仕様書（YAML） |

`POST /api/analyze` が扱うのは日本語の読みとかなのモーラだけです。英語・中国語は `/api/synthesize/*` に `language` と `phonemizer` を指定し、必要なら `reading` でARPAbetかPinyinを直接渡してください。

---

## 共通の入力制限とエラー

- JSON本文は1 MiBまでで、単一のJSONオブジェクトを指定します。未知のJSONフィールドは400で拒否します。
- 1発話の `text` と有効な読み（`reading`、なければ `kana`）は、それぞれ500文字までです。
- `manual_pitch.points` は最大1000個、`mora_durations_ms` は最大500要素です。
- batchは最大16発話で、非圧縮WAVの合計は256 MiBまでです。
- 同時に実行する合成と読み解析は合わせて4件まで、batchの要求はさらに2件までです。超えた要求は空きを待ちます。待つ順番は保証しません。

これらの上限は固定です。必要なら呼出し側で要求を分けてください。上限を超えると413を返します。

API処理のエラーは `{"error":"説明","code":"分類"}` と対応するHTTPステータスで返します。未定義のパスへの404と、対応していないメソッドへの405は、HTTPルーターの応答です。

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
  "language": "ja",
  "method": "waveform-synthesis",
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
- `text` (string): 読み上げる文章（`reading` がなければ必須）。
- `reading` (string): 読み（かな、ARPAbet、Pinyin）を直接指定するときに使用。
- `language` (string): 言語（`ja`、`en`、`zh`、`es`、`pt`）。必須。
- `method` (string): 合成の方式（`waveform-synthesis` か `reference-matching`）。必須。
- `voicebank_id` (string): 音源ID。必須。
- `model_id` (string): 抑揚モデルID。
- `renderer` (string): 波形合成で使う合成エンジンID（省略時は既定のRenderer。同梱の既定は `utautts-world-phrase`）。
- `intonation_strength` (number): 抑揚の強さ（0〜8、既定値 `4.0`）。
- `mora_duration_ms` (number): 基本モーラ長（既定値 `120`）。
- `pause_duration_ms` (number): 句読点などの休止長（既定値 `180`）。
- `renderer_settings` (object): Renderer固有のオプション（例: `{"timing_warp": false}`）。

### レスポンス
- 成功すると、`200` と `Content-Type: audio/wav` でWAVを返します。
- レスポンスヘッダーには解析した読みの `X-UtauTTS-Reading` や、使ったRendererのIDの `X-UtauTTS-Engine` が付きます。

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
