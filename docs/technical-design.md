# UtauTTS 技術設計ガイド

## 1. 合成方式の概要

UtauTTSは、UTAUボイスバンクの原音の声質を保持しつつ、`oto.ini` に基づく原音選択・タイミング配置・時間伸縮と、機械学習モデルによる自然な韻律（ピッチ・音量）を組み合わせて文章を読み上げる連結型テキスト音声合成システムです。

---

## 2. システム全体像

GUI、CLI、HTTP Serverのいずれのインターフェースも、共通の `synth.Service` コア層を介して処理を実行します。

```text
[ GUI (Qt/QML) ]   [ CLI ]   [ HTTP Server ]
          │           │             │
          └─────┬─────┴─────────────┘
                ▼
          synth.Service
          ├── Language Frontends （読み・音素・モーラ分解）
          ├── Open JTalk Helper   （アクセント句・単語・品詞特徴の抽出）
          ├── Voicebank Resolver  （原音候補ラティスとViterbi経路探索）
          ├── Prosody Model       （10msピッチ曲線・モーラ音量スケール予測）
          └── Plan Builder        （中間表現 Plan の構築）
                │
                ▼
          render.Config
          ├── UnitRenderer        （WORLD phrase / Classic UTAU）
          └── NeuralSynthesizer   （DiffSinger）
                │
                ▼
          音声波形 (PCM) + 診断レポート (RenderReport)
```

### 処理パイプライン

1. **言語解析（Frontend）**: テキストを読み・音素・モーラ列に変換します。日本語では常駐型Open JTalkヘルパープロセスから言語特徴量を取得します。
2. **発話設計（Prosody）**: 抑揚モデル（TCN）により、10ms単位のピッチ相対値（cent）およびモーラごとの音量倍率を予測します。
3. **原音選択（Lattice & Viterbi）**: ボイスバンクから原音候補を抽出し、Target CostとConcatenation Costに基づき最適経路を決定します。同一WAV内の連続原音にはボーナススコアを付与します。
4. **計画生成（Plan Builder）**: `oto.ini` 設定値と目標長を突き合わせ、各原音の正確な配置タイミングを決定します。
5. **波形描画（Render）**: 
   - `utautts-world-phrase`: 原音ごとのWORLD音響特徴量（F0・スペクトル包絡・非周期性指標）を時間軸上に配置・補間し、一括再合成します。
   - `classic-utau`: 外部resamplerとwavtoolを呼び出して個別波形を加工・接続します。

---

## 3. 日本語音声における補正パイプライン

原音の声質（スペクトル包絡）そのものは変更せず、自然な発話のための補正を段階的に適用します。

| 段階 | 補正内容 | 処理モジュール | 設定項目 |
| :--- | :--- | :--- | :--- |
| **原音選択** | 収録のない「ヴ」「デュ」等を同音行で自動代替 | `voicebank` | なし |
| **発話設計** | 文末音調（疑問文の語尾上昇等）の付加 | `tts/japanese_boundary.go` | `boundary_tone` |
| **発話設計** | 文脈（助詞・句末）に応じたモーラ長補正 | `tts/japanese_duration.go` | `context_duration` |
| **発話計画** | 句読点の種類に応じた休止長調整 | `plan/pause_context.go` | `pause_context` |
| **描画（本体）** | 母音過度伸長の抑制（長音時の音質維持） | `render/base/timing.go` | `stretch_adapt` |
| **描画（本体）** | 無声破裂音直後の微小ピッチ上昇（Micro-prosody） | `render/worldline/microprosody.go` | `microprosody` |
| **描画（bridge）**| 自然音声モデルに基づくモーラ内音素長最適化 | `speechtiming`, `worldrender` | `timing_warp` |

---

## 4. 中間表現「Plan」の役割

原音選択とRendererを疎結合にするため、`internal/plan` の `Plan` 構造体を中間表現として使用します。
- モーラ位置、音素コンテキスト、使用WAV、`oto.ini` 適用値（offset, consonant, cutoff, preutterance, overlap）
- 目標タイミング（開始時刻・モーラ長）
- ピッチ係数、音量係数、経路探索スコア

Planはイミュータブル（読み取り専用）として扱われ、Renderer実行時の動的補正結果は `RenderReport` に分離して記録されます。これにより、同一のPlanを異なるRendererに渡して品質比較することが可能です。

---

## 5. キャッシュ戦略とリソース管理

高速なレスポンスを実現するため、各層でキャッシュを運用しています。
- **ボイスバンク定義**: メモリ上に常駐保持。
- **Open JTalk辞書**: 別プロセスのヘルパーを起動して辞書（約100MB）を常駐化。
- **WAV & 音響特徴量**: プロセス内LRUキャッシュ（最大256MB）により、再解析コストを削減。
