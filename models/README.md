# Prosody model license

## Bundled intonation choices

既定の抑揚モデルは`frame-intonation-v8`です。ピッチと長さの両方を予測する場合は
`prosody-multitask-v1`を選べます。学習と比較の手順は
[フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)を参照してください。

英語では`english-intonation-v1`を使います。強勢と句末の上げ下げを
英語のモーラ情報だけから予測する軽量モデルで、Open JTalkや外部データは必要ありません。
英語のカードで日本語モデルが選ばれている場合も、同じフォルダにこのモデルがあれば自動で切り替わります。

The official Japanese JSON model files currently bundled in this directory were trained
using JSUT audio. They may be used, modified, and redistributed only for
academic research, non-commercial research, and personal use. Commercial use
requires prior permission from the JSUT rights holders. They are not
distributed under the Creative Commons Attribution-ShareAlike 4.0
International license.

Each distributable model must declare its own `license` and a normalized,
package-root-relative `license_notice` path below `licenses/`. A model based on
another corpus or model may have different terms; follow the notice named by
that model instead of applying this JSUT notice automatically.

Copyright (c) 2026 yh

The official JSUT-derived models also use jsut-label-derived annotations. When
redistributing those models or adaptations for the permitted uses, preserve
the attribution, source information, and applicable dataset terms recorded in
`licenses/JSUT-DATA-AND-LABELS.txt`. The complete conditions are in
`licenses/PROSODY-MODELS.txt`.

These model terms do not grant rights to the upstream JSUT audio, BASIC5000
text, or jsut-label data, and do not replace the upstream terms.

UtauTTS source code remains covered by the repository-level MIT License. This model license does not cover bundled voicebanks or other third-party assets.

`english-intonation-v1`はUtauTTS用に作成した係数だけで構成され、
`licenses/ENGLISH-INTONATION-V1.txt`のMIT Licenseで配布します。
