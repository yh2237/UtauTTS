"""原音境界の注釈表と誤差を出力する。時刻はoto.offset基準のms。未注釈は未評価。"""
import argparse
from collections import defaultdict
import hashlib
import json
import math
from pathlib import Path


def export(template):
    rows, hashes = [], {}
    for index, unit in enumerate(template['source_units']):
        if unit.get('silent') or not unit.get('source'):
            continue
        source = Path(unit['source']).resolve()
        if source not in hashes:
            hashes[source] = hashlib.sha256(source.read_bytes()).hexdigest()
        anchors = unit.get('speech_source_anchors_ms', [])
        if len(anchors) < 2:
            continue
        for anchor_index, estimate in enumerate(anchors[1:-1], start=1):
            rows.append({'unit_index': index, 'alias': unit['alias'], 'role': unit['role'],
                         'source': str(source), 'source_sha256': hashes[source],
                         'offset_ms': unit['offset_ms'], 'anchor_index': anchor_index,
                         'estimate_ms': estimate, 'mapping': unit.get('speech_mapping'),
                         'source_end_ms': anchors[-1],
                         'boundary_label': '', 'observed_ms': None, 'annotation_kind': 'unobserved'})
    return {'version': 1, 'id': template['id'], 'time_origin': 'oto.offset', 'boundaries': rows}


def audit(document):
    if document.get('version') != 1 or document.get('time_origin') != 'oto.offset':
        raise ValueError('unsupported boundary review schema/time origin')
    groups, unobserved, verified = defaultdict(list), 0, {}
    identities = set()
    for row in document['boundaries']:
        key = (row['unit_index'], row['anchor_index'])
        if key in identities:
            raise ValueError('duplicate boundary identity')
        identities.add(key)
        value = row.get('observed_ms')
        if value is None:
            unobserved += 1
            continue
        if row.get('annotation_kind') != 'manual' or not row.get('boundary_label'):
            raise ValueError('a manual boundary label is required for observations')
        if not math.isfinite(value) or value < 0 or value > row['source_end_ms'] or not math.isfinite(row['estimate_ms']):
            raise ValueError('invalid boundary time')
        path = Path(row['source'])
        if path not in verified:
            verified[path] = hashlib.sha256(path.read_bytes()).hexdigest()
        if verified[path] != row['source_sha256']:
            raise ValueError('source changed since boundary export')
        groups[(row['boundary_label'], row.get('mapping', 'unknown'))].append(abs(value-row['estimate_ms']))
    metrics = [{'label': label, 'mapping': mapping, 'count': len(errors),
                'mae_ms': sum(errors)/len(errors), 'max_error_ms': max(errors),
                'within_20ms': sum(error <= 20 for error in errors)/len(errors)}
               for (label, mapping), errors in sorted(groups.items())]
    return {'labelled': sum(len(values) for values in groups.values()), 'unobserved': unobserved,
            'metrics': metrics, 'status': 'measured' if metrics else 'awaiting-manual-boundaries'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--export-template', type=Path)
    mode.add_argument('--annotations', type=Path)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    if args.export_template:
        result = export(json.loads(args.export_template.read_text(encoding='utf-8-sig')))
    else:
        result = audit(json.loads(args.annotations.read_text(encoding='utf-8-sig')))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2, allow_nan=False)+'\n', encoding='utf-8')


if __name__ == '__main__':
    main()
