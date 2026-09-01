# イントネーションとモーラ長の編集

手動ピッチ編集JSONでは読みの各モーラに対する音高補正（cent）を指定します。補正は学習イントネーションへ加算されてRendererへ渡されます。

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

画面上の編集方法は[GUIの使い方](gui.md#イントネーションとモーラ長の編集)を参照してください。JSONで指定していないモーラの補正値は0 centです。

## CLI

```powershell
go run ./cmd/utautts-cli `
  --voicebank "release/UtauTTS/voice/ボイスバンク" `
  --text "こんにちは" `
  --renderer utautts-world-phrase `
  --prosody frame-intonation-v8 `
  --prosody-pitch-only `
  --apply-pitch `
  --manual-pitch "out/manual-pitch.json" `
  --out "out/manual-pitch.wav"
```

手動カーブは10ms刻みへ補間されて急激な変化はRenderer側の安全制約で滑らかに制限されます。モーラ長は`mora_durations_ms`へ読みのモーラ順で入れます。位置も明示する場合は`mora_positions_ms`を使います。
