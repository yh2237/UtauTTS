"""原音の区間ライブラリを別の文の音素時刻へ自動で対応付ける。"""
import argparse
import copy
from pathlib import Path

from source_phone_common import load_tool

spans = load_tool('source-span-mapping.py')
alignment = spans.alignment


from source_phone_common import checked_phones


def build_library(paths, out, prefer_last=False):
    entries, replacements = {}, []
    language = None
    for path in paths:
        report = alignment.read(path)
        if report.get('time_origin') != 'oto-offset':
            raise ValueError('oto-offset library required')
        if language is not None and language != report['language']:
            raise ValueError('mixed library languages')
        language = report['language']
        for unit in report['units']:
            digest, duration = alignment.clip_identity(unit['original_clip'])
            if digest != unit['source_sha256']:
                raise ValueError('library source PCM changed')
            phones = unit['source_clip_phone_intervals']
            checked_phones(phones, duration)
            row = dict(source_sha256=digest, duration_ms=duration, phones=phones,
                       acoustic_model=unit['acoustic_model'], alignment_sha256=unit['alignment_sha256'],
                       status='unverified', training_eligible=False,
                       provenance=dict(report=str(Path(path).resolve()), report_sha256=spans.file_hash(path), alias=unit['alias']))
            old = entries.get(digest)
            if old and (old['phones'] != row['phones'] or old['acoustic_model'] != row['acoustic_model']):
                if not prefer_last:
                    raise ValueError('conflicting source hypotheses; choose a library or explicitly prefer the last report')
                replacements.append(dict(source_sha256=digest, previous=old['provenance'], selected=row['provenance']))
            entries[digest] = row
    if not entries:
        raise ValueError('empty source library')
    result = dict(version=1, language=language, time_origin='oto-offset',
                  kind='experimental-unverified-source-phone-library', entries=list(entries.values()), replacements=replacements)
    alignment.write(out, result)
    return result


def attach_library(report_path, library_path):
    report_path = Path(report_path)
    report, library = alignment.read(report_path), alignment.read(library_path)
    if library.get('version') != 1 or library.get('time_origin') != 'oto-offset' or library['language'] != report['language']:
        raise ValueError('incompatible source library')
    entries = {}
    for row in library['entries']:
        digest = row['source_sha256']
        if digest in entries:
            raise ValueError('duplicate library source identity')
        checked_phones(row['phones'], row['duration_ms'])
        entries[digest] = row
    reused, missing = [], []
    for unit in report['units']:
        unit['source_clip'] = str((report_path.parent/unit['source_clip']).resolve())
        # 前の文の対応付けや既存の整列情報を流用しない。
        unit.pop('forced_phone_intervals', None)
        unit.pop('phone_alignment', None)
        if (unit['role'] != 'ending' and not (report['language']=='zh' and unit['role']=='mora')) or not unit.get('assigned_coda_phones'):
            continue
        digest, duration = alignment.clip_identity(unit['source_clip'])
        if digest != unit['analysis']['source_sha256'] or abs(duration-unit['analysis']['duration_ms']) > .01:
            raise ValueError('current source PCM changed')
        row = entries.get(digest)
        if not row:
            missing.append(dict(unit_index=unit['unit_index'], alias=unit['alias'], reason='source-not-in-library'))
            continue
        if abs(duration-row['duration_ms']) > .01:
            raise ValueError('library source duration mismatch')
        unit['forced_phone_intervals'] = copy.deepcopy(row['phones'])
        unit['phone_alignment'] = dict(kind='forced', status='unverified', source_sha256=digest,
                                      acoustic_model=row['acoustic_model'], alignment_sha256=row['alignment_sha256'],
                                      transcript_source='source-library-reuse', library_sha256=spans.file_hash(library_path),
                                      provenance=row['provenance'])
        reused.append(unit['unit_index'])
    report['source_library_audit'] = dict(reused=reused, missing=missing, verified_count=0)
    return report


def map_report(report_path, library_path, out):
    out = Path(out)
    if out.exists():
        raise ValueError('use a fresh output directory')
    report = attach_library(report_path, library_path)
    out.mkdir(parents=True)
    aligned = out/'aligned-observations.json'
    alignment.write(aligned, report)
    proposal = spans.propose(aligned, out/'requests.json')
    endings = {u['unit_index'] for u in report['units'] if (u['role'] == 'ending' or (report['language']=='zh' and u['role']=='mora')) and u.get('assigned_coda_phones')}
    selected = {u['unit_index'] for u in proposal['units']}
    coverage = dict(ending_units=len(endings), mapped_units=len(selected), unmapped_units=len(endings-selected),
                    selected_indices=sorted(selected), library_audit=report['source_library_audit'],
                    rejected=[r for r in proposal['rejected'] if r['unit_index'] in endings],
                    fallback='unmapped-units-keep-existing-oto-timing', training_eligible_units=0)
    alignment.write(out/'coverage.json', coverage)
    if selected:
        spans.select(aligned, out/'requests.json', out/'selected')
    return coverage


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('build')
    p.add_argument('--spans', action='append', required=True)
    p.add_argument('--out', required=True)
    p.add_argument('--prefer-last', action='store_true', help='explicitly choose the last conflicting source hypothesis')
    p = commands.add_parser('map')
    p.add_argument('--report', required=True); p.add_argument('--library', required=True); p.add_argument('--out', required=True)
    args = parser.parse_args()
    if args.command == 'build':
        result = build_library(args.spans, args.out, args.prefer_last)
        print(f"indexed {len(result['entries'])} sources; explicit replacements: {len(result['replacements'])}")
    else:
        result = map_report(args.report, args.library, args.out)
        print(f"mapped {result['mapped_units']}/{result['ending_units']} ending units; others retain existing timing")


if __name__ == '__main__':
    main()
