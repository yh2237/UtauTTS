import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("prepare_full", Path(__file__).with_name("prepare-jsut-full-labels.py"))
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class LabelsTest(unittest.TestCase):
    def test_actual_phone_and_accent(self):
        data = "0 1000000 x-sil+k=x/A:xx+xx+xx/F:xx_xx\n1000000 1500000 x-k+a=x/A:0+1+1/F:1_1\n1500000 2500000 x-a+sil=x/A:0+1+1/F:1_1\n2500000 3000000 x-sil+x=x/A:xx+xx+xx/F:xx_xx"
        result = prepare.convert(data, {("k", "a"): "か"})
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["duration_ms"], 150)
        self.assertTrue(result[0]["accent_high"])
        self.assertEqual(result[0]["accent_nucleus"], 1)
        with self.assertRaises(ValueError):
            prepare.convert(data, {})
        with self.assertRaises(ValueError):
            prepare.convert(data.replace("1000000 1500000", "900000 1500000"), {("k", "a"): "か"})


if __name__ == "__main__":
    unittest.main()
