import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location('zh_discovery',Path(__file__).with_name('source-phone-discovery-zh.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class ChineseDiscoveryTests(unittest.TestCase):
    def unit(self):return dict(role='mora',alias='hen',assigned_coda_phones=['n'],requested_context=[dict(symbol='h',role='onset'),dict(symbol='e',role='nucleus'),dict(symbol='n',role='coda')])
    def test_alias_and_mfa_phone_sets_are_kept_separate(self):
        acoustic,canonical=m.phones_for_unit(self.unit())
        self.assertEqual(acoustic,['x','ə','n']);self.assertEqual(canonical,['h','e','n'])
    def test_compound_or_wrong_alias_not_guessed(self):
        unit=self.unit();unit['alias']='other';self.assertIsNone(m.phones_for_unit(unit))
        unit=self.unit();unit['requested_context'].insert(1,dict(symbol='i',role='medial'));self.assertIsNone(m.phones_for_unit(unit))
    def test_velar_nasal_mapping(self):
        unit=self.unit();unit['alias']='hang';unit['assigned_coda_phones']=['ng'];unit['requested_context'][1]['symbol']='a';unit['requested_context'][2]['symbol']='ng'
        self.assertEqual(m.phones_for_unit(unit),(['x','a','ŋ'],['h','a','ng']))
    def test_tone_suffix_is_not_a_phone(self):
        unit=self.unit();unit['alias']='henB3'
        self.assertEqual(m.phones_for_unit(unit),(['x','ə','n'],['h','e','n']))

if __name__=='__main__':unittest.main()
