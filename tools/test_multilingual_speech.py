"""数式検証用データ。自然音声の学習結果ではない。"""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
import wave
import numpy as np


def module(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).parent / (name+'.py'))
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


trainer = module('train-multilingual-speech')
prepare = module('prepare-multilingual-speech')
boundary = module('audit-speech-boundaries')


def fixture():
    result = []
    for i, split in enumerate(('train', 'validation', 'test')):
        phones = [{'symbol': 'aa', 'features': ['bias', 'phone=aa'], 'baseline_ms': 100.,
                   'duration_ms': 150., 'pitch_cents': [10., 20., 30.], 'energy_log_ratio': .1} for _ in range(6)]
        result.append({'version': 1, 'feature_version': 1, 'id': f'fixture-{i}', 'language': 'en',
                       'speaker': str(i), 'split': split, 'kind': 'natural', 'alignment': 'manual',
                       'corpus': 'unit-test-fixture', 'license': 'test-only', 'audio_sha256': str(i),
                       'text': f'test-{i}', 'phones': phones})
    return result


class SpeechTests(unittest.TestCase):
    def test_fit_and_held_out_exclusion(self):
        records = fixture()
        model = trainer.train(records, 'en', 'test-only')
        score = model['evaluation']['validation']
        self.assertLess(score['model_duration_mae_ms'], score['baseline_duration_mae_ms'])
        changed = copy.deepcopy(records)
        for p in changed[1]['phones']: p['duration_ms'] = 400
        other = trainer.train(changed, 'en', 'test-only')
        self.assertEqual(model['duration_log_ratio'], other['duration_log_ratio'])
        self.assertEqual(model['pitch_phone_counts']['aa'], 6)

    def test_leakage_and_generated_labels_rejected(self):
        for key in ('speaker', 'text', 'audio_sha256'):
            records = fixture()
            records[1][key] = records[0][key]
            with self.assertRaises(ValueError): trainer.train(records, 'en', 'test-only')
        records = fixture(); records[0]['kind'] = 'generated'
        with self.assertRaises(ValueError): trainer.train(records, 'en', 'test-only')

    def test_pitch_and_alignment_validation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            rate = 16000
            samples = (.2*np.sin(2*np.pi*220*np.arange(rate)/rate)*32767).astype('<i2')
            path = root/'test.wav'
            with wave.open(str(path), 'wb') as wav:
                wav.setnchannels(1); wav.setsampwidth(2); wav.setframerate(rate); wav.writeframes(samples.tobytes())
            template = {'version':1, 'feature_version':1, 'id':'fixture', 'language':'en', 'text':'fixture',
                        'phones':[{'position':0, 'phone_index':0, 'symbol':'aa', 'role':'nucleus', 'baseline_ms':120, 'features':['bias']}]}
            observation = {'id':'fixture', 'audio_path':'test.wav', 'speaker':'fixture', 'split':'train', 'kind':'natural',
                           'corpus':'test-only', 'license':'test-only', 'alignment':'manual',
                           'phones':[{'position':0, 'phone_index':0, 'symbol':'aa', 'start_ms':100, 'end_ms':900}]}
            row = prepare.prepare(template, observation, root)['phones'][0]
            self.assertEqual(row['duration_ms'], 800)
            self.assertTrue(all(abs(v) < 10 for v in row['pitch_cents']))
            observation['phones'][0]['symbol'] = 'different'
            with self.assertRaises(ValueError): prepare.prepare(template, observation, root)

    def test_unobserved_boundaries_are_not_zero_error(self):
        value = boundary.audit({'version':1, 'time_origin':'oto.offset', 'boundaries':[
            {'unit_index':0, 'anchor_index':1, 'observed_ms':None}]})
        self.assertEqual(value['labelled'], 0)
        self.assertEqual(value['status'], 'awaiting-manual-boundaries')


if __name__ == '__main__': unittest.main()
