"""周辺音の除外、繰り返し音素の曖昧さ、区間対応の同一性を検証する。"""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
import wave

spec = importlib.util.spec_from_file_location('source_spans', Path(__file__).with_name('source-span-mapping.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class SpanTests(unittest.TestCase):
    def fixture(self):
        phones = [dict(symbol='eh',start_ms=0,end_ms=210), dict(symbol='k',start_ms=210,end_ms=340),
                  dict(symbol='eh',start_ms=340,end_ms=530)]
        unit = dict(unit_index=16, position=7, alias='e k', role='ending', source_clip='source.wav',
                    assigned_coda_phones=['k'], requested_context=[
                        dict(symbol='eh',role='nucleus',start_ms=100,duration_ms=140),
                        dict(symbol='k',role='coda',start_ms=240,duration_ms=70)],
                    forced_phone_intervals=phones,
                    phone_alignment=dict(kind='forced',source_sha256='pcm',alignment_sha256='alignment',acoustic_model='test'),
                    analysis=dict(source_sha256='pcm',duration_ms=550,window_ms=20,
                                  periodicity_method='normalized-autocorrelation-local-peak-80-500hz-v2',
                                  low_energy_threshold_dbfs=-60,
                                  frames=[dict(start_ms=i*10,end_ms=(i+1)*10,rms_dbfs=-20,periodicity=.9,zero_crossing_rate=.03) for i in range(55)],
                                  landmarks=[dict(kind='energy-rise',source_ms=300,duration_ms=20,relative_to_peak_db=-5,heuristic_score=.8)]))
        request = dict(unit_index=16,source_sha256='pcm',alignment_sha256='alignment',
                       mappings=[dict(source_phone_index=1,requested_phone_index=1)],left_context_count=1)
        return unit, request

    def test_target_coda_excludes_following_vowel(self):
        unit, request = self.fixture()
        row = m.select_unit(unit, request, 'en')
        self.assertEqual((row['core_start_ms'], row['core_end_ms']), (210, 340))
        self.assertEqual((row['context_start_ms'], row['context_end_ms']), (0, 340))
        self.assertEqual(row['adjacent_context_phone_indices'], [0])
        self.assertEqual(row['excluded_phone_indices'], [2])
        self.assertAlmostEqual(row['mappings'][0]['duration_ratio'], 70/130)
        self.assertFalse(row['training_eligible'])
        self.assertEqual(row['selection_status'], 'unverified')

    def test_mismatched_stale_or_duplicate_mapping_rejected(self):
        unit, request = self.fixture()
        for bad in [dict(source_sha256='stale'), dict(alignment_sha256='stale'),
                    dict(mappings=[dict(source_phone_index=0,requested_phone_index=1)]),
                    dict(mappings=request['mappings']*2), dict(left_context_count=2),
                    dict(right_context_count=-1)]:
            value = copy.deepcopy(request); value.update(bad)
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                m.select_unit(unit, value, 'en')
        unit['assigned_coda_phones'] = ['k', 's']
        with self.assertRaises(ValueError):
            m.select_unit(unit, request, 'en')

    def test_ambiguous_repeated_source_coda_not_proposed(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            unit, _ = self.fixture()
            unit['forced_phone_intervals'].append(dict(symbol='k',start_ms=530,end_ms=550))
            m.alignment.write(root/'report.json', dict(language='en',units=[unit]))
            result = m.propose(root/'report.json', root/'requests.json')
            self.assertEqual(result['units'], [])
            self.assertIn('ambiguous', result['rejected'][0]['reason'])

    def test_ambiguous_requested_coda_not_proposed(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            unit, _ = self.fixture()
            unit['requested_context'].append(dict(symbol='k',role='coda',start_ms=310,duration_ms=70))
            m.alignment.write(root/'report.json', dict(language='en',units=[unit]))
            result = m.propose(root/'report.json', root/'requests.json')
            self.assertEqual(result['units'], [])
            self.assertIn('requested coda', result['rejected'][0]['reason'])

    def test_transient_crossing_core_end_is_not_silently_trimmed(self):
        unit, request = self.fixture()
        unit['analysis']['landmarks'][0].update(source_ms=335,duration_ms=20)
        row = m.select_unit(unit, request, 'en')
        self.assertIn('landmark-extends-past-selected-core', row['warnings'])
        self.assertFalse(row['landmark_candidates'][0]['fully_inside_core'])
        self.assertEqual(row['core_end_ms'], 340)

    def test_pcm_crops_and_report_changes_are_bound_to_selection(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            with wave.open(str(root/'source.wav'), 'wb') as audio:
                audio.setnchannels(2); audio.setsampwidth(2); audio.setframerate(16000)
                audio.writeframes(b'\x01\x00\x02\x00'*8800)
            unit, _ = self.fixture()
            digest, _ = m.alignment.clip_identity(root/'source.wav')
            unit['analysis']['source_sha256'] = unit['phone_alignment']['source_sha256'] = digest
            m.alignment.write(root/'report.json', dict(language='en',units=[unit]))
            proposal = m.propose(root/'report.json', root/'requests.json')
            self.assertEqual(len(proposal['units']), 1)
            result = m.select(root/'report.json', root/'requests.json', root/'clips')
            row = result['units'][0]
            self.assertEqual(row['core_clip']['source_start_sample'], 3360)
            self.assertEqual(row['core_clip']['source_end_sample'], 5440)
            self.assertEqual(row['core_clip']['duration_ms'], 130)
            self.assertEqual(row['context_clip']['duration_ms'], 340)
            with wave.open(str(root/'clips'/row['core_clip']['path']), 'rb') as audio:
                self.assertEqual(audio.getnchannels(), 2)
                self.assertEqual(audio.readframes(1), b'\x01\x00\x02\x00')
            with wave.open(str(root/'source.wav'), 'wb') as audio:
                audio.setnchannels(2); audio.setsampwidth(2); audio.setframerate(16000)
                audio.writeframes(b'\x03\x00\x04\x00'*8800)
            with self.assertRaises(ValueError):
                m.select(root/'report.json', root/'requests.json', root/'changed-pcm')
            unit['requested_context'][1]['duration_ms'] = 80
            m.alignment.write(root/'report.json', dict(language='en',units=[unit]))
            with self.assertRaises(ValueError):
                m.select(root/'report.json', root/'requests.json', root/'changed')

    def test_assigned_coda_coverage_and_nonmonotone_target_rejected(self):
        unit, request = self.fixture()
        unit['assigned_coda_phones'] = []
        request['mappings'] = [dict(source_phone_index=0,requested_phone_index=0),dict(source_phone_index=1,requested_phone_index=1)]
        unit['requested_context'][1]['start_ms'] = 200
        with self.assertRaises(ValueError):
            m.select_unit(unit, request, 'en')
        unit, request = self.fixture()
        unit['forced_phone_intervals'][1]['end_ms'] = 210
        with self.assertRaises(ValueError):
            m.select_unit(unit, request, 'en')


if __name__ == '__main__':
    unittest.main()
