package render

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/processutil"
)

const utauPitchIntervalMS = 60000.0 / 120.0 * 5.0 / 480.0

type externalUtauSegment struct {
	positionMS float64
	skipMS     float64
	wave       []float64
	envelope   []worldlineEnvelopePoint
}

func renderUtauExternalResampler(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	resampler := strings.TrimSpace(cfg.ExternalResamplerPath)
	if resampler == "" {
		return nil, errors.New("external UTAU resampler is not configured by the renderer plugin")
	}
	info, err := os.Stat(resampler)
	if err != nil || info.IsDir() {
		if err == nil {
			err = errors.New("path is a directory")
		}
		return nil, fmt.Errorf("external UTAU resampler %q: %w", resampler, err)
	}
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = CVVCTimingLegacy
	}
	if cfg.CVVCTiming != CVVCTimingLegacy && cfg.CVVCTiming != CVVCTimingSequential {
		return nil, fmt.Errorf("unknown CVVC timing mode %q", cfg.CVVCTiming)
	}
	if cfg.CVVCTransitionGain == 0 {
		cfg.CVVCTransitionGain = 1
	}
	if cfg.CVVCTransitionGain < 0 || cfg.CVVCTransitionGain > 1 {
		return nil, fmt.Errorf("CVVC transition gain must be between 0 and 1; got %.3f", cfg.CVVCTransitionGain)
	}
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	phoneTimings, phraseStartMS := openUtauPhoneTimings(synthesisPlan.Units, cfg.CVVCTiming)
	leadingMS := limitLeadingPreutterance(math.Max(0, -phraseStartMS), cfg.LeadingPreutteranceMS)
	synthesisPlan.LeadingMarginMS = leadingMS

	cache := newSourceCache()
	timings := make([]effectiveTiming, len(synthesisPlan.Units))
	for index := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[index]
		timings[index] = normalizeTiming(*unit, cfg.ReleaseMS)
		if !unit.Silent {
			timings[index].preutteranceMS = phoneTimings[index].preutter
			timings[index].overlapMS = phoneTimings[index].overlap
			timings[index].consonantMS = unit.ConsonantMS
			timings[index].scale = 1
		}
		unit.TimingScale = timings[index].scale
		unit.EffectivePreutteranceMS = timings[index].preutteranceMS
		unit.EffectiveConsonantMS = timings[index].consonantMS
		unit.EffectiveOverlapMS = timings[index].overlapMS
		unit.IntonationFactor = 1
	}
	intonation := identityFactors(len(synthesisPlan.Units))
	sourcePitches, _, err := measureWorldlinePitches(synthesisPlan, &cache)
	if err != nil {
		return nil, err
	}
	if cfg.ApplyPitch {
		intonation = analyzeIntonationFromPitches(synthesisPlan, timings, sourcePitches, cfg.IntonationStrength)
	}
	reference := medianFloat(nonzeroFloats(sourcePitches))
	if reference <= 0 {
		reference = 220
	}
	factors := make([]float64, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		factors[index] = intonation[index] * effectiveUnitPitchFactor(unit, cfg.ApplyPitch)
	}

	tempDir, err := os.MkdirTemp("", "utautts-resampler-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	segments := make([]externalUtauSegment, 0, len(synthesisPlan.Units))
	sampleRate := 0
	for index := range synthesisPlan.Units {
		if err := contextError(cfg.Context); err != nil {
			return nil, err
		}
		unit := &synthesisPlan.Units[index]
		if unit.Silent {
			continue
		}
		timing := phoneTimings[index]
		envelopePoints := openUtauEnvelopeFromTiming(*unit, timing)
		if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
			envelopePoints = cvvcPreBoundaryEnvelope(envelopePoints, timing)
		}
		pitchLeadingMS := unit.PreutteranceMS
		skipMS := math.Max(0, pitchLeadingMS-timing.preutter)
		durationCorrection := timing.preutter - timing.tailIntrude + timing.tailOverlap
		requiredMS := math.Max(unit.DurationMS+durationCorrection+skipMS, unit.ConsonantMS)
		requiredMS = math.Ceil(requiredMS/50+0.5) * 50
		unitPitch := sourcePitches[index]
		if unitPitch <= 0 {
			unitPitch = reference
		}
		tone := max(0, min(127, int(math.Round(69+12*math.Log2(unitPitch/440)))))
		pitchLengthMS := envelopePoints[4].XMS + pitchLeadingMS
		pitchCount := max(0, int(math.Ceil(pitchLengthMS/utauPitchIntervalMS)))
		pitchValues := make([]int, pitchCount)
		pitchStartMS := unit.NoteStartMS - pitchLeadingMS
		for pitchIndex := range pitchValues {
			timeMS := pitchStartMS + float64(pitchIndex)*utauPitchIntervalMS
			target := externalTargetF0At(synthesisPlan, sourcePitches, factors, reference, timeMS)
			target *= pitchCurveFactorAt(cfg.PitchCurve, timeMS)
			pitchValues[pitchIndex] = int(math.Round(1200 * math.Log2(target/midiFrequency(tone))))
		}
		outputPath := filepath.Join(tempDir, fmt.Sprintf("unit-%04d.wav", index))
		volume := int(math.Round(math.Max(0, unit.EnergyFactor) * 100))
		if unit.EnergyFactor == 0 {
			volume = 100
		}
		if unit.Role == "transition" {
			volume = int(math.Round(float64(volume) * cfg.CVVCTransitionGain))
		}
		arguments := []string{
			unit.Source, outputPath, midiToneName(tone), "100", "",
			formatUtauNumber(unit.OffsetMS), formatUtauNumber(requiredMS),
			formatUtauNumber(unit.ConsonantMS), formatUtauNumber(unit.CutoffMS),
			strconv.Itoa(volume), "0", "!120", encodeUtauPitch(pitchValues),
		}
		ctx := cfg.Context
		if ctx == nil {
			ctx = context.Background()
		}
		commandContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
		command := exec.CommandContext(commandContext, resampler, arguments...)
		command.Dir = filepath.Dir(resampler)
		processutil.Configure(command)
		output, commandErr := command.CombinedOutput()
		cancel()
		if commandErr != nil {
			detail := strings.TrimSpace(string(output))
			if detail != "" {
				commandErr = fmt.Errorf("%w: %s", commandErr, detail)
			}
			return nil, fmt.Errorf("resample unit %q: %w", unit.Alias, commandErr)
		}
		pcm, err := audio.ReadWav(outputPath)
		if err != nil {
			return nil, fmt.Errorf("read resampled unit %q: %w", unit.Alias, err)
		}
		pcm = toMono(pcm)
		if sampleRate == 0 {
			sampleRate = pcm.SampleRate
		}
		if pcm.SampleRate != sampleRate {
			pcm = resampleRate(pcm, sampleRate)
		}
		unit.SourceF0Hz = sourcePitches[index]
		unit.TargetF0Hz = externalTargetF0At(synthesisPlan, sourcePitches, factors, reference, unit.NoteStartMS)
		unit.TargetF0Hz *= pitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS)
		unit.IntonationFactor = intonation[index]
		segments = append(segments, externalUtauSegment{
			positionMS: unit.NoteStartMS - timing.preutter + leadingMS,
			skipMS:     skipMS, wave: pcmFloats(pcm.Data), envelope: envelopePoints,
		})
	}
	if sampleRate == 0 || len(segments) == 0 {
		return nil, errors.New("external UTAU resampler produced no samples")
	}
	minimumFrames := msToFrames(synthesisPlan.DurationMS+cfg.ReleaseMS+leadingMS, sampleRate)
	mix := make([]float64, minimumFrames)
	for _, segment := range segments {
		position := msToFramesSigned(segment.positionMS, sampleRate)
		skip := msToFrames(segment.skipMS, sampleRate)
		for sourceIndex := skip; sourceIndex < len(segment.wave); sourceIndex++ {
			destination := position + sourceIndex - skip
			if destination < 0 {
				continue
			}
			if destination >= len(mix) {
				mix = append(mix, make([]float64, destination-len(mix)+1)...)
			}
			gain := externalEnvelopeGain(sourceIndex, sampleRate, segment.skipMS, segment.envelope)
			mix[destination] += segment.wave[sourceIndex] * gain
		}
	}
	preventClipping(mix, 0.98)
	return &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: floatPCM(mix)}, nil
}

func externalTargetF0At(synthesisPlan *plan.Plan, pitches, factors []float64, reference, timeMS float64) float64 {
	index := 0
	for index+1 < len(synthesisPlan.Units) && synthesisPlan.Units[index+1].NoteStartMS <= timeMS {
		index++
	}
	target := func(position int) float64 {
		value := pitches[position]
		if value <= 0 {
			value = reference
		}
		return value * factors[position]
	}
	value := target(index)
	if index+1 < len(pitches) {
		left, right := synthesisPlan.Units[index].NoteStartMS, synthesisPlan.Units[index+1].NoteStartMS
		if right > left {
			progress := math.Max(0, math.Min(1, (timeMS-left)/(right-left)))
			value = math.Exp(math.Log(value)*(1-progress) + math.Log(target(index+1))*progress)
		}
	}
	return value
}

func externalEnvelopeGain(sample, sampleRate int, skipMS float64, points []worldlineEnvelopePoint) float64 {
	if len(points) == 0 {
		return 1
	}
	timeMS := float64(sample) / float64(sampleRate) * 1000
	shift := -points[0].XMS + skipMS
	if timeMS <= points[0].XMS+shift {
		return points[0].Y
	}
	for index := 1; index < len(points); index++ {
		left, right := points[index-1], points[index]
		if timeMS <= right.XMS+shift {
			span := right.XMS - left.XMS
			if span <= 0 {
				return left.Y
			}
			progress := (timeMS - (left.XMS + shift)) / span
			return left.Y + (right.Y-left.Y)*progress
		}
	}
	return points[len(points)-1].Y
}

func formatUtauNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func midiFrequency(tone int) float64 {
	return 440 * math.Pow(2, float64(tone-69)/12)
}

func midiToneName(tone int) string {
	tone = max(0, min(127, tone))
	names := [...]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	return names[tone%12] + strconv.Itoa(tone/12-1)
}

func encodeUtauPitch(values []int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	encodedValue := func(value int) string {
		value = max(-2048, min(2047, value))
		if value < 0 {
			value += 4096
		}
		return string([]byte{alphabet[(value>>6)&0x3f], alphabet[value&0x3f]})
	}
	var builder strings.Builder
	last, duplicates := "", 0
	for _, value := range values {
		encoded := encodedValue(value)
		if encoded == last {
			duplicates++
			continue
		}
		if duplicates > 0 {
			fmt.Fprintf(&builder, "#%d#", duplicates)
			duplicates = 0
		}
		builder.WriteString(encoded)
		last = encoded
	}
	if duplicates > 0 {
		fmt.Fprintf(&builder, "#%d#", duplicates)
	}
	return builder.String()
}
