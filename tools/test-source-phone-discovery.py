"""エイリアス仮説と母音の音響的支持を検証する。"""
import copy
import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location('source_discovery',Path(__file__).with_name('source-phone-discovery.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)


class DiscoveryTests(unittest.TestCase):
    def unit(self):
        return dict(role='ending',alias='6 kB3',assigned_coda_phones=['k'],
                    requested_context=[dict(symbol='uh',role='nucleus'),dict(symbol='k',role='coda')])

    def test_vccv_includes_following_vowel_hypothesis(self):
        rows=m.alias_hypotheses(self.unit(),{'uh':['6'],'k':['k']},'en-vccv')
        self.assertEqual([r['phones'] for r in rows],[['uh','k'],['uh','k','uh']])
        self.assertTrue(rows[1]['preferred'])

    def test_terminal_and_delta_prefer_vc(self):
        unit=self.unit();unit['alias']='6k-B3'
        self.assertTrue(m.alias_hypotheses(unit,{'uh':['6'],'k':['k']},'en-vccv')[0]['preferred'])
        unit['alias']='U k'
        rows=m.alias_hypotheses(unit,{'uh':['U'],'k':['k']},'en-delta')
        self.assertEqual(len(rows),1)
        self.assertEqual(rows[0]['phones'],['uh','k'])

    def test_unknown_or_conflicting_alias_not_guessed(self):
        for alias in ('6 q','6 kExtra','other'):
            unit=self.unit();unit['alias']=alias
            self.assertEqual(m.alias_hypotheses(unit,{'uh':['6'],'k':['k']},'en-vccv'),[])

    def acoustic_unit(self):
        return dict(forced_phone_intervals=[dict(symbol='uh',start_ms=0,end_ms=200),dict(symbol='k',start_ms=200,end_ms=340),dict(symbol='uh',start_ms=340,end_ms=500)],
                    analysis=dict(duration_ms=550,window_ms=20,periodicity_method='normalized-autocorrelation-local-peak-80-500hz-v2',low_energy_threshold_dbfs=-60,
                                  frames=[dict(start_ms=i*10,end_ms=(i+1)*10,rms_dbfs=-20,periodicity=.9,zero_crossing_rate=.03) for i in range(55)],landmarks=[]))

    def test_silent_added_vowel_rejected(self):
        unit=self.acoustic_unit()
        for frame in unit['analysis']['frames']:
            if frame['start_ms']>=340:frame['periodicity']=0;frame['rms_dbfs']=-80
        score=m.score_candidate(unit,dict(phones=['uh','k','uh'],preferred=True))
        self.assertFalse(score['accepted'])
        self.assertEqual(score['reason'],'vowel-hypothesis-lacks-periodic-support')

    def test_periodic_added_vowel_supported_but_unverified(self):
        result=m.score_candidate(self.acoustic_unit(),dict(phones=['uh','k','uh'],preferred=True))
        self.assertTrue(result['accepted'])
        self.assertFalse(result['audit']['training_eligible'])
        self.assertGreater(result['vowels'][-1]['periodic_ms'],25)

    def test_mismatched_transcript_rejected(self):
        self.assertFalse(m.score_candidate(self.acoustic_unit(),dict(phones=['uh','d','uh'],preferred=True))['accepted'])

    def test_uncovered_audible_tail_not_silently_cut(self):
        unit=self.acoustic_unit()
        unit['forced_phone_intervals']=unit['forced_phone_intervals'][:2]
        result=m.score_candidate(unit,dict(phones=['uh','k'],preferred=True))
        self.assertFalse(result['accepted'])
        self.assertEqual(result['reason'],'alignment-leaves-audible-source-activity')

    def test_alias_preference_cannot_decide_identical_acoustic_evidence(self):
        unit=self.acoustic_unit()
        a=m.score_candidate(unit,dict(phones=['uh','k','uh'],preferred=True))
        b=m.score_candidate(unit,dict(phones=['uh','k','uh'],preferred=False))
        self.assertLess(abs(a['penalty']-b['penalty']),.1)


if __name__=='__main__':unittest.main()
