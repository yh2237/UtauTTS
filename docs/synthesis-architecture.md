# UTAU音源の合成処理

UTAU音源を使う合成経路の、開発者向けの構成です。DiffSingerなどのニューラル合成は別の経路を使います。

## 処理の段階

| 段階 | 担当 | 出力 |
| --- | --- | --- |
| 発音解析 | `frontend`、`tts`の言語profile | 読み、音素、強勢・声調など |
| 発話設計 | `prosody`、言語profile | 長さ・ピッチ・音量の予測 |
| 原音選択 | `voicebank`、`connection` | aliasと原音の選択 |
| 発話計画 | `tts/stage_plan.go`、`plan` | unitと音素の出力時刻 |
| ピッチ確定 | `tts/stage_pitch.go` | 描画用のピッチ曲線 |
| 描画 | `render`、選択したprovider | 音声と描画結果の診断 |

`tts.SynthesizeWithOptions`がこれらを順に実行します。言語固有の読み、韻律、音素の長さは`languageProfile`にまとめ、WORLD固有の伸縮や接続は`render/worldline`で扱います。

共通の`Mora`型は日本語のモーラ以外にも使われます。英語・中国語を扱う場合は、名前だけで判断せず、`Language`と`Phones`を参照してください。

## 補正と上書き

発話計画では、言語profileの音素長と予測を`plan.Build`へ渡し、構築後にユーザーのunit指定を適用します。ピッチは、指定済みの曲線を優先し、未指定時に言語規則またはフレーム抑揚モデルから生成します。その後、言語別の境界音調、手動ピッチの順に処理します。手動ピッチの適用後は曲線の変化を制限します。

WORLDの英語・中国語では、まずotoと音響的な推定から原音の時間写像を作ります。試聴用の手動区間があればそれを優先し、なければ原音区間ライブラリを照合します。手動区間の不整合はエラーとし、ライブラリの未登録・不整合はotoの写像へ戻します。

通常ライブラリと手動区間は、共通の`SourceSpan`型と`placeSourceSpan`で検証・描画します。音素時刻と原音区間を対応付けた後、対象の破裂音に対して過渡区間を保護します。適用した写像は描画結果の`speech_mapping`と原音・出力のanchorに記録されます。

既存ツールとの互換性のため、`ExperimentalSourceSpan`などの旧型名は別名として残しています。WORLD設定の`E2A`・`E2B`も維持し、内部では閉鎖・解放分離と日本語の破裂音保護を表すメソッド名で参照します。

## 日本語の補正の流れ

日本語の補正は段階ごとに分かれています。原音の声質（包絡）を変える補正はありません。

| 段階 | 補正 | 場所 | 設定 |
| --- | --- | --- | --- |
| 原音選択 | 録音の無いヴ行・デュ・テュを同音の行で代替 | `voicebank/resolver.go`（`equivalentKanaForms`） | なし |
| 発話設計 | フレーム抑揚モデル、強さの拡大（2を超えると大きい動きほど広げる） | `prosody`、`tts`（`scaleAutomaticPitchCurve`） | `intonation_strength` |
| 発話設計 | 文末の音調 | `tts/japanese_profile.go` | `boundary_tone` |
| 発話設計 | 文脈に応じたモーラ長（既定は無効） | `tts/japanese_duration.go` | `context_duration` |
| 発話計画 | 句読点の休止長 | `plan/pause_context.go` | `pause_context` |
| 発話計画 | CVVCで続く子音が長いときにVCを延ばす | `plan/plan.go`（`cvvcTransitionDuration`） | なし |
| 発話計画 | 原音の校正（固定部・有声開始・破裂の過渡） | `voicebank/speech_profile.go` | なし |
| 描画（本体） | 伸縮の有界化（原音より大きく伸ばすとき） | `render/base/timing.go`（`AdaptStretchTiming`） | `stretch_adapt` |
| 描画（本体） | 単独音の母音だけのモーラを前の母音から滑らかにつなぐ | `render/worldline/singlecv_legato.go` | なし |
| 描画（本体） | 子音の前後の小さな音高の動き | `render/worldline/microprosody.go` | `microprosody` |
| 描画（本体） | 時間伸縮の入力（モーラ、CVVCではVCを含む子音の長さ） | `render/worldline/worldline.go`（`timingWarpJob`） | `timing_warp` |
| 描画（bridge） | 母音接続・破裂音の保護・同じ母音の隙間の補修 | `worldrender/world_speech.go`、`world_gap.go`、`stop_burst.go` | なし |
| 描画（bridge） | 学習した読み上げの動きに合わせてモーラの中だけ時間伸縮 | `speechtiming`、`worldrender/timing_warp.go` | `timing_warp` |

描画（本体）の処理は`render/worldline/worldline.go`の`renderWorldlineEngine`が、タイミング（`prepareWorldlineTiming`）、音高（`prepareWorldlinePitch`）、原音の準備、素片（`worldlineUnitBuilder`）、bridgeの実行（`runWorldlineBridge`）の順に行います。bridgeへ渡す内容は`provider.UnitRendererJob`に明示し、bridgeは合成計画を読み直しません。

## 原音データの作成

原音の観測、MFAによる整列、候補探索、区間選択、ライブラリ作成は事前処理です。通常の合成中にMFAは起動しません。

`tools/source_phone_common.py`はJSON入出力、PCM識別、音素列照合、ライブラリ区間検証、観測のコピーとパス解決を共有します。英語・中国語の探索条件と音響的な採否判定は、それぞれの探索ツールに保持します。CLIのファイル名と引数は共通化の前後で同じです。

データの導入と作成手順は[原音区間ライブラリ](source-understanding.md)、利用者向けの設定は[日本語・英語・中国語の読み上げ](multilingual.md)を参照してください。
