"""原音ツール共通の入出力、PCM識別、音素区間の検証。"""
import hashlib
import json
import math
from pathlib import Path
import re
import wave

def read(path):
    return json.loads(Path(path).read_text(encoding='utf-8'))


def write(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')


def normalize(phone):
    return re.sub(r'[012]$', '', phone).lower()


def clip_identity(path):
    with wave.open(str(path), 'rb') as audio:
        if audio.getsampwidth() != 2 or audio.getcomptype() != 'NONE':
            raise ValueError('16-bit PCM source required')
        rate, channels = audio.getframerate(), audio.getnchannels()
        pcm = audio.readframes(audio.getnframes())
        digest = hashlib.sha256(f'v1/{rate}/{channels}/'.encode() + pcm).hexdigest()
        return digest, audio.getnframes() * 1000 / rate


def file_hash(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def sequence_matches(sequence, wanted):
    return [i for i in range(len(sequence)-len(wanted)+1) if sequence[i:i+len(wanted)] == wanted] if wanted else []


def checked_phones(phones, duration):
    if not phones:
        raise ValueError('empty source phone sequence')
    previous = 0.0
    for phone in phones:
        start, end = phone['start_ms'], phone['end_ms']
        if not all(isinstance(v, (int, float)) and math.isfinite(v) for v in (start, end)) or start < previous-.001 or end <= start or end > duration+1:
            raise ValueError('invalid source phone intervals')
        if not isinstance(phone['symbol'], str) or not phone['symbol']:
            raise ValueError('invalid source phone symbol')
        previous = end


def load_tool(filename):
    import importlib.util
    import sys
    path = Path(__file__).with_name(filename)
    name = '_utautts_' + path.stem.replace('-', '_')
    if name not in sys.modules:
        spec = importlib.util.spec_from_file_location(name, path)
        module = importlib.util.module_from_spec(spec)
        sys.modules[name] = module
        try:
            spec.loader.exec_module(module)
        except Exception:
            del sys.modules[name]
            raise
    return sys.modules[name]


def observed_source(unit, report_path):
    """原音パスはレポート基準。元の観測は変更しない。"""
    import copy
    result = copy.deepcopy(unit)
    result['source_clip'] = str((Path(report_path).parent / unit['source_clip']).resolve())
    return result
