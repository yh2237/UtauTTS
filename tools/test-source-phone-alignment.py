"""音素列・原音同一性・未確認整列の取り扱いを検証する。"""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import wave

spec = importlib.util.spec_from_file_location('source_alignment', Path(__file__).with_name('source-phone-alignment.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class AlignmentTests(unittest.TestCase):
    def test_chinese_acoustic_labels_map_only_after_exact_match(self):
        row=dict(phones=['x','ə','n'],canonical_phones=['h','e','n'],duration_ms=300)
        result=m.intervals(dict(tiers={'phones':dict(entries=[[0,.04,'x'],[.04,.2,'ə'],[.2,.3,'n']])}),row)
        self.assertEqual([p['symbol'] for p in result],['h','e','n'])
        self.assertEqual(result[1]['acoustic_label'],'ə')
        with self.assertRaises(ValueError):m.intervals(dict(tiers={'phones':dict(entries=[[0,.04,'x'],[.04,.2,'a'],[.2,.3,'n']])}),row)
    def acoustic_unit(self, tail_active):
        frames = [dict(start_ms=i*10, end_ms=(i+1)*10, rms_dbfs=-25 if i < 18 or tail_active else -80,
                       periodicity=.9, zero_crossing_rate=.03) for i in range(32)]
        return dict(analysis=dict(duration_ms=320, frames=frames, window_ms=20,
                                 periodicity_method='normalized-autocorrelation-local-peak-80-500hz-v2',
                                 low_energy_threshold_dbfs=-60, landmarks=[
                                     dict(kind='energy-rise', source_ms=110, heuristic_score=.8,
                                          relative_to_peak_db=-10)]),
                    forced_phone_intervals=[dict(symbol='l', start_ms=0, end_ms=80),
                                            dict(symbol='d', start_ms=80, end_ms=180)])

    def test_uncovered_active_tail_requires_review(self):
        unit = self.acoustic_unit(True)
        result = m.acoustic_audit(unit, 'en')
        self.assertEqual(result['status'], 'needs-review')
        self.assertIn('periodic-activity-outside-alignment', result['warnings'])
        self.assertEqual(result['uncovered_intervals'][0]['periodic_ms'], 100)
        self.assertFalse(result['training_eligible'])
        self.assertEqual(result['phones'][1]['strong_candidate_count'], 1)
        result = m.acoustic_audit(self.acoustic_unit(False), 'en')
        self.assertEqual(result['warnings'], [])
        self.assertFalse(result['training_eligible'])

    def test_weak_stop_candidate_and_short_boundary_residue(self):
        unit = self.acoustic_unit(True)
        unit['forced_phone_intervals'][-1]['end_ms'] = 300
        unit['analysis']['landmarks'][0]['relative_to_peak_db'] = -40
        result = m.acoustic_audit(unit, 'en')
        self.assertEqual(result['warnings'], ['stop-without-strong-landmark-candidate'])
        self.assertEqual(result['uncovered_intervals'][0]['active_ms'], 0)
        self.assertEqual(result['uncovered_intervals'][0]['checked_start_ms'], result['uncovered_intervals'][0]['checked_end_ms'])
        self.assertNotIn('strong_candidate_count', m.acoustic_audit(unit, 'zh')['phones'][1])

    def test_audit_rejects_incomplete_features_and_stored_alignment(self):
        unit = self.acoustic_unit(False)
        unit['analysis']['frames'].pop(10)
        with self.assertRaises(ValueError):
            m.acoustic_audit(unit, 'en')
        unit = self.acoustic_unit(False)
        unit['analysis'].pop('periodicity_method')
        with self.assertRaises(ValueError):
            m.acoustic_audit(unit, 'en')
        unit = self.acoustic_unit(False)
        unit['forced_phone_intervals'][-1]['end_ms'] = 400
        with self.assertRaises(ValueError):
            m.acoustic_audit(unit, 'en')

    def test_rejects_unknown_mismatch_and_bad_coordinates(self):
        row = dict(phones=['L', 'D'], duration_ms=300)
        for entries in ([[0, .1, 'L'], [.1, .3, 'spn']], [[0, .1, 'L']],
                        [[0, .2, 'L'], [.1, .3, 'D']], [[0, .1, 'L'], [.1, .4, 'D']],
                        [[0, .1, 'L'], [.1, float('nan'), 'D']]):
            with self.subTest(entries=entries), self.assertRaises(ValueError):
                m.intervals(dict(tiers={'phones':dict(entries=entries)}), row)

    def test_clip_binding_review_and_rejected_alignment(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            clip = root / 'clip.wav'
            with wave.open(str(clip), 'wb') as audio:
                audio.setnchannels(1); audio.setsampwidth(2); audio.setframerate(16000)
                audio.writeframes(b'\x01\x00'*4800)
            digest, duration = m.clip_identity(clip)
            analysis = self.acoustic_unit(False)['analysis']
            analysis.update(source_sha256=digest, duration_ms=duration)
            analysis['frames'] = analysis['frames'][:30]
            unit = dict(unit_index=6, alias='l d-', source_clip='clip.wav',
                        analysis=analysis)
            report = dict(language='en', units=[unit], observed_phone_intervals=[], annotation_status='unobserved')
            m.write(root/'report.json', report)
            m.write(root/'requests.json', dict(units=[dict(unit_index=6, phones=['L','D'])]))
            prepared = root/'prepared'
            m.prepare(root/'report.json', root/'requests.json', prepared)
            alignment = dict(tiers={'phones':dict(entries=[[0,.1,'L'],[.1,.3,'D']])})
            m.write(root/'alignments/source0006.json', alignment)
            result = m.import_alignments(prepared/'manifest.json', root/'alignments', 'test model', root/'aligned.json')
            self.assertEqual(result['observed_phone_intervals'], [])
            self.assertEqual(result['annotation_status'], 'unobserved')
            self.assertEqual(result['units'][0]['phone_alignment']['status'], 'unverified')
            audited = m.audit(root/'aligned.json', root/'audit.json')
            self.assertEqual(audited['training_eligible_units'], 0)
            self.assertEqual(audited['aligned_units'], 1)
            reference = dict(unit_index=6, source_sha256=digest, annotation_kind='manual',
                             phones=[dict(symbol='l',start_ms=10,end_ms=120), dict(symbol='d',start_ms=120,end_ms=290)])
            m.write(root/'manual.json', dict(units=[reference]))
            metrics = m.evaluate(root/'aligned.json', root/'manual.json', root/'metrics.json')
            self.assertEqual(metrics['overall']['mae_ms'], 15)
            reference['annotation_kind'] = 'forced'
            m.write(root/'manual.json', dict(units=[reference]))
            with self.assertRaises(ValueError):
                m.evaluate(root/'aligned.json', root/'manual.json', root/'metrics.json')
            alignment['tiers']['phones']['entries'][1][2] = 'T'
            m.write(root/'alignments/source0006.json', alignment)
            result = m.import_alignments(prepared/'manifest.json', root/'alignments', 'test', root/'rejected.json')
            self.assertEqual(len(result['alignment_audit']['rejected']), 1)
            self.assertNotIn('forced_phone_intervals', result['units'][0])
            with wave.open(str(prepared/'corpus/source0006.wav'), 'wb') as audio:
                audio.setnchannels(1); audio.setsampwidth(2); audio.setframerate(16000)
                audio.writeframes(b'\x02\x00'*4800)
            with self.assertRaises(ValueError):
                m.import_alignments(prepared/'manifest.json', root/'alignments', 'test', root/'changed.json')
            with wave.open(str(clip), 'wb') as audio:
                audio.setnchannels(1); audio.setsampwidth(2); audio.setframerate(16000)
                audio.writeframes(b'\x02\x00'*4800)
            with self.assertRaises(ValueError):
                m.audit(root/'aligned.json', root/'changed-audit.json')


if __name__ == '__main__':
    unittest.main()
