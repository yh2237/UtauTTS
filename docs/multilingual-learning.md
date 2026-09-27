# 英語・中国語の発話モデルの学習

音素ごとの時間長・相対ピッチ・相対音量を学習し、UTAU原音を使う合成へ適用します。疎な言語特徴とridge回帰を使う実験用モデルで、同梱モデルはありません。英語の既定抑揚モデルとは別の学習経路です。

英語TCNの学習は[英語フレーム抑揚モデルの学習](english-frame-intonation-training.md)、原音区間の解析は[原音区間ライブラリ](source-understanding.md)を参照してください。

## 学習データ

自然音声のmono・16bit PCM WAVと、各音素の開始・終了時刻が必要です。強制整列を使う場合は、録音と音素列・境界が対応しているか確認してください。音節時刻の均等分割は行いません。

話者単位で学習・検証・試験を分けます。検証データは必須です。話者・同一文章・同一音声が分割をまたぐデータは使用できません。コーパスの出典とライセンスも入力データへ記録します。

## 言語特徴を出力する

自然音声と同じ文章をUtauTTSで解析・合成し、`--plan-out`で合成計画を保存します。phonemizer・辞書・弱形設定を学習と推論で揃えてください。

```powershell
go run ./cmd/tools/speech-score --plan out/example.plan.json --id utterance-001 --out out/utterance-001.template.json
```

`phones`の`baseline_ms`は規則による基準長です。実測値は次の観測データへ記録します。基準音素長は既定で120msで、変更する場合は`--base-ms`を指定します。

## 音声と音素時刻を結び付ける

観測データの`phones`は、テンプレートと同じ順序・位置・音素番号・記号にします。`audio_path`は観測JSONのディレクトリから解決します。`alignment`には`manual`または`forced`を指定します。

```json
{
  "utterances": [{
    "id": "utterance-001",
    "audio_path": "wavs/utterance-001.wav",
    "speaker": "speaker-001",
    "split": "train",
    "kind": "natural",
    "corpus": "corpus name and version",
    "license": "source license",
    "alignment": "manual",
    "phones": [{
      "position": 0, "phone_index": 0, "symbol": "p",
      "start_ms": 100, "end_ms": 170
    }]
  }]
}
```

```powershell
python tools/prepare-multilingual-speech.py out/utterance-001.template.json out/utterance-002.template.json --observations data/observations.json --out out/speech-corpus.jsonl
```

時間長は実測境界から、相対音量は音素区間のRMSから取得します。相対ピッチは発話内の有声F0中央値に対するセント値です。有声フレームが足りない区間のピッチは欠測として扱います。

## 学習する

PythonとNumPyを使用します。言語は`en`または`zh`を指定し、言語ごとにモデルを作成します。

```powershell
python tools/train-multilingual-speech.py out/speech-corpus.jsonl --language en --id my-english-speech-v1 --out out/my-english-speech-v1.json
```

モデルにはコーパス・ライセンス・入力manifestのSHA-256・学習時の音素数・評価結果を記録します。ピッチ・音量の教師が足りない場合、その出力項目は作成しません。検証値は自然音声の測定値との比較であり、合成後の音声は別途試聴してください。

## 合成に適用する

通常のCLI指定に次を追加します。

```powershell
--speech-model out/my-english-speech-v1.json
```

モデルと言語が一致しない場合はエラーになります。学習側で5例未満の音素は規則へ戻ります。ピッチは有声教師の被覆も確認し、不足する音素には規則を使います。

| 項目 | 適用範囲 |
| --- | --- |
| 時間長 | 規則値の0.5〜2倍、最終値は8〜500ms |
| 相対ピッチ | ±300セント |
| 相対音量 | 0.7〜1.3倍 |

手動の単位長とピッチ曲線を優先します。`--prosody-pitch-only`ではモデルの時間長・音量を適用しません。中国語のピッチ補正は既存の声調曲線を土台にします。

適用モデルは合成計画の`speech_model_id`に記録します。同じ文章・音源・Renderer・設定で規則のみの場合と比較してください。
