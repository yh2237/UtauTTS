"""F0 extraction through UtauTTS's native WORLD engine (Harvest)."""
import ctypes
from pathlib import Path
import numpy as np


class WorldEngineF0:
    def __init__(self, path, method=1):
        if method != 1:
            raise ValueError("UtauTTS WORLD F0 supports Harvest (method 1) only")
        self.path = Path(path).resolve()
        self.library = ctypes.CDLL(str(self.path))
        self.function = self.library.UtauTTSWorldF0
        self.function.argtypes = [ctypes.POINTER(ctypes.c_double), ctypes.c_int, ctypes.c_int,
                                  ctypes.c_double, ctypes.POINTER(ctypes.c_double), ctypes.c_int,
                                  ctypes.c_char_p, ctypes.c_int]
        self.function.restype = ctypes.c_int

    def extract(self, samples, sample_rate, frame_ms=10):
        values = np.ascontiguousarray(samples, dtype=np.float64)
        if values.ndim != 1 or sample_rate <= 0 or not np.isfinite(frame_ms) or frame_ms <= 0 or not np.isfinite(values).all():
            raise ValueError("invalid F0 input")
        if len(values) < 2:
            return np.zeros(0)
        output = np.empty(int(1000 * len(values) / sample_rate / frame_ms) + 2)
        error = ctypes.create_string_buffer(1024)
        count = self.function(values.ctypes.data_as(ctypes.POINTER(ctypes.c_double)),len(values),sample_rate,frame_ms,
                              output.ctypes.data_as(ctypes.POINTER(ctypes.c_double)),len(output),error,len(error))
        if count <= 0:
            raise RuntimeError(error.value.decode("utf-8", errors="replace"))
        return output[:count].copy()
