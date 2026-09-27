"""原音の明示音素列をMFAへ渡し、未確認の整列区間を取り込む。"""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import shutil
import wave


def read(path):
    return json.loads(Path(path).read_text(encoding='utf-8'))


def write(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')


def normalize(phone):
    return re.sub(r'[012]$', '', phone).lower()


def clip_identity(path):
    with wave.open(str(path), 'rb') as audio:
        if audio.getsampwidth() != 2 or audio.getcomptype() != 'NONE':
            raise ValueError('16-bit PCM source required')
        rate, channels = audio.getframerate(), audio.getnchannels()
        pcm = audio.readframes(audio.getnframes())
        digest = hashlib.sha256(f'v1/{rate}/{channels}/'.encode() + pcm).hexdigest()
        return digest, audio.getnframes() * 1000 / rate


def prepare(report_path, requests_path, out):
    report_path, out = Path(report_path), Path(out)
    report, requests = read(report_path), read(requests_path)
    units = {u['unit_index']: u for u in report['units']}
    rows, seen = [], set()
    for request in requests['units']:
        index, phones = request['unit_index'], request['phones']
        if index in seen or index not in units:
            raise ValueError('duplicate or unknown source unit')
        seen.add(index)
        if not phones or any(not isinstance(p, str) or not p or not p.isprintable() or any(c.isspace() for c in p) for p in phones):
            raise ValueError('explicit nonempty acoustic-model phone sequence required')
        unit = units[index]
        source = (report_path.parent / unit['source_clip']).resolve()
        digest, duration = clip_identity(source)
        if digest != unit['analysis']['source_sha256']:
            raise ValueError('source PCM changed since observation')
        name = f'source{index:04d}'
        destination = out / 'corpus' / f'{name}.wav'
        if destination.exists():
            raise ValueError('output source already exists; use a fresh directory')
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, destination)
        destination.with_suffix('.lab').write_text(name, encoding='utf-8')
        rows.append(dict(unit_index=index, id=name, alias=unit['alias'], phones=phones,
                         source_sha256=digest, duration_ms=duration))
        if 'canonical_phones' in request:
            canonical=request['canonical_phones']
            if len(canonical)!=len(phones) or any(not isinstance(p,str) or not re.fullmatch(r'[a-z]+',p) for p in canonical):
                raise ValueError('invalid canonical phone sequence')
            rows[-1]['canonical_phones']=canonical
    if not rows:
        raise ValueError('no source units requested')
    (out / 'dictionary.dict').write_text(''.join(r['id'] + '\t' + ' '.join(r['phones']) + '\n' for r in rows), encoding='utf-8')
    manifest = dict(version=1, report=str(report_path.resolve()), language=report['language'],
                    time_origin='oto-offset', units=rows)
    write(out / 'manifest.json', manifest)
    return manifest


def intervals(alignment, row):
    tiers = [v['entries'] for k, v in alignment['tiers'].items() if k == 'phones' or k.endswith(' phones')]
    if len(tiers) != 1:
        raise ValueError('exactly one phone tier required')
    result, previous = [], 0.0
    for start, end, label in tiers[0]:
        if not all(isinstance(x, (int, float)) and math.isfinite(x) for x in (start, end)):
            raise ValueError('non-finite interval')
        start, end = start * 1000, end * 1000
        if start < previous - .001 or end <= start or start < 0 or end > row['duration_ms'] + 1:
            raise ValueError('non-monotone or out-of-source interval')
        previous = end
        if label in ('', 'sil', 'sp'):
            continue
        if label in ('spn', '<unk>'):
            raise ValueError('unknown aligned phone')
        result.append(dict(symbol=normalize(label), acoustic_label=label, start_ms=start, end_ms=end))
    if [p['symbol'] for p in result] != [normalize(p) for p in row['phones']]:
        raise ValueError('aligned phone sequence mismatch')
    if 'canonical_phones' in row:
        if len(row['canonical_phones'])!=len(result):raise ValueError('canonical phone count mismatch')
        for phone,canonical in zip(result,row['canonical_phones']):phone['symbol']=canonical
    return result


def import_alignments(manifest_path, alignments, model, out):
    manifest_path, alignments = Path(manifest_path), Path(alignments)
    manifest = read(manifest_path)
    report_path = Path(manifest['report'])
    report = read(report_path)
    units = {u['unit_index']: u for u in report['units']}
    accepted, rejected = [], []
    for row in manifest['units']:
        index = row['unit_index']
        if index not in units:
            raise ValueError('source unit removed from report')
        unit = units[index]
        for clip in (report_path.parent / unit['source_clip'], manifest_path.parent / 'corpus' / (row['id'] + '.wav')):
            digest, duration = clip_identity(clip)
            if digest != row['source_sha256'] or digest != unit['analysis']['source_sha256'] or abs(duration-row['duration_ms']) > .01:
                raise ValueError('source identity mismatch')
        try:
            path = alignments / (row['id'] + '.json')
            phones = intervals(read(path), row)
            unit['forced_phone_intervals'] = phones
            unit['phone_alignment'] = dict(kind='forced', status='unverified', acoustic_model=model,
                                          source_sha256=row['source_sha256'], alignment_sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
                                          transcript_source='explicit-source-phone-request')
            accepted.append(index)
        except (ValueError, FileNotFoundError, KeyError) as error:
            unit.pop('forced_phone_intervals', None)
            unit.pop('phone_alignment', None)
            rejected.append(dict(unit_index=index, reason=str(error)))
    # 観測欄は人手確認まで変更しない。
    report['alignment_audit'] = dict(accepted=accepted, rejected=rejected, verified_count=0)
    # 出力先を変えても元の確認用音声を参照できるようにする。
    for unit in report['units']:
        unit['source_clip'] = str((report_path.parent / unit['source_clip']).resolve())
    write(out, report)
    return report


def evaluate(report_path, references_path, out):
    report, references = read(report_path), read(references_path)
    units = {u['unit_index']: u for u in report['units']}
    errors, seen, per_phone = [], set(), {}
    for reference in references['units']:
        index = reference['unit_index']
        if index in seen or reference['annotation_kind'] != 'manual':
            raise ValueError('unique manual references required')
        seen.add(index)
        unit = units[index]
        if reference['source_sha256'] != unit['analysis']['source_sha256']:
            raise ValueError('reference source mismatch')
        digest, _ = clip_identity(Path(report_path).parent / unit['source_clip'])
        if digest != reference['source_sha256']:
            raise ValueError('reference audio changed')
        predicted = unit['forced_phone_intervals']
        actual = reference['phones']
        if [p['symbol'] for p in actual] != [p['symbol'] for p in predicted]:
            raise ValueError('reference phone sequence mismatch')
        previous = 0
        for manual, forced in zip(actual, predicted):
            a, b = manual['start_ms'], manual['end_ms']
            if not all(isinstance(x, (int, float)) and math.isfinite(x) for x in (a, b)) or a < previous or b <= a or b > unit['analysis']['duration_ms']:
                raise ValueError('invalid manual interval')
            previous = b
            pair = [abs(a-forced['start_ms']), abs(b-forced['end_ms'])]
            errors.extend(pair)
            per_phone.setdefault(manual['symbol'], []).extend(pair)
    def metrics(values):
        return dict(boundaries=len(values), mae_ms=sum(values)/len(values), max_ms=max(values), within_20_ms=sum(e <= 20 for e in values)/len(values)) if values else dict(boundaries=0, mae_ms=None, max_ms=None, within_20_ms=None)
    result = dict(manual_units=len(seen), unreviewed_units=len(units)-len(seen), overall=metrics(errors), by_phone={p:metrics(v) for p,v in per_phone.items()})
    write(out, result)
    return result


def acoustic_audit(unit, language):
    analysis = unit['analysis']
    duration = analysis['duration_ms']
    phones = unit.get('forced_phone_intervals', [])
    if not phones:
        return dict(status='not-aligned', training_eligible=False, warnings=[], phones=[])
    if analysis.get('periodicity_method') != 'normalized-autocorrelation-local-peak-80-500hz-v2':
        raise ValueError('regenerate source observations with local-peak periodicity v2')
    # 整列区間を再検証し、未被覆区間を求める。
    previous, gaps = 0.0, []
    for phone in phones:
        start, end = phone['start_ms'], phone['end_ms']
        if not all(math.isfinite(x) for x in (start, end)) or start < previous - .001 or end <= start or end > duration + 1:
            raise ValueError('invalid stored alignment interval')
        if start > previous:
            gaps.append((previous, start))
        previous = end
    if previous < duration:
        gaps.append((previous, duration))
    frames = analysis['frames']
    previous = 0.0
    for frame in frames:
        start, end = frame['start_ms'], frame['end_ms']
        values = [start, end, frame['rms_dbfs'], frame['periodicity'], frame['zero_crossing_rate']]
        if not all(math.isfinite(x) for x in values) or abs(start-previous) > .001 or end <= start or end > duration + .001:
            raise ValueError('invalid acoustic frame coverage')
        if not 0 <= frame['periodicity'] <= 1 or not 0 <= frame['zero_crossing_rate'] <= 1:
            raise ValueError('invalid acoustic frame feature')
        previous = end
    if not frames or abs(previous-duration) > .001:
        raise ValueError('incomplete acoustic frame coverage')

    def overlap(frame, start, end):
        return max(0.0, min(frame['end_ms'], end)-max(frame['start_ms'], start))

    threshold = analysis['low_energy_threshold_dbfs']
    # 前方窓のにじみと境界付近の不確かさを除く。
    guard = max(40.0, analysis['window_ms'])
    uncovered = []
    warnings = []
    for start, end in gaps:
        checked_start, checked_end = start, end
        if start > 0:
            checked_start += guard
        if end < duration:
            checked_end -= guard
        checked_start = min(end, checked_start)
        checked_end = max(checked_start, checked_end)
        active = periodic = 0.0
        for frame in frames:
            weight = overlap(frame, checked_start, checked_end)
            if frame['rms_dbfs'] >= threshold:
                active += weight
                if frame['periodicity'] >= .65:
                    periodic += weight
        issue = dict(start_ms=start, end_ms=end, checked_start_ms=checked_start,
                     checked_end_ms=checked_end, active_ms=active, periodic_ms=periodic)
        uncovered.append(issue)
        if periodic >= 30:
            warnings.append('periodic-activity-outside-alignment')
        elif active >= 30:
            warnings.append('acoustic-activity-outside-alignment')

    details = []
    for phone in phones:
        start, end = phone['start_ms'], phone['end_ms']
        power = periodic = active = 0.0
        for frame in frames:
            weight = overlap(frame, start, end)
            power += weight * 10**(frame['rms_dbfs']/10)
            if frame['rms_dbfs'] >= threshold:
                active += weight
                if frame['periodicity'] >= .65:
                    periodic += weight
        detail = dict(symbol=phone['symbol'], start_ms=start, end_ms=end,
                      windowed_rms_dbfs=10*math.log10(max(1e-14, power/(end-start))),
                      active_ms=active, periodic_ms=periodic)
        if language == 'en' and phone['symbol'] in ('p', 'b', 't', 'd', 'k', 'g'):
            candidates = [dict(landmark) for landmark in analysis['landmarks']
                          if start <= landmark['source_ms'] < end]
            detail['stop_landmark_candidates'] = candidates
            strong = [c for c in candidates if c['relative_to_peak_db'] >= -25 and c['heuristic_score'] >= .4]
            detail['strong_candidate_count'] = len(strong)
            if not strong:
                warnings.append('stop-without-strong-landmark-candidate')
        details.append(detail)
    return dict(status='needs-review' if warnings else 'acoustically-consistent-unverified',
                training_eligible=False, warnings=sorted(set(warnings)), boundary_guard_ms=guard,
                uncovered_intervals=uncovered, phones=details)


def audit(report_path, out):
    report_path = Path(report_path)
    report = read(report_path)
    rows = []
    for unit in report['units']:
        if unit.get('forced_phone_intervals'):
            digest, duration = clip_identity(report_path.parent / unit['source_clip'])
            analysis = unit['analysis']
            if digest != analysis['source_sha256'] or abs(duration-analysis['duration_ms']) > .01:
                raise ValueError('source identity mismatch in acoustic audit')
            alignment = unit.get('phone_alignment', {})
            if alignment.get('source_sha256') != digest or alignment.get('kind') != 'forced':
                raise ValueError('alignment identity mismatch in acoustic audit')
        result = acoustic_audit(unit, report['language'])
        result.update(unit_index=unit['unit_index'], alias=unit['alias'], source_sha256=unit['analysis']['source_sha256'])
        rows.append(result)
    value = dict(version=1, audit_kind='acoustic-consistency-not-boundary-accuracy', language=report['language'],
                 observation_report_sha256=hashlib.sha256(report_path.read_bytes()).hexdigest(),
                 parameters=dict(periodicity_threshold=.65, review_activity_ms=30,
                                 strong_landmark_min_score=.4, strong_landmark_min_relative_db=-25,
                                 minimum_boundary_guard_ms=40),
                 aligned_units=sum(r['status'] != 'not-aligned' for r in rows),
                 review_units=sum(r['status'] == 'needs-review' for r in rows), verified_units=0,
                 training_eligible_units=0, units=rows)
    write(out, value)
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('prepare')
    p.add_argument('--report', required=True); p.add_argument('--requests', required=True); p.add_argument('--out', required=True)
    p = commands.add_parser('import')
    p.add_argument('--manifest', required=True); p.add_argument('--alignments', required=True); p.add_argument('--model', required=True); p.add_argument('--out', required=True)
    p = commands.add_parser('evaluate')
    p.add_argument('--report', required=True); p.add_argument('--references', required=True); p.add_argument('--out', required=True)
    p = commands.add_parser('audit')
    p.add_argument('--report', required=True); p.add_argument('--out', required=True)
    args = parser.parse_args()
    if args.command == 'prepare': result = prepare(args.report, args.requests, args.out); print(f"prepared {len(result['units'])} source clips")
    elif args.command == 'import': print(json.dumps(import_alignments(args.manifest, args.alignments, args.model, args.out)['alignment_audit']))
    elif args.command == 'evaluate': print(json.dumps(evaluate(args.report, args.references, args.out)))
    else:
        result = audit(args.report, args.out)
        print(json.dumps({k:v for k,v in result.items() if k != 'units'}))


if __name__ == '__main__':
    main()
