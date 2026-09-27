"""MFAの音素時刻をUtauTTS音節へ対応付け、英語TCNの教師JSONLを作る。"""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import subprocess

def normalize(phone):
    return re.sub(r'[012]$', '', phone).lower()

def build(row, alignment, helper):
    tiers = alignment['tiers']
    words = next(v['entries'] for k,v in tiers.items() if k.endswith('words'))
    phones = next(v['entries'] for k,v in tiers.items() if k.endswith('phones'))
    if any(p[2] == 'spn' for p in phones): raise ValueError('unknown phone')
    phones = [p for p in phones if p[2] not in ('', 'sil', 'sp')]
    if not phones or any(b <= a for a,b,_ in phones): raise ValueError('invalid phone interval')
    groups = []
    for start,end,word in words:
        if word in ('', '<eps>', 'sil'): continue
        group = [p for p in phones if start <= (p[0]+p[1])/2 < end]
        if not group or word == '<unk>': raise ValueError('unknown/unmapped word')
        groups.append(group)
    if sum(map(len,groups)) != len(phones): raise ValueError('unmapped phones')
    reading = ' | '.join(' '.join(p[2] for p in group) for group in groups)
    helper.stdin.write(json.dumps({'reading':reading})+'\n'); helper.stdin.flush()
    parsed = json.loads(helper.stdout.readline())
    if 'error' in parsed: raise ValueError(parsed['error'])
    tokens, offset = [], 0
    for token in parsed['tokens']:
        if token['pause']: raise ValueError('unexpected parsed pause')
        count = len(token['phones'])
        aligned = phones[offset:offset+count]
        if [normalize(p[2]) for p in aligned] != [p['symbol'] for p in token['phones']]: raise ValueError('phone sequence mismatch')
        start,end = aligned[0][0]*1000,aligned[-1][1]*1000
        if not 20 <= end-start <= 1500: raise ValueError('extreme syllable duration')
        if tokens and start-tokens[-1]['end_ms'] >= 80:
            tokens.append({'language':'en','pause':True,'start_ms':tokens[-1]['end_ms'],'end_ms':start,'word_index':-1})
        token.update(start_ms=start,end_ms=end)
        tokens.append(token)
        offset += count
    if offset != len(phones): raise ValueError('unused phones')
    audio = Path(row['audio_path'])
    return dict(version=1,id=row['id'],language='en',speaker=row['speaker'],text=row['text'],
                audio_path=str(audio),audio_sha256=hashlib.sha256(audio.read_bytes()).hexdigest(),
                alignment_source='MFA english_us_arpa; matched phoneme intervals',
                start_ms=0,end_ms=alignment['end']*1000,tokens=tokens)

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--manifest',type=Path,required=True); p.add_argument('--alignments',type=Path,required=True)
    p.add_argument('--helper',required=True); p.add_argument('--out',type=Path,required=True)
    p.add_argument('--failed-list',type=Path,required=True)
    args=p.parse_args()
    rows=json.loads(args.manifest.read_text(encoding='utf-8'))
    bad={Path(line.strip()).stem for line in args.failed_list.read_text(encoding='utf-8').splitlines() if line.strip()}
    speakers=sorted({r['speaker'] for r in rows},key=lambda s:hashlib.sha256(s.encode()).hexdigest())
    if len(speakers)<10: raise ValueError('at least 10 speakers required')
    splits={s:('test' if i<3 else 'validation' if i<6 else 'train') for i,s in enumerate(speakers)}
    helper=subprocess.Popen([args.helper],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True,encoding='utf-8')
    accepted,rejected,seen=[],[],{}
    try:
        for row in rows:
            try:
                if row['id'] in bad: raise ValueError('official restoration failure list')
                path=args.alignments/row['speaker']/(row['id']+'.json')
                record=build(row,json.loads(path.read_text(encoding='utf-8')),helper)
                record['split']=splits[row['speaker']]
                key=re.sub(r'\W+','',row['text'].lower())
                if key in seen: raise ValueError('duplicate text')
                seen[key]=record['split']
                accepted.append(record)
            except (ValueError,FileNotFoundError,KeyError,IndexError,StopIteration) as error:
                rejected.append(dict(id=row['id'],reason=str(error)))
    finally:
        helper.stdin.close(); helper.wait()
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in accepted),encoding='utf-8')
    report=dict(accepted=len(accepted),splits=dict(Counter(r['split'] for r in accepted)),speaker_splits=splits,rejected=rejected)
    args.out.with_suffix('.audit.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    print(json.dumps({k:v for k,v in report.items() if k!='rejected'},indent=2))

if __name__=='__main__': main()
