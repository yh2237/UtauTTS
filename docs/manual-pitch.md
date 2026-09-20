# イントネーションとモーラ長の編集

手動ピッチ編集JSONでは、読みの各モーラに対する音高補正をcent単位で指定します。この補正は自動イントネーションに加算され、Rendererへ渡されます。

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

`position`は読みのモーラ配列に対する0始まりの位置です。`mora`も書いた場合はその位置のモーラと一致するか検証されます。休止モーラは編集対象になりません。

`mode`は次の2種類です。

- `offset`: 学習イントネーションへ補正値を加算します。通常はこちらを使います。
- `replace`: 学習イントネーションを使わず、手動カーブだけを使います。

## GUI

画面上の操作は[基本編集](gui.md#基本編集)と[拡張編集](gui.md#拡張編集)を参照してください。JSONで指定していないモーラの補正値は0 centです。

## CLI

```powershell
go run ./cmd/utautts-cli `
  --voicebank "voice/ボイスバンク" `
  --text "こんにちは" `
  --renderer utautts-world-phrase `
  --prosody frame-intonation-v8 `
  --prosody-pitch-only `
  --apply-pitch `
  --manual-pitch "out/manual-pitch.json" `
  --out "out/manual-pitch.wav"
```

手動ピッチを波形へ反映するには`--apply-pitch`が必要です。手動カーブは10 ms間隔へ補間され、急激な変化は安全な範囲に抑えられます。

モーラごとの長さは、読みの順に並べた配列、または`{"mora_durations_ms": [...]}`形式のJSONを`--mora-durations`へ指定します。`mora_positions_ms`はプレビューや合成結果に含まれる出力値です。
