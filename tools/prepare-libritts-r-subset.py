"""LibriTTS-Rから話者数と発話数を制限して初回実験データを抽出する。"""
import argparse
from collections import defaultdict
import json
from pathlib import Path, PurePosixPath
import tarfile
import wave

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--speakers', type=int, default=32)
    parser.add_argument('--per-speaker', type=int, default=40)
    args = parser.parse_args()
    texts, chosen, counts, speakers = {}, {}, defaultdict(int), []
    args.out.mkdir(parents=True, exist_ok=True)
    try:
        with tarfile.open(args.archive, 'r|gz') as archive:
            for member in archive:
                parts = PurePosixPath(member.name).parts
                if len(parts) != 5 or parts[1] != 'train-clean-100' or not member.isfile(): continue
                speaker, name = parts[2], parts[-1]
                if speaker not in speakers:
                    if len(speakers) >= args.speakers: break
                    speakers.append(speaker)
                if name.endswith('.normalized.txt'):
                    texts[name.removesuffix('.normalized.txt')] = archive.extractfile(member).read().decode('utf-8').strip()
                elif name.endswith('.wav') and counts[speaker] < args.per_speaker:
                    path = args.out / speaker / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(archive.extractfile(member).read())
                    with wave.open(str(path)) as wav: duration = wav.getnframes()/wav.getframerate()
                    if 1 <= duration <= 12:
                        chosen[path.stem] = dict(id=path.stem, speaker=speaker, audio_path=str(path.resolve()), duration_s=duration)
                        counts[speaker] += 1
                    else: path.unlink()
    except (EOFError, tarfile.ReadError) as error:
        raise ValueError('archive incomplete; complete the download before retrying') from error
    rows = []
    for identity, row in chosen.items():
        if identity not in texts: continue
        row['text'] = texts[identity]
        (args.out / row['speaker'] / (identity+'.lab')).write_text(row['text'], encoding='utf-8')
        rows.append(row)
    (args.out / 'manifest.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2), encoding='utf-8')
    print('speakers', len(set(r['speaker'] for r in rows)), 'utterances', len(rows), 'hours', sum(r['duration_s'] for r in rows)/3600, flush=True)

if __name__ == '__main__': main()
