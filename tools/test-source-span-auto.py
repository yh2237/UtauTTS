"""別文への対応付けと未知・変更済み原音の扱いを検証する。"""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
import wave

spec = importlib.util.spec_from_file_location('source_auto', Path(__file__).with_name('source-span-auto.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class AutoTests(unittest.TestCase):
    def fixture(self, root):
        source = root/'source.wav'
        with wave.open(str(source), 'wb') as f:
            f.setnchannels(1); f.setsampwidth(2); f.setframerate(10000); f.writeframes(b'\0\0'*5500)
        digest, duration = m.alignment.clip_identity(source)
        phones = [dict(symbol='eh', start_ms=0, end_ms=210), dict(symbol='k', start_ms=210, end_ms=340), dict(symbol='eh', start_ms=340, end_ms=530)]
        span = dict(alias='e k', original_clip=str(source), source_sha256=digest,
                    source_clip_phone_intervals=phones, acoustic_model='test', alignment_sha256='alignment')
        span_path, library_path, report_path = root/'spans.json', root/'library.json', root/'observations.json'
        m.alignment.write(span_path, dict(language='en', time_origin='oto-offset', units=[span]))
        m.build_library([span_path], library_path)
        unit = dict(unit_index=29, position=12, alias='e k', role='ending', source_clip='source.wav',
                    assigned_coda_phones=['k'], requested_context=[dict(symbol='eh', role='nucleus', start_ms=700, duration_ms=140),
                                                                dict(symbol='k', role='coda', start_ms=840, duration_ms=70)],
                    analysis=dict(source_sha256=digest, duration_ms=duration, window_ms=20,
                                  periodicity_method='normalized-autocorrelation-local-peak-80-500hz-v2', low_energy_threshold_dbfs=-60,
                                  frames=[dict(start_ms=i*10, end_ms=(i+1)*10, rms_dbfs=-70, periodicity=0, zero_crossing_rate=0) for i in range(55)], landmarks=[]))
        m.alignment.write(report_path, dict(language='en', units=[unit]))
        return span_path, library_path, report_path, unit

    def test_new_indices_and_timings_use_same_source_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); _, library, report, _ = self.fixture(root)
            result = m.map_report(report, library, root/'mapped')
            self.assertEqual(result['selected_indices'], [29])
            row = m.alignment.read(root/'mapped/selected/spans.json')['units'][0]
            self.assertEqual(row['position'], 12)
            self.assertEqual(row['mappings'][0]['requested_start_ms'], 840)
            self.assertEqual((row['core_start_ms'], row['core_end_ms']), (210, 340))
            self.assertFalse(row['training_eligible'])

    def test_unknown_source_keeps_existing_timing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); _, library, report, unit = self.fixture(root)
            with wave.open(str(root/'other.wav'),'wb') as f:
                f.setnchannels(1);f.setsampwidth(2);f.setframerate(10000);f.writeframes(b'\1\0'*5500)
            unit['source_clip']='other.wav';unit['analysis']['source_sha256']=m.alignment.clip_identity(root/'other.wav')[0]
            m.alignment.write(report,dict(language='en',units=[unit]))
            result=m.map_report(report,library,root/'mapped')
            self.assertEqual((result['mapped_units'],result['unmapped_units']),(0,1))
            self.assertFalse((root/'mapped/selected/spans.json').exists())

    def test_changed_pcm_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); _,library,report,_=self.fixture(root)
            data=bytearray((root/'source.wav').read_bytes());data[-1]=1;(root/'source.wav').write_bytes(data)
            with self.assertRaisesRegex(ValueError,'PCM changed'):m.attach_library(report,library)

    def test_conflicting_hypotheses_require_explicit_selection(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); span,_,_,_=self.fixture(root)
            changed=m.alignment.read(span);changed['units'][0]['source_clip_phone_intervals'][1]['end_ms']=330
            other=root/'other.json';m.alignment.write(other,changed)
            with self.assertRaisesRegex(ValueError,'conflicting'):m.build_library([span,other],root/'bad.json')
            result=m.build_library([span,other],root/'chosen.json',True)
            self.assertEqual(len(result['replacements']),1)

    def test_ambiguous_source_sequence_not_applied(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); _,library,report,_=self.fixture(root)
            value=m.alignment.read(library);value['entries'][0]['phones'][0]['symbol']='k';m.alignment.write(library,value)
            result=m.map_report(report,library,root/'mapped')
            self.assertEqual(result['mapped_units'],0)
            self.assertIn('ambiguous',result['rejected'][0]['reason'])


if __name__ == '__main__':
    unittest.main()
