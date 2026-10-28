# UtauTTS 技術設計ガイド

## 1. 合成方式の概要

UtauTTSは、UTAUボイスバンクの原音の声質を保って文章を読み上げるテキスト音声合成システムです。日本語は、`oto.ini`に基づく原音の選択・配置・時間伸縮と、機械学習モデルによる韻律（ピッチ・音量）を組み合わせる連結型の波形合成で、英語・中国語・スペイン語・ポルトガル語は参照照合で読み上げます。

---

## 2. システム全体像

GUI、CLI、HTTP Serverは、どれも共通の`synth.Service`を通して処理します。

```text
[ GUI (Qt/QML) ]   [ CLI ]   [ HTTP Server ]
          │           │             │
          └─────┬─────┴─────────────┘
                ▼
          synth.Service
          ├── Language Frontends （読み・音素・モーラ分解）
          ├── g2p （数字・通貨・外来語・混在文字の前処理）
          ├── Open JTalk          （アクセント句・単語・品詞特徴の抽出）
          ├── Voicebank Resolver  （原音候補ラティスとViterbi経路探索）
          ├── Prosody Model       （10msピッチ曲線・モーラ音量スケール予測）
          └── Plan Builder        （中間表現 Plan の構築）
                │
                ▼
          render
          └── WORLD phrase        （utautts-world-phrase）
                │
                ▼
          音声波形 (PCM) + 診断レポート (RenderReport)
```

### 処理パイプライン

1. 言語解析（Frontend）: テキストを読み・音素・モーラの列にします。日本語では、Open JTalkのライブラリ（WebAssembly版はWasmモジュール）から言語特徴量を得ます。
2. 発話設計（Prosody）: 抑揚モデル（TCN）で、10msごとのピッチの相対値（cent）と、モーラごとの音量の倍率を予測します。
3. 原音選択（Lattice & Viterbi）: ボイスバンクから原音の候補を集め、Target CostとConcatenation Costから最適な経路を決めます。同じWAVの中で続く原音には加点します。
4. 計画生成（Plan Builder）: `oto.ini`の設定値と目標の長さを突き合わせて、各原音を置く時刻を決めます。
5. 波形描画（Render）
   - `utautts-world-phrase`: 原音ごとのWORLD音響特徴量（F0・スペクトル包絡・非周期性指標）を時間軸に並べて補間し、まとめて合成し直します。

### 方式

合成の方式は波形合成と参照照合の2つで、上の処理パイプラインは波形合成のものです。

参照照合は原音選択とPlanを使いません。`refspeech`が参照TTSに文を読ませ、その音声に近いフレームを音源の索引から選んで、WORLDで合成します（[参照照合](how-utautts-speaks.md#9-参照照合)）。WORLDのライブラリは波形合成と同じものを使います。

WORLDの解析の設定は方式ごとに違います。

| 項目 | 波形合成（`utautts-world-phrase`） | 参照照合 |
| :--- | :--- | :--- |
| フレーム周期 | 10ms | 5ms |
| F0 | 音源の`.frq`。なければHarvest | Harvest |
| D4Cのしきい値 | 0 | 0.85 |
| スペクトル包絡 | そのまま | 60次に符号化 |

---

## 3. 日本語音声における補正パイプライン

原音の声質（スペクトル包絡）は変えずに、自然な発話にするための補正を段階ごとに掛けます。

| 段階 | 補正内容 | 処理モジュール | 設定項目 |
| :--- | :--- | :--- | :--- |
| 原音選択 | 収録のない「ヴ」「デュ」などを同じ行の音で代替 | `voicebank` | なし |
| 発話設計 | 文末の音調（疑問文の語尾上げなど）を付加 | `tts/japanese_boundary.go` | `boundary_tone` |
| 発話設計 | 文脈（助詞・句末）に応じたモーラ長の補正 | `tts/japanese_duration.go` | `context_duration` |
| 発話計画 | 句読点の種類に応じた休止長の調整 | `plan/pause_context.go` | `pause_context` |
| 描画（本体） | 母音の伸ばしすぎの抑制（長音の音質を保つ） | `render/base/timing.go` | `stretch_adapt` |
| 描画（本体） | 無声破裂音の直後の小さなピッチの上昇（Micro-prosody） | `render/world/microprosody.go` | `microprosody` |
| 描画（WORLD）| 自然音声モデルに基づくモーラ内の音素長の調整 | `speechtiming`, `worldrender` | `timing_warp` |

---

## 4. 中間表現「Plan」の役割

原音の選択と波形の描画を切り離すため、`internal/plan`の`Plan`構造体を中間表現にします。主な内容は次のとおりです。
- モーラの位置、音素の文脈、使うWAV、`oto.ini`の適用値（offset, consonant, cutoff, preutterance, overlap）
- 目標のタイミング（開始時刻・モーラ長）
- ピッチ係数、音量係数、経路探索のスコア

Planは読み取り専用として扱い、描画のときに補正した結果は`RenderReport`に分けて記録します。

---

## 5. キャッシュとリソース管理

応答を速くするため、各層でキャッシュを使います。
- ボイスバンクの定義: メモリに保持します。
- Open JTalkの辞書: ライブラリを同じプロセスに読み込み、辞書（約100MB）を読み込んだまま保ちます。
- WAVと音響特徴量: WAV（`internal/sourceaudio`）とWORLDの音響特徴量（`UtauWLD/worldrender`）を、それぞれ最大256MiBのLRUキャッシュに保持し、解析し直す手間を減らします。
- 参照の索引: 音源ごとの索引を利用者のキャッシュ（`UtauTTS/reference-index`）に保存し、読み込んだ索引はメモリに保持します。合成のたびに録音の一覧・大きさ・更新時刻を確かめ、変わっていれば作り直します。版・特徴量モデルが合わない索引と、長く使っていない録音ごとの解析結果は、索引を作る前に削除します。
