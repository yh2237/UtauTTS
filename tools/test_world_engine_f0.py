"""Integration checks for the native Harvest adapter (build WORLD first)."""
import os
import sys
import unittest
from pathlib import Path

import numpy as np
from world_engine_f0 import WorldEngineF0


class WorldEngineF0Test(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        suffix = '.dll' if sys.platform == 'win32' else '.dylib' if sys.platform == 'darwin' else '.so'
        path = Path(os.environ.get('UTAUTTS_WORLD_ENGINE', str(Path(__file__).resolve().parents[1] / 'runtime' / ('utautts-world-engine' + suffix))))
        if not path.exists():
            raise unittest.SkipTest('build utautts-world-engine before running integration tests')
        cls.engine = WorldEngineF0(path)

    def test_silence(self):
        track = self.engine.extract(np.zeros(16000), 16000)
        self.assertEqual(len(track), 101)
        self.assertTrue(np.all(track == 0))

    def test_periodic_voice(self):
        time = np.arange(32000) / 16000
        samples = sum(np.sin(2 * np.pi * 200 * harmonic * time) / harmonic for harmonic in range(1, 8))
        track = self.engine.extract(samples, 16000)
        voiced = track[track > 0]
        self.assertGreater(len(voiced), 100)
        self.assertLess(abs(float(np.median(voiced)) - 200), 5)

    def test_invalid_input(self):
        for samples, rate, frame in [(np.zeros((2, 2)), 16000, 10), ([np.nan, 0], 16000, 10), ([0, 0], 0, 10), ([0, 0], 16000, 0)]:
            with self.assertRaises(ValueError):
                self.engine.extract(samples, rate, frame)
        with self.assertRaises(ValueError):
            WorldEngineF0(self.engine.path, method=0)


if __name__ == '__main__':
    unittest.main()
