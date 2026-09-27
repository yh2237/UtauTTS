"""未知の英語VC原音に音素列の仮説を作り、整列結果から区間候補を選ぶ。"""
import argparse
import math
from pathlib import Path
import re

from source_phone_common import load_tool, observed_source

auto = load_tool('source-span-auto.py')
alignment = auto.alignment
VOWELS = set('aa ae ah ao ax aw ay eh er ey ih iy ow oy uh uw'.split())


def alias_hypotheses(unit, symbols, phonemizer):
    if unit['role'] != 'ending' or not unit.get('assigned_coda_phones'):
        return []
    nuclei = {p['symbol'] for p in unit['requested_context'] if p['role'] == 'nucleus'}
    if len(nuclei) != 1:
        return []
    vowel = next(iter(nuclei))
    coda = unit['assigned_coda_phones']
    if vowel not in VOWELS or any(p not in symbols for p in [vowel]+coda):
        return []
    # 音階接尾辞だけを除き、記号の大文字小文字を保つ。
    alias = re.sub(r'[A-G][#b]?-?\d+$', '', unit['alias']).strip()
    terminal = alias.endswith('-')
    compact = re.sub(r'\s+', '', alias.strip('- '))
    possible = ['']
    for phone in [vowel]+coda:
        possible = [prefix+s for prefix in possible for s in symbols[phone]]
        if len(possible)>4096:
            return []
    if compact not in possible:
        return []
    primary = 'vcv' if phonemizer == 'en-vccv' and not terminal else 'vc'
    result = [dict(kind='vc', phones=[vowel]+coda, preferred=primary=='vc')]
    # VCCVのVC原音には後続母音が入る場合がある。
    if phonemizer == 'en-vccv':
        result.append(dict(kind='vcv', phones=[vowel]+coda+[vowel], preferred=primary=='vcv'))
    return result


def acoustic_label(phone):
    return ('AH' if phone=='ax' else phone.upper()) + ('0' if phone in VOWELS else '')


def prepare(reports, libraries, out):
    out=Path(out)
    if out.exists():
        raise ValueError('use a fresh output directory')
    known=set()
    for path in libraries:
        library=alignment.read(path)
        if library['language']!='en':raise ValueError('English source library required')
        known.update(row['source_sha256'] for row in library['entries'])
    units,requests,provenance,skipped,seen=[],[],[],[],set()
    for path in reports:
        report=alignment.read(path)
        if report['language']!='en' or report.get('phonemizer') not in ('en-vccv','en-delta'):
            raise ValueError('English Delta/VCCV source observations required')
        for unit in report['units']:
            if not unit.get('assigned_coda_phones'):continue
            digest=unit['analysis']['source_sha256']
            if digest in known or digest in seen:continue
            hypotheses=alias_hypotheses(unit,report['source_phone_symbols'],report['phonemizer'])
            if not hypotheses:
                skipped.append(dict(report=str(Path(path).resolve()),unit_index=unit['unit_index'],alias=unit['alias'],reason='unsupported-or-ambiguous-alias'))
                continue
            seen.add(digest)
            for hypothesis in hypotheses:
                index=len(units)
                row=observed_source(unit, path);row['unit_index']=index
                units.append(row)
                requests.append(dict(unit_index=index,phones=[acoustic_label(p) for p in hypothesis['phones']]))
                provenance.append(dict(candidate_index=index,source_sha256=digest,source_report=str(Path(path).resolve()),
                                       source_report_sha256=auto.spans.file_hash(path),source_unit_index=unit['unit_index'],alias=unit['alias'],**hypothesis))
    if not units:raise ValueError('no supported unknown sources')
    out.mkdir(parents=True)
    alignment.write(out/'observations.json',dict(language='en',units=units,annotation_status='unobserved'))
    alignment.write(out/'requests.json',dict(units=requests))
    alignment.write(out/'hypotheses.json',dict(version=1,kind='alias-derived-unverified-source-phone-hypotheses',candidates=provenance,skipped=skipped))
    alignment.prepare(out/'observations.json',out/'requests.json',out/'mfa')
    return dict(sources=len(seen),candidates=len(units),skipped=skipped)


def score_candidate(unit, hypothesis):
    phones=unit.get('forced_phone_intervals',[])
    if not phones or [p['symbol'] for p in phones] != [('ah' if p=='ax' else p) for p in hypothesis['phones']]:
        return dict(accepted=False,reason='missing-or-mismatched-alignment')
    audit=alignment.acoustic_audit(unit,'en')
    frames=unit['analysis']['frames']
    vowel_scores=[]
    for phone in phones:
        if phone['symbol'] not in VOWELS:continue
        duration=phone['end_ms']-phone['start_ms']
        active=periodic=0.0
        for frame in frames:
            overlap=max(0,min(frame['end_ms'],phone['end_ms'])-max(frame['start_ms'],phone['start_ms']))
            if frame['rms_dbfs']>=unit['analysis']['low_energy_threshold_dbfs']:
                active+=overlap
                if frame['periodicity']>=.65:periodic+=overlap
        fraction=periodic/duration
        vowel_scores.append(dict(symbol=phone['symbol'],start_ms=phone['start_ms'],end_ms=phone['end_ms'],periodic_ms=periodic,periodic_fraction=fraction,active_fraction=active/duration))
        if duration<40 or periodic<25 or fraction<.25:
            return dict(accepted=False,reason='vowel-hypothesis-lacks-periodic-support',vowels=vowel_scores,audit=audit)
    # 未被覆の音響活動を優先して減らし、同点近傍ではエイリアスの仮説を使う。
    warnings=audit['warnings']
    if any('outside-alignment' in w for w in warnings):
        return dict(accepted=False,reason='alignment-leaves-audible-source-activity',vowels=vowel_scores,audit=audit)
    penalty=sum(1 for w in warnings if 'outside-alignment' in w)+sum(1-v['periodic_fraction'] for v in vowel_scores)/len(vowel_scores)
    penalty+=0 if hypothesis['preferred'] else .02
    return dict(accepted=True,penalty=penalty,vowels=vowel_scores,audit=audit)


def finish(work, alignments, model, out):
    work,out=Path(work),Path(out)
    if out.exists():raise ValueError('use a fresh output directory')
    out.mkdir(parents=True)
    report=alignment.import_alignments(work/'mfa/manifest.json',alignments,model,out/'aligned-observations.json')
    hypotheses=alignment.read(work/'hypotheses.json')
    units={u['unit_index']:u for u in report['units']}
    groups={};scores=[]
    for hypothesis in hypotheses['candidates']:
        index=hypothesis['candidate_index'];unit=units[index]
        scored=score_candidate(unit,hypothesis)
        record=dict(hypothesis=hypothesis,**scored);scores.append(record)
        if scored['accepted']:groups.setdefault(hypothesis['source_sha256'],[]).append((scored['penalty'],index))
    chosen,rejected=[],[]
    for digest in sorted({h['source_sha256'] for h in hypotheses['candidates']}):
        ranked=sorted(groups.get(digest,[]))
        if not ranked:
            rejected.append(dict(source_sha256=digest,reason='no-acoustically-supported-hypothesis'));continue
        if len(ranked)>1 and ranked[1][0]-ranked[0][0]<.1:
            rejected.append(dict(source_sha256=digest,reason='ambiguous-hypothesis-score'));continue
        unit=units[ranked[0][1]]
        unit['phone_alignment']['transcript_source']='alias-derived-acoustically-screened-hypothesis'
        unit['phone_alignment']['hypothesis_candidate_index']=ranked[0][1]
        chosen.append(unit)
    selection_report=copy.deepcopy(report);selection_report['units']=chosen
    alignment.write(out/'chosen-observations.json',selection_report)
    proposals=auto.spans.propose(out/'chosen-observations.json',out/'requests.json')
    result=dict(version=1,kind='heuristic-acoustic-consistency-not-boundary-accuracy',model=model,
                candidates=scores,chosen_candidates=[u['unit_index'] for u in chosen],rejected=rejected,
                verified_units=0,training_eligible_units=0)
    alignment.write(out/'discovery-audit.json',result)
    if proposals['units']:
        auto.spans.select(out/'chosen-observations.json',out/'requests.json',out/'selected')
        auto.build_library([out/'selected/spans.json'],out/'library.json')
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    commands=parser.add_subparsers(dest='command',required=True)
    p=commands.add_parser('prepare');p.add_argument('--report',action='append',required=True);p.add_argument('--library',action='append',default=[]);p.add_argument('--out',required=True)
    p=commands.add_parser('finish');p.add_argument('--work',required=True);p.add_argument('--alignments',required=True);p.add_argument('--model',default='english_us_arpa');p.add_argument('--out',required=True)
    args=parser.parse_args()
    if args.command=='prepare':
        result=prepare(args.report,args.library,args.out);print(f"prepared {result['sources']} sources / {result['candidates']} hypotheses")
    else:
        result=finish(args.work,args.alignments,args.model,args.out);print(f"selected {len(result['chosen_candidates'])} sources; rejected {len(result['rejected'])}")


if __name__=='__main__':main()
