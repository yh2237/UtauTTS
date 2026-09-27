"""整列済み自然音声から時間長・音量・3点ピッチを学習する。検証・試験データは学習しない。"""
import argparse
from collections import Counter
import hashlib
import json
import math
from pathlib import Path
import numpy as np


def validate(records, language):
    if not records or language not in ('en', 'zh'):
        raise ValueError('aligned en/zh records required')
    identities, splits = set(), {}
    for r in records:
        if r.get('version') != 1 or r.get('feature_version') != 1 or r.get('language') != language:
            raise ValueError('unsupported record schema or mixed languages')
        if r.get('kind') != 'natural' or r.get('alignment') not in ('manual', 'forced'):
            raise ValueError('unaligned/generated data cannot be natural-speech training labels')
        for key in ('id', 'speaker', 'corpus', 'license', 'text', 'audio_sha256'):
            if not r.get(key):
                raise ValueError(f'missing {key}')
        if r['id'] in identities or r.get('split') not in ('train', 'validation', 'test'):
            raise ValueError('duplicate ID or missing split')
        identities.add(r['id'])
        for key in (('speaker', r['speaker']), ('audio', r['audio_sha256']), ('text', r['text'])):
            if key in splits and splits[key] != r['split']:
                raise ValueError(f'train/held-out leakage: {key}')
            splits[key] = r['split']
        if not r.get('phones'):
            raise ValueError('empty phone record')
        for p in r['phones']:
            if not p.get('symbol') or not p.get('features') or 'bias' not in p['features'] or len(set(p['features'])) != len(p['features']):
                raise ValueError('invalid phone features')
            for key in ('baseline_ms', 'duration_ms'):
                value = p.get(key)
                if not isinstance(value, (int, float)) or not math.isfinite(value) or value <= 0:
                    raise ValueError(f'invalid measured {key}')
            pitch = p.get('pitch_cents')
            if pitch is not None and (len(pitch) != 3 or not all(math.isfinite(v) for v in pitch)):
                raise ValueError('invalid pitch knots')
            energy = p.get('energy_log_ratio')
            if energy is not None and not math.isfinite(energy):
                raise ValueError('invalid energy')
    if not any(r['split'] == 'train' for r in records) or not any(r['split'] == 'validation' for r in records):
        raise ValueError('separate training and validation speakers required')


def design(rows, index):
    x = np.zeros((len(rows), len(index)))
    for i, row in enumerate(rows):
        for feature in row['features']:
            if feature in index:
                x[i, index[feature]] = 1
    return x


def fit_head(rows, index, target, ridge):
    chosen = [(row, target(row)) for row in rows if target(row) is not None]
    if len(chosen) < 5:
        return None
    x = design([row for row, _ in chosen], index)
    y = np.array([value for _, value in chosen])
    weights = np.linalg.solve(x.T @ x + ridge * np.eye(len(index)), x.T @ y)
    return {feature: float(weights[i]) for feature, i in index.items()}


def train(records, language, model_id, ridge=10.):
    validate(records, language)
    if not math.isfinite(ridge) or ridge <= 0:
        raise ValueError('positive finite ridge regularization required')
    training = [p for r in records if r['split'] == 'train' for p in r['phones']]
    index = {f: i for i, f in enumerate(sorted({f for p in training for f in p['features']}))}
    duration = fit_head(training, index, lambda p: math.log(p['duration_ms']/p['baseline_ms']), ridge)
    if duration is None:
        raise ValueError('at least five training phones required')
    model = {'version': 1, 'feature_version': 1, 'id': model_id, 'language': language,
             'corpus': '; '.join(sorted({r['corpus'] for r in records})),
             'license': '; '.join(sorted({r['license'] for r in records})), 'training_data_kind': 'natural',
             'phone_counts': dict(Counter(p['symbol'].lower() for p in training)), 'duration_log_ratio': duration}
    energy = fit_head(training, index, lambda p: p.get('energy_log_ratio'), ridge)
    if energy is not None:
        model['energy_log_ratio'] = energy
    pitch = [fit_head(training, index, lambda p, k=k: None if p.get('pitch_cents') is None else p['pitch_cents'][k], ridge) for k in range(3)]
    if all(head is not None for head in pitch):
        model['pitch_cents'] = pitch
        model['pitch_phone_counts'] = dict(Counter(p['symbol'].lower() for p in training if p.get('pitch_cents') is not None))
    metrics = {}
    for split in ('validation', 'test'):
        held = [p for r in records if r['split'] == split for p in r['phones']]
        if not held:
            continue
        actual, baseline, predicted = [], [], []
        pitch_error, energy_error = [], []
        covered = 0
        for p in held:
            known = model['phone_counts'].get(p['symbol'].lower(), 0) >= 5
            covered += known
            residual = sum(duration.get(f, 0) for f in p['features']) if known else 0.
            value = p['baseline_ms'] * math.exp(max(math.log(.5), min(math.log(2), residual)))
            predicted.append(max(8, min(500, value)) if known else p['baseline_ms'])
            actual.append(p['duration_ms']); baseline.append(p['baseline_ms'])
            if model.get('pitch_phone_counts', {}).get(p['symbol'].lower(), 0) >= 5 and 'pitch_cents' in model and p.get('pitch_cents') is not None:
                pitch_error.extend(abs(max(-300, min(300, sum(model['pitch_cents'][k].get(f, 0) for f in p['features']))) - p['pitch_cents'][k]) for k in range(3))
            if known and energy is not None and p.get('energy_log_ratio') is not None:
                energy_error.append(abs(max(math.log(.7), min(math.log(1.3), sum(energy.get(f, 0) for f in p['features']))) - p['energy_log_ratio']))
        metrics[split] = {'phones': len(held), 'covered_phones': covered,
                          'baseline_duration_mae_ms': float(np.mean(np.abs(np.array(actual)-baseline))),
                          'model_duration_mae_ms': float(np.mean(np.abs(np.array(actual)-predicted))),
                          'pitch_mae_cents': float(np.mean(pitch_error)) if pitch_error else None,
                          'energy_log_mae': float(np.mean(energy_error)) if energy_error else None}
    model['evaluation'] = metrics
    model['status'] = 'experimental-requires-listening'
    return model


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('corpus', type=Path)
    parser.add_argument('--language', required=True, choices=('en', 'zh'))
    parser.add_argument('--id', required=True)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--ridge', type=float, default=10.)
    args = parser.parse_args()
    records = [json.loads(line) for line in args.corpus.read_text(encoding='utf-8-sig').splitlines() if line.strip()]
    model = train(records, args.language, args.id, args.ridge)
    model['training_manifest_sha256'] = hashlib.sha256(args.corpus.read_bytes()).hexdigest()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(model, ensure_ascii=False, indent=2, allow_nan=False)+'\n', encoding='utf-8')
    print(json.dumps(model['evaluation'], indent=2))


if __name__ == '__main__':
    main()
