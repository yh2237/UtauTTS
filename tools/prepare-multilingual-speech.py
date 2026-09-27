"""自然音声の実測音素時刻を取り込む。入力形式はdocs/multilingual-learning.md参照。"""
import argparse
import hashlib
import json
import math
import wave
from pathlib import Path
import numpy as np


def read_wave(path):
    with wave.open(str(path), 'rb') as wav:
        if wav.getnchannels() != 1 or wav.getsampwidth() != 2:
            raise ValueError('expected mono 16-bit PCM natural WAV')
        rate = wav.getframerate()
        samples = np.frombuffer(wav.readframes(wav.getnframes()), dtype='<i2').astype(float) / 32768
    return rate, samples


def pitch_track(samples, rate):
    # 無声フレームは欠測にする。
    result = []
    size, hop = round(rate * .04), max(1, round(rate * .01))
    for start in range(0, len(samples) - size + 1, hop):
        block = samples[start:start + size].copy()
        if np.sqrt(np.mean(block * block)) < .003:
            continue
        block -= block.mean()
        lo, hi = max(1, rate // 500), min(size - 2, rate // 60)
        fft = np.fft.rfft(block, n=2 * size)
        corr = np.fft.irfft(fft * fft.conj(), n=2 * size)[:size]
        energy = np.concatenate(([0.], np.cumsum(block * block)))
        lags = np.arange(lo, hi + 1)
        norm = np.sqrt((energy[size - lags]) * (energy[size] - energy[lags]))
        values = corr[lags] / np.maximum(norm, 1e-12)
        peaks = [i for i in range(1, len(values) - 1) if values[i] > values[i-1] and values[i] >= values[i+1]]
        if not peaks:
            continue
        best = max(values[i] for i in peaks)
        if best < .65:
            continue
        chosen = next(i for i in peaks if values[i] >= max(.65, best * .95))
        result.append(((start + size / 2) * 1000 / rate, rate / float(lags[chosen])))
    return result


def prepare(template, observation, root):
    for field in ('id', 'speaker', 'corpus', 'license', 'audio_path'):
        if not observation.get(field):
            raise ValueError(f'missing provenance {field}')
    if observation.get('kind') != 'natural' or observation.get('alignment') not in ('manual', 'forced'):
        raise ValueError('only explicitly aligned natural recordings are training observations')
    if observation.get('split') not in ('train', 'validation', 'test'):
        raise ValueError('explicit train/validation/test split required')
    if template.get('version') != 1 or template.get('feature_version') != 1 or template['id'] != observation['id']:
        raise ValueError('template identity/version mismatch')
    path = (root / observation['audio_path']).resolve()
    rate, samples = read_wave(path)
    total_ms = len(samples) * 1000 / rate
    expected = template['phones']
    actual = observation['phones']
    if len(expected) != len(actual) or not expected:
        raise ValueError('alignment phone count differs from linguistic template')
    last = 0.
    rms_values, spans = [], []
    for want, got in zip(expected, actual):
        for field in ('position', 'phone_index', 'symbol'):
            if got[field] != want[field]:
                raise ValueError(f'alignment mismatch at {field}: {got} vs {want}')
        start, end = float(got['start_ms']), float(got['end_ms'])
        if not math.isfinite(start + end) or start < last - 1e-6 or end <= start or end > total_ms + 1e-6:
            raise ValueError('nonfinite, overlapping, empty or out-of-recording phone interval')
        block = samples[round(start * rate / 1000):round(end * rate / 1000)]
        if not len(block):
            raise ValueError('empty measured phone')
        rms_values.append(max(1e-6, float(np.sqrt(np.mean(block * block)))))
        spans.append((start, end))
        last = end
    track = pitch_track(samples, rate)
    speech_track = [(ms, hz) for ms, hz in track if any(start <= ms <= end for start, end in spans)]
    median_f0 = float(np.median([hz for _, hz in speech_track])) if speech_track else None
    median_energy = float(np.median(rms_values))
    rows = []
    for want, span, rms in zip(expected, spans, rms_values):
        start, end = span
        pitch = None
        voiced = [(ms, hz) for ms, hz in speech_track if start <= ms <= end]
        if len(voiced) >= 3 and median_f0:
            pitch = []
            for fraction in (0., .5, 1.):
                ms = start + (end - start) * fraction
                nearest = min(voiced, key=lambda item: abs(item[0] - ms))
                if abs(nearest[0] - ms) > max(20, (end - start) * .2):
                    pitch = None
                    break
                pitch.append(1200 * math.log2(nearest[1] / median_f0))
        rows.append({**want, 'duration_ms': end - start,
                     'pitch_cents': pitch, 'energy_log_ratio': math.log(rms / median_energy)})
    return {'version': 1, 'feature_version': 1, 'id': template['id'], 'language': template['language'],
            'text': template['text'], 'speaker': observation['speaker'], 'split': observation['split'],
            'kind': 'natural', 'corpus': observation['corpus'], 'license': observation['license'],
            'alignment': observation['alignment'], 'audio_sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'pitch_source': 'normalized-autocorrelation-v1', 'phones': rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('templates', nargs='+', type=Path)
    parser.add_argument('--observations', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    observations = json.loads(args.observations.read_text(encoding='utf-8-sig'))['utterances']
    by_id = {row['id']: row for row in observations}
    if len(by_id) != len(observations):
        raise ValueError('duplicate observation IDs')
    records = []
    for path in args.templates:
        template = json.loads(path.read_text(encoding='utf-8-sig'))
        records.append(prepare(template, by_id[template['id']], args.observations.parent))
    if len({r['id'] for r in records}) != len(records):
        raise ValueError('duplicate template IDs')
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(''.join(json.dumps(r, ensure_ascii=False, allow_nan=False) + '\n' for r in records), encoding='utf-8')


if __name__ == '__main__':
    main()
