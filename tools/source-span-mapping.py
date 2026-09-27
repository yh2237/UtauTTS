"""録音の音素列から合成対象の区間候補と確認用WAVを作る。"""
import argparse
import hashlib
import importlib.util
import math
from pathlib import Path
import wave

spec = importlib.util.spec_from_file_location('source_alignment', Path(__file__).with_name('source-phone-alignment.py'))
alignment = importlib.util.module_from_spec(spec)
spec.loader.exec_module(alignment)


def file_hash(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def sequence_matches(sequence, wanted):
    return [i for i in range(len(sequence)-len(wanted)+1) if sequence[i:i+len(wanted)] == wanted] if wanted else []


def propose(report_path, out):
    report = alignment.read(report_path)
    rows, rejected = [], []
    for unit in report['units']:
        try:
            wanted = unit.get('assigned_coda_phones', [])
            chinese_syllable=report['language']=='zh' and unit['role']=='mora' and bool(wanted)
            if chinese_syllable:wanted=[p['symbol'] for p in unit['requested_context']]
            if not wanted:
                raise ValueError('no assigned coda sequence')
            source = unit.get('forced_phone_intervals', [])
            source_matches = sequence_matches([p['symbol'] for p in source], wanted)
            if len(source_matches) != 1:
                raise ValueError('missing or ambiguous source sequence')
            context = unit['requested_context']
            target_matches = [i for i in sequence_matches([p['symbol'] for p in context], wanted)
                              if chinese_syllable or all(p['role'] == 'coda' for p in context[i:i+len(wanted)])]
            if len(target_matches) != 1:
                raise ValueError('missing or ambiguous requested coda sequence')
            start, target = source_matches[0], target_matches[0]
            rows.append(dict(unit_index=unit['unit_index'], source_sha256=unit['analysis']['source_sha256'],
                             alignment_sha256=unit['phone_alignment']['alignment_sha256'],
                             mappings=[dict(source_phone_index=start+i, requested_phone_index=target+i) for i in range(len(wanted))],
                             left_context_count=min(1, start), right_context_count=0))
        except (ValueError, KeyError) as error:
            rejected.append(dict(unit_index=unit['unit_index'], alias=unit['alias'], reason=str(error)))
    value = dict(version=1, report_sha256=file_hash(report_path), proposal_status='unverified-unique-sequence-match',
                 units=rows, rejected=rejected)
    alignment.write(out, value)
    return value


def checked_indices(mappings, key, length):
    indices = [mapping[key] for mapping in mappings]
    if not indices or any(type(i) is not int or i < 0 or i >= length for i in indices):
        raise ValueError('invalid phone index')
    if indices != list(range(indices[0], indices[0]+len(indices))):
        raise ValueError('phone indices must be unique, ordered and contiguous')
    return indices


def select_unit(unit, request, language):
    phones, context = unit['forced_phone_intervals'], unit['requested_context']
    metadata = unit['phone_alignment']
    if request['source_sha256'] != unit['analysis']['source_sha256'] or request['source_sha256'] != metadata['source_sha256']:
        raise ValueError('selection source identity mismatch')
    if request['alignment_sha256'] != metadata['alignment_sha256'] or metadata['kind'] != 'forced':
        raise ValueError('selection alignment identity mismatch')
    check = alignment.acoustic_audit(unit, language)
    source_indices = checked_indices(request['mappings'], 'source_phone_index', len(phones))
    requested_indices = checked_indices(request['mappings'], 'requested_phone_index', len(context))
    selected, targets = [phones[i] for i in source_indices], [context[i] for i in requested_indices]
    if [p['symbol'] for p in selected] != [p['symbol'] for p in targets]:
        raise ValueError('selected source and requested phone sequence mismatch')
    assigned = unit.get('assigned_coda_phones', [])
    chinese_syllable=language=='zh' and unit['role']=='mora' and bool(assigned)
    if chinese_syllable:
        if [p['symbol'] for p in targets] != [p['symbol'] for p in context]:raise ValueError('Chinese selection must cover the whole syllable')
        assigned=[]
    if assigned and ([p['symbol'] for p in targets] != assigned or any(p['role'] != 'coda' for p in targets)):
        raise ValueError('selection must cover exactly the assigned coda phones')
    previous = -math.inf
    rows = []
    for source_index, target_index, phone, target in zip(source_indices, requested_indices, selected, targets):
        start, duration = target['start_ms'], target['duration_ms']
        if not all(isinstance(x, (int, float)) and math.isfinite(x) for x in (start, duration)) or start < 0 or duration <= 0 or start < previous - .001:
            raise ValueError('invalid requested phone timing')
        previous = start+duration
        source_duration = phone['end_ms']-phone['start_ms']
        rows.append(dict(source_phone_index=source_index, requested_phone_index=target_index, symbol=phone['symbol'],
                         source_start_ms=phone['start_ms'], source_end_ms=phone['end_ms'],
                         requested_start_ms=start, requested_end_ms=previous,
                         duration_ratio=duration/source_duration))
    left, right = request.get('left_context_count', 0), request.get('right_context_count', 0)
    if type(left) is not int or type(right) is not int or left < 0 or right < 0 or left > source_indices[0] or source_indices[-1]+right >= len(phones):
        raise ValueError('invalid adjacent context count')
    context_indices = list(range(source_indices[0]-left, source_indices[-1]+right+1))
    warnings = list(check['warnings'])
    if any(not .5 <= row['duration_ratio'] <= 2 for row in rows):
        warnings.append('duration-ratio-outside-experimental-range')
    start, end = selected[0]['start_ms'], selected[-1]['end_ms']
    landmarks = []
    for landmark in unit['analysis']['landmarks']:
        if start <= landmark['source_ms'] < end:
            record = dict(landmark)
            record['fully_inside_core'] = landmark['source_ms']+landmark['duration_ms'] <= end
            landmarks.append(record)
            if not record['fully_inside_core']:
                warnings.append('landmark-extends-past-selected-core')
    return dict(unit_index=unit['unit_index'], position=unit['position'], alias=unit['alias'], role=unit['role'],
                source_sha256=request['source_sha256'], alignment_sha256=request['alignment_sha256'],
                acoustic_model=metadata['acoustic_model'], selection_status='unverified', training_eligible=False,
                source_phone_indices=source_indices, adjacent_context_phone_indices=[i for i in context_indices if i not in source_indices],
                excluded_phone_indices=[i for i in range(len(phones)) if i not in context_indices],
                source_clip_phone_intervals=phones, mappings=rows, warnings=sorted(set(warnings)),
                core_start_ms=start, core_end_ms=end,
                context_start_ms=phones[context_indices[0]]['start_ms'], context_end_ms=phones[context_indices[-1]]['end_ms'],
                landmark_candidates=landmarks)


def write_clip(source, destination, start_ms, end_ms):
    with wave.open(str(source), 'rb') as audio:
        rate, channels = audio.getframerate(), audio.getnchannels()
        frames = audio.getnframes()
        first = max(0, int(math.floor(start_ms*rate/1000)))
        last = min(frames, int(math.ceil(end_ms*rate/1000)))
        if first >= last or first >= frames:
            raise ValueError('empty selected PCM interval')
        audio.setpos(first)
        pcm = audio.readframes(last-first)
    with wave.open(str(destination), 'wb') as audio:
        audio.setnchannels(channels); audio.setsampwidth(2); audio.setframerate(rate); audio.writeframes(pcm)
    digest, duration = alignment.clip_identity(destination)
    return dict(path=destination.name, source_start_sample=first, source_end_sample=last, sample_rate=rate,
                actual_start_ms=first*1000/rate, actual_end_ms=last*1000/rate, duration_ms=duration, pcm_sha256=digest)


def select(report_path, requests_path, out):
    report_path, out = Path(report_path), Path(out)
    report, requests = alignment.read(report_path), alignment.read(requests_path)
    if requests['report_sha256'] != file_hash(report_path):
        raise ValueError('observation report changed since span proposal')
    units = {u['unit_index']: u for u in report['units']}
    rows, sources, seen = [], [], set()
    for request in requests['units']:
        index = request['unit_index']
        if index not in units or index in seen:
            raise ValueError('unknown or duplicate source unit')
        seen.add(index)
        unit = units[index]
        source = (report_path.parent / unit['source_clip']).resolve()
        digest, duration = alignment.clip_identity(source)
        if digest != request['source_sha256'] or abs(duration-unit['analysis']['duration_ms']) > .01:
            raise ValueError('selected source PCM changed')
        rows.append(select_unit(unit, request, report['language']))
        sources.append(source)
    if not rows:
        raise ValueError('no unambiguous spans selected')
    if out.exists():
        raise ValueError('use a fresh output directory')
    out.mkdir(parents=True)
    for row, source in zip(rows, sources):
        row['original_clip'] = str(source)
        for kind in ('core', 'context'):
            destination = out / f"unit-{row['unit_index']:03d}-{kind}.wav"
            row[kind+'_clip'] = write_clip(source, destination, row[kind+'_start_ms'], row[kind+'_end_ms'])
    result = dict(version=1, language=report['language'], time_origin='oto-offset',
                  observation_report_sha256=file_hash(report_path), selection_request_sha256=file_hash(requests_path),
                  requested_timing_kind='synthesis-plan-not-natural-reference',
                  mapping_kind='unverified-source-to-requested-phone-span', training_eligible_units=0, units=rows)
    alignment.write(out/'spans.json', result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('propose')
    p.add_argument('--report', required=True); p.add_argument('--out', required=True)
    p = commands.add_parser('select')
    p.add_argument('--report', required=True); p.add_argument('--requests', required=True); p.add_argument('--out', required=True)
    args = parser.parse_args()
    if args.command == 'propose':
        result = propose(args.report, args.out)
        print(f"proposed {len(result['units'])} spans; {len(result['rejected'])} units remain unmapped")
    else:
        result = select(args.report, args.requests, args.out)
        print(f"exported {len(result['units'])} unverified spans: {args.out}")


if __name__ == '__main__':
    main()
