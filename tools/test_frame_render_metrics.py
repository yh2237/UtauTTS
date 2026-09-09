import unittest
import numpy as np
from frame_render_metrics import render_contour


class RenderMetricsTest(unittest.TestCase):
    def test_pause_and_bounds(self):
        result = render_contour([-900, -300, 0, 300, 900], [True, True, False, True, True])
        self.assertEqual(result[2], 0)
        self.assertLessEqual(max(abs(result)), 90)

    def test_constant_and_translation(self):
        np.testing.assert_allclose(render_contour([100] * 10, [True] * 10), 0)
        values = np.arange(30.) ** 2
        np.testing.assert_allclose(render_contour(values, [True] * 30),
                                   render_contour(values + 500, [True] * 30))


if __name__ == "__main__":
    unittest.main()
