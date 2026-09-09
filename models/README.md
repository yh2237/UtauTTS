# Prosody model license

## Bundled intonation choices

`frame-intonation-v8` remains the default. `frame-intonation-v9` is an alternative
trained on all 5,000 BASIC5000 utterances with JSUT accent annotations. It is
not designated as a quality replacement for v8. Its recorded evaluation uses
training utterances, not an independent test set. Both use the version 8
frame-model JSON schema; the model ID and schema version serve different purposes.

The official JSON model files currently bundled in this directory were trained
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
