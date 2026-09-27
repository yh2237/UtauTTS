"""中国語の単母音＋鼻音韻尾の原音をMFAで整列する。"""
import argparse
import re
from pathlib import Path

from source_phone_common import load_tool, observed_source

auto = load_tool('source-span-auto.py')
alignment=auto.alignment
INITIALS={'b':'p','p':'pʰ','m':'m','f':'f','d':'t','t':'tʰ','n':'n','l':'l','g':'k','k':'kʰ','h':'x','j':'tɕ','q':'tɕʰ','x':'ɕ','s':'s','z':'ts','c':'tsʰ'}
VOWELS={'a':'a','e':'ə','i':'i','u':'u','v':'y'}


def phones_for_unit(unit):
    context=unit['requested_context']
    if unit['role']!='mora' or unit.get('assigned_coda_phones') not in (['n'],['ng']):return None
    if [p['role'] for p in context] not in (['onset','nucleus','coda'],['nucleus','coda']):return None
    canonical=[p['symbol'] for p in context]
    vowel=next(p['symbol'] for p in context if p['role']=='nucleus')
    if vowel not in VOWELS:return None
    # 単純な音節名に一致する原音だけを対象にする。
    expected=''.join(canonical)
    alias=re.sub(r'[A-G][#b]?-?\d+$','',unit['alias']).strip('- ')
    if alias!=expected:return None
    labels=[]
    for p in context:
        if p['role']=='onset':
            if p['symbol'] not in INITIALS:return None
            labels.append(INITIALS[p['symbol']])
        elif p['role']=='nucleus':labels.append(VOWELS[p['symbol']])
        else:labels.append('ŋ' if p['symbol']=='ng' else 'n')
    return labels,canonical


def prepare(report_path,out):
    out=Path(out);report_path=Path(report_path)
    if out.exists():raise ValueError('use a fresh output directory')
    report=alignment.read(report_path)
    if report['language']!='zh':raise ValueError('Chinese observations required')
    requests=[];selected=[];rejected=[]
    for unit in report['units']:
        pair=phones_for_unit(unit)
        if pair is None:
            if unit.get('assigned_coda_phones'):rejected.append(dict(unit_index=unit['unit_index'],reason='unsupported-compound-or-alias'))
            continue
        unit=observed_source(unit, report_path);selected.append(unit)
        requests.append(dict(unit_index=unit['unit_index'],phones=pair[0],canonical_phones=pair[1]))
    if not selected:raise ValueError('no supported nasal syllables')
    out.mkdir(parents=True)
    (out/'mfa-config.yaml').write_text('tokenization: simple\n',encoding='utf-8')
    report['units']=selected
    alignment.write(out/'observations.json',report);alignment.write(out/'requests.json',dict(units=requests,rejected=rejected))
    alignment.prepare(out/'observations.json',out/'requests.json',out/'mfa')
    return len(selected)


def finish(work,alignments,out):
    work,out=Path(work),Path(out)
    if out.exists():raise ValueError('use a fresh output directory')
    out.mkdir(parents=True)
    report=alignment.import_alignments(work/'mfa/manifest.json',alignments,'mandarin_mfa',out/'aligned-observations.json')
    selected=[];audit=[]
    for unit in report['units']:
        check=alignment.acoustic_audit(unit,'zh')
        reason=None
        if not unit.get('forced_phone_intervals'):reason='not-aligned'
        elif any('outside-alignment' in w for w in check['warnings']):reason='uncovered-audible-source'
        else:
            for phone in check['phones']:
                if phone['symbol'] in ('n','ng') and (phone['end_ms']-phone['start_ms']<30-.001 or phone['periodic_ms']<20-.001):reason='nasal-without-periodic-support'
        audit.append(dict(unit_index=unit['unit_index'],alias=unit['alias'],accepted=reason is None,reason=reason,acoustic_audit=check))
        if reason is None:
            unit['phone_alignment']['transcript_source']='alias-derived-simple-mandarin-nasal-syllable'
            selected.append(unit)
    report['units']=selected
    alignment.write(out/'selected-observations.json',report)
    alignment.write(out/'audit.json',dict(kind='acoustic-consistency-not-boundary-accuracy',units=audit,verified_units=0,training_eligible_units=0))
    if selected:
        auto.spans.propose(out/'selected-observations.json',out/'requests.json')
        auto.spans.select(out/'selected-observations.json',out/'requests.json',out/'selected')
        auto.build_library([out/'selected/spans.json'],out/'library.json')
    return len(selected)


def main():
    p=argparse.ArgumentParser(description=__doc__);commands=p.add_subparsers(dest='command',required=True)
    c=commands.add_parser('prepare');c.add_argument('--report',required=True);c.add_argument('--out',required=True)
    c=commands.add_parser('finish');c.add_argument('--work',required=True);c.add_argument('--alignments',required=True);c.add_argument('--out',required=True)
    a=p.parse_args()
    count=prepare(a.report,a.out) if a.command=='prepare' else finish(a.work,a.alignments,a.out)
    print(f'{a.command}: {count} supported Chinese source units')


if __name__=='__main__':main()
