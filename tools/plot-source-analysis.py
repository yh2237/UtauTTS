"""原音観測のレベルと周期性を境界候補と並べて描画する。"""
import argparse
import hashlib
import json
from pathlib import Path
import wave
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib import mlab
import numpy as np


def detailed_review(report_path, units, out, spans=None):
    fig, axes = plt.subplots(len(units), 3, figsize=(16, 3.5*len(units)), squeeze=False)
    for row, unit in enumerate(units):
        with wave.open(str(report_path.parent / unit['source_clip']), 'rb') as audio:
            if audio.getsampwidth() != 2 or audio.getcomptype() != 'NONE':
                raise ValueError('16-bit PCM source required')
            rate, channels = audio.getframerate(), audio.getnchannels()
            pcm = audio.readframes(audio.getnframes())
        digest = hashlib.sha256(f'v1/{rate}/{channels}/'.encode()+pcm).hexdigest()
        if digest != unit['analysis']['source_sha256']:
            raise ValueError('source PCM changed since observation')
        samples = np.frombuffer(pcm, dtype='<i2').reshape(-1, channels).mean(axis=1)/32768
        wave_axis, spectrum_axis, level_axis = axes[row]
        wave_axis.plot(np.arange(len(samples))*1000/rate, samples, lw=.4, color='#164f72')
        wave_axis.set_ylabel('PCM amplitude')
        nfft = min(len(samples), max(64, int(rate*.020)))
        hop = max(1, int(rate*.002))
        power, frequencies, times = mlab.specgram(samples, NFFT=nfft, Fs=rate, noverlap=max(0, nfft-hop))
        relative = 10*np.log10(np.maximum(power, 1e-20)/max(float(power.max()), 1e-20))
        spectrum_axis.pcolormesh(times*1000, frequencies/1000, relative, shading='auto', cmap='magma', vmin=-70, vmax=0)
        spectrum_axis.set_ylim(0, min(8, rate/2000))
        spectrum_axis.set_ylabel('Frequency: kHz')
        frames = unit['analysis']['frames']
        level_axis.plot([f['start_ms'] for f in frames], [f['rms_dbfs'] for f in frames], color='#164f72')
        level_axis.axhline(unit['analysis']['low_energy_threshold_dbfs'], color='gray', ls=':')
        level_axis.set_ylabel('20 ms RMS: dBFS'); level_axis.set_ylim(-100, 0)
        for landmark in unit['analysis']['landmarks']:
            if landmark['kind'] != 'energy-rise':
                level_axis.axvline(landmark['source_ms'], color='#267aa0', ls='--', lw=.8)
        phones = unit.get('forced_phone_intervals', [])
        for phone in phones:
            for axis in axes[row]:
                axis.axvline(phone['start_ms'], color='#2cba99', lw=1, ls=':')
                axis.text((phone['start_ms']+phone['end_ms'])/2, .95, phone['symbol'],
                          transform=axis.get_xaxis_transform(), ha='center', va='top', fontsize=9,
                          bbox=dict(facecolor='white', alpha=.8, edgecolor='none', pad=1))
        if phones:
            for axis in axes[row]:
                axis.axvline(phones[-1]['end_ms'], color='#2cba99', lw=1, ls=':')
        selected = (spans or {}).get(unit['unit_index'])
        if selected:
            if selected['source_sha256'] != digest or selected['alignment_sha256'] != unit['phone_alignment']['alignment_sha256']:
                raise ValueError('span source or alignment identity mismatch')
            for axis in axes[row]:
                axis.axvspan(selected['core_start_ms'], selected['core_end_ms'], color='#2cba99', alpha=.16,
                             label='selected core (unverified)')
                axis.axvspan(selected['context_start_ms'], selected['context_end_ms'], ymin=0, ymax=.025,
                             color='#267aa0', alpha=.9)
            level_axis.legend(loc='lower left', fontsize=8)
        for axis in axes[row]:
            axis.set_xlim(0, unit['analysis']['duration_ms'])
            axis.set_xlabel('ms after oto.offset'); axis.grid(alpha=.15)
        wave_axis.set_title(f"{unit['alias']} / source waveform")
        spectrum_axis.set_title('20 ms spectrum / 2 ms hop / relative power')
        level_axis.set_title('Forced phone boundaries are unverified')
    fig.subplots_adjust(left=.065, right=.99, bottom=.16 if len(units)==1 else .06,
                        top=.89 if len(units)==1 else .96, hspace=.6, wspace=.3)
    out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out, dpi=150); plt.close(fig)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--alias', action='append', default=[])
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--detail', action='store_true', help='raw waveform and spectrogram for boundary review')
    parser.add_argument('--spans', type=Path, help='optional source-span-mapping output for detailed review')
    args = parser.parse_args()
    report = json.loads(args.report.read_text(encoding='utf-8'))
    units = [u for u in report['units'] if not args.alias or u['alias'] in args.alias]
    if not units: raise ValueError('no matching units')
    spans = None
    if args.spans:
        if not args.detail:
            raise ValueError('--spans requires --detail')
        value = json.loads(args.spans.read_text(encoding='utf-8'))
        if value['observation_report_sha256'] != hashlib.sha256(args.report.read_bytes()).hexdigest():
            raise ValueError('span observation report identity mismatch')
        spans = {u['unit_index']:u for u in value['units']}
    if args.detail:
        detailed_review(args.report, units, args.out, spans)
        return
    fig, axes = plt.subplots(len(units), 2, figsize=(12, 3*len(units)), squeeze=False)
    colors = {'periodic':'#bfe4c6', 'aperiodic-high-crossing':'#ffd3a8',
              'mixed-or-uncertain':'#fff2c2', 'low-energy':'#dddddd'}
    markers = {'preutterance-transient':'#c12828', 'fixed-transient':'#8531a6', 'energy-rise':'#267aa0'}
    for row, unit in enumerate(units):
        a = unit['analysis']; frames = a['frames']; times = [f['start_ms'] for f in frames]
        left, right = axes[row]
        for region in a['regions']:
            for axis in (left, right):
                axis.axvspan(region['start_ms'], region['end_ms'], color=colors[region['evidence']], alpha=.6)
        left.plot(times, [f['rms_dbfs'] for f in frames], color='#164f72', label='20 ms RMS')
        left.axhline(a['low_energy_threshold_dbfs'], color='gray', ls=':', label='low-energy threshold')
        right.plot(times, [f['periodicity'] for f in frames], color='#164f72', label='40 ms periodicity')
        right.plot(times, [f['zero_crossing_rate'] for f in frames], color='#a25415', label='zero-crossing rate')
        right.set_ylim(0, 1.05)
        for phone in unit.get('forced_phone_intervals', []):
            right.axvline(phone['start_ms'], color='#8531a6', ls=':')
            right.text((phone['start_ms']+phone['end_ms'])/2, .97, phone['symbol'], ha='center', fontsize=9)
        if unit.get('forced_phone_intervals'):
            right.axvline(unit['forced_phone_intervals'][-1]['end_ms'], color='#8531a6', ls=':')
        for landmark in a['landmarks']:
            left.axvline(landmark['source_ms'], color=markers[landmark['kind']], ls='--', alpha=.8)
            if landmark['kind'] != 'energy-rise':
                left.text(landmark['source_ms'], -12 if landmark['kind']=='preutterance-transient' else -23,
                          f"{landmark['kind']}\n{landmark['source_ms']:.1f} ms", fontsize=8, ha='center')
        left.set_ylim(-100, 0); left.set_ylabel('dBFS')
        for axis in (left, right):
            axis.set_xlim(0, a['duration_ms']); axis.set_xlabel('Frame start: ms after oto.offset')
            axis.grid(alpha=.2); axis.legend(loc='lower left', fontsize=8)
        left.set_title(f"{unit['alias']} / source level and candidate landmarks")
        right.set_title('Acoustic evidence / forced phones (unverified)')
    fig.suptitle('Source understanding prototype: shaded regions are heuristic acoustic classes')
    fig.subplots_adjust(left=.09, right=.98, bottom=.14 if len(units)==1 else .05,
                        top=.82 if len(units)==1 else .94, hspace=.65, wspace=.23)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(args.out, dpi=150); plt.close(fig)

if __name__ == '__main__': main()
