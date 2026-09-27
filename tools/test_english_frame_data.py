import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

def load(name):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).parent/(name+'.py'))
    module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
    return module

trainer=load('train-frame-intonation-tcn')

class EnglishFrameTests(unittest.TestCase):
    def test_features_and_pause_boundaries(self):
        tokens=[dict(language='en',phones=[dict(symbol='eh',role='nucleus')],word_index=0,word_end=True,stress=1,stress_known=True),dict(language='en',pause=True)]
        features=trainer.token_features(tokens,0)
        self.assertEqual(features['syllable_nucleus=eh'],1)
        self.assertEqual(features['syllable_stress=1'],1)
        self.assertEqual(features['next=<PAUSE>'],1)
        self.assertEqual(features['phrase_end'],1)
        self.assertFalse(any(k.startswith('accent_') for k in features))
        self.assertNotIn('en_word_start',trainer.token_features(tokens,1))

    def test_speaker_leakage_rejected(self):
        rows=[dict(version=1,id=str(i),language='en',speaker='same',split=split,text=str(i),audio_path='fixture.wav',tokens=[{'language':'en'}]) for i,split in enumerate(['train','validation'])]
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'data.jsonl'
            path.write_text('\n'.join(map(json.dumps,rows)),encoding='utf-8')
            with self.assertRaisesRegex(ValueError,'leakage'): trainer.load_records(path)

    def test_english_does_not_get_japanese_accent(self):
        row=dict(language='en',tokens=[dict(language='en',phones=[],word_index=0)])
        self.assertEqual(trainer.add_openjtalk_features([row],False)[0]['tokens'],row['tokens'])

if __name__=='__main__': unittest.main()
