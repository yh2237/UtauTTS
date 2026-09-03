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
	"unicode"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/processutil"
)

func utauPitchIntervalMS(tempo float64) float64 {
	return 60000.0 / tempo * 5.0 / 480.0
}

type externalUtauSegment struct {
	positionMS float64
	skipMS     float64
	wave       []float64
	envelope   []worldlineEnvelopePoint
}

type effectiveResamplerExpression struct {
	velocity, modulation int
	volume               *int
	flags                string
	tempo                float64
}

// utauResamplerArgumentsはOpenUtau互換の位置引数。
type utauResamplerArguments struct {
	input, output      string
	tone               int
	velocity           int
	flags              string
	offsetMS           float64
	requiredMS         float64
	consonantMS        float64
	cutoffMS           float64
	volume, modulation int
	tempo              float64
	pitches            []int
}

func (args utauResamplerArguments) commandLine() []string {
	return []string{
		args.input, args.output, midiToneName(args.tone), strconv.Itoa(args.velocity), args.flags,
		formatUtauNumber(args.offsetMS), formatUtauNumber(args.requiredMS),
		formatUtauNumber(args.consonantMS), formatUtauNumber(args.cutoffMS),
		strconv.Itoa(args.volume), strconv.Itoa(args.modulation),
		"!" + formatUtauNumber(args.tempo), encodeUtauPitch(args.pitches),
	}
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
	if cfg.ExternalWavtoolPath != "" {
		if info, statErr := os.Stat(cfg.ExternalWavtoolPath); statErr != nil || info.IsDir() {
			if statErr == nil {
				statErr = errors.New("path is a directory")
			}
			return nil, fmt.Errorf("external UTAU wavtool %q: %w", cfg.ExternalWavtoolPath, statErr)
		}
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
	velocity := 100
	if cfg.ExternalResamplerVelocitySet {
		velocity = cfg.ExternalResamplerVelocity
	}
	modulation := 0
	if cfg.ExternalResamplerModulationSet {
		modulation = cfg.ExternalResamplerModulation
	}
	tempo := cfg.ExternalResamplerTempo
	if tempo == 0 {
		tempo = 120
	}
	expressions, err := resolveResamplerExpressions(cfg.ExternalResamplerExpressions, effectiveResamplerExpression{
		velocity: velocity, flags: cfg.ExternalResamplerFlags, modulation: modulation, tempo: tempo,
	})
	if err != nil {
		return nil, err
	}
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
	wavtoolOutput := filepath.Join(tempDir, "phrase.wav")
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
		expressionPosition := unit.Position
		if unit.Role == "transition" {
			expressionPosition = unit.ParentPosition
		}
		expression := expressions.get(expressionPosition)
		unit.ResamplerVelocity = expression.velocity
		unit.ResamplerFlags = expression.flags
		unit.ResamplerModulation = expression.modulation
		unit.ResamplerTempo = expression.tempo
		pitchIntervalMS := utauPitchIntervalMS(expression.tempo)
		envelopePoints := openUtauEnvelopeFromTiming(*unit, timing)
		if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
			envelopePoints = cvvcPreBoundaryEnvelope(envelopePoints, timing)
		}
		stretchRatio := math.Pow(2, 1-float64(expression.velocity)*0.01)
		pitchLeadingMS := unit.PreutteranceMS * stretchRatio
		skipOverMS := pitchLeadingMS - timing.preutter
		skipMS := math.Max(0, skipOverMS)
		durationCorrection := timing.preutter - timing.tailIntrude + timing.tailOverlap
		requiredMS := math.Max(unit.DurationMS+durationCorrection+skipOverMS, unit.ConsonantMS)
		requiredMS = math.Ceil(requiredMS/50+0.5) * 50
		unitPitch := sourcePitches[index]
		if unitPitch <= 0 {
			unitPitch = reference
		}
		tone := max(0, min(127, int(math.Round(69+12*math.Log2(unitPitch/440)))))
		pitchLengthMS := envelopePoints[4].XMS + pitchLeadingMS
		pitchCount := max(0, int(math.Ceil(pitchLengthMS/pitchIntervalMS)))
		pitchValues := make([]int, pitchCount)
		pitchStartMS := unit.NoteStartMS - pitchLeadingMS
		for pitchIndex := range pitchValues {
			timeMS := pitchStartMS + float64(pitchIndex)*pitchIntervalMS
			target := externalTargetF0At(synthesisPlan, sourcePitches, factors, reference, timeMS)
			target *= pitchCurveFactorAt(cfg.PitchCurve, timeMS)
			pitchValues[pitchIndex] = int(math.Round(1200 * math.Log2(target/midiFrequency(tone))))
		}
		outputPath := filepath.Join(tempDir, fmt.Sprintf("unit-%04d.wav", index))
		volume := int(math.Round(math.Max(0, unit.EnergyFactor) * 100))
		if unit.EnergyFactor == 0 {
			volume = 100
		}
		if expression.volume != nil {
			volume = *expression.volume
		}
		if unit.Role == "transition" {
			volume = int(math.Round(float64(volume) * cfg.CVVCTransitionGain))
		}
		unit.ResamplerVolume = volume
		arguments := (utauResamplerArguments{
			input: unit.Source, output: outputPath, tone: tone,
			velocity: expression.velocity, flags: expression.flags, offsetMS: unit.OffsetMS, requiredMS: requiredMS,
			consonantMS: unit.ConsonantMS, cutoffMS: unit.CutoffMS,
			volume: volume, modulation: expression.modulation, tempo: expression.tempo, pitches: pitchValues,
		}).commandLine()
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
		unit.SourceF0Hz = sourcePitches[index]
		unit.TargetF0Hz = externalTargetF0At(synthesisPlan, sourcePitches, factors, reference, unit.NoteStartMS)
		unit.TargetF0Hz *= pitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS)
		unit.IntonationFactor = intonation[index]
		if cfg.ExternalWavtoolPath != "" {
			durationTicks := unit.DurationMS * expression.tempo * 480 / 60000
			duration := formatUtauNumber(durationTicks) + "@" + formatUtauNumber(expression.tempo)
			if durationCorrection >= 0 {
				duration += "+"
			}
			duration += formatUtauNumber(durationCorrection)
			if err := runExternalWavtool(cfg.Context, cfg.ExternalWavtoolPath, wavtoolOutput, outputPath,
				skipOverMS, duration, envelopePoints, timing.overlap); err != nil {
				return nil, fmt.Errorf("wavtool unit %q: %w", unit.Alias, err)
			}
			continue
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
		segments = append(segments, externalUtauSegment{
			positionMS: unit.NoteStartMS - timing.preutter + leadingMS,
			skipMS:     skipMS, wave: pcmFloats(pcm.Data), envelope: envelopePoints,
		})
	}
	if cfg.ExternalWavtoolPath != "" {
		if err := finalizeExternalWavtoolOutput(wavtoolOutput); err != nil {
			return nil, err
		}
		pcm, err := audio.ReadWav(wavtoolOutput)
		if err != nil {
			return nil, fmt.Errorf("read external wavtool output: %w", err)
		}
		return toMono(pcm), nil
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

func resolveResamplerExpressions(values []ResamplerExpression, defaults effectiveResamplerExpression) (resamplerExpressionDefaults, error) {
	if !validResamplerExpression(defaults) {
		return resamplerExpressionDefaults{}, errors.New("invalid default resampler expression")
	}
	result := map[int]effectiveResamplerExpression{}
	for _, value := range values {
		if value.Position < 0 {
			return resamplerExpressionDefaults{}, fmt.Errorf("resampler expression position must not be negative: %d", value.Position)
		}
		if _, exists := result[value.Position]; exists {
			return resamplerExpressionDefaults{}, fmt.Errorf("duplicate resampler expression position %d", value.Position)
		}
		effective := defaults
		if value.Velocity != nil {
			effective.velocity = *value.Velocity
		}
		if value.Volume != nil {
			effective.volume = value.Volume
		}
		if value.Flags != nil {
			effective.flags = *value.Flags
		}
		if value.Modulation != nil {
			effective.modulation = *value.Modulation
		}
		if value.Tempo != nil {
			effective.tempo = *value.Tempo
		}
		if !validResamplerExpression(effective) {
			return resamplerExpressionDefaults{}, fmt.Errorf("invalid resampler expression at position %d", value.Position)
		}
		result[value.Position] = effective
	}
	return resamplerExpressionDefaults{values: result, fallback: defaults}, nil
}

func validResamplerExpression(value effectiveResamplerExpression) bool {
	return value.velocity >= 0 && value.velocity <= 200 && (value.volume == nil || *value.volume >= 0 && *value.volume <= 200) &&
		value.modulation >= 0 && value.modulation <= 100 &&
		value.tempo > 0 && value.tempo <= 1000 && !math.IsNaN(value.tempo) && !math.IsInf(value.tempo, 0) &&
		strings.IndexFunc(value.flags, unicode.IsSpace) < 0
}

type resamplerExpressionDefaults struct {
	values   map[int]effectiveResamplerExpression
	fallback effectiveResamplerExpression
}

func (values resamplerExpressionDefaults) get(position int) effectiveResamplerExpression {
	if value, ok := values.values[position]; ok {
		return value
	}
	return values.fallback
}

func runExternalWavtool(ctx context.Context, executable, output, input string, skipMS float64, duration string, points []worldlineEnvelopePoint, overlapMS float64) error {
	if len(points) != 5 {
		return fmt.Errorf("wavtool envelope needs 5 points; got %d", len(points))
	}
	percent := func(value float64) string { return formatUtauNumber(value * 100) }
	arguments := []string{
		output, input, formatUtauNumber(skipMS), duration,
		"0.000000",
		formatUtauNumber(points[1].XMS - points[0].XMS),
		formatUtauNumber(points[4].XMS - points[3].XMS),
		percent(points[0].Y), percent(points[1].Y), percent(points[3].Y), percent(points[4].Y),
		formatUtauNumber(overlapMS), "0.000000",
		formatUtauNumber(points[2].XMS - points[1].XMS), percent(points[2].Y),
	}
	if ctx == nil {
		ctx = context.Background()
	}
	commandContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandContext, executable, arguments...)
	command.Dir = filepath.Dir(executable)
	processutil.Configure(command)
	outputText, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(outputText))
		if detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

func finalizeExternalWavtoolOutput(output string) error {
	if info, err := os.Stat(output); err == nil && !info.IsDir() {
		return nil
	}
	header, headerErr := os.ReadFile(output + ".whd")
	data, dataErr := os.ReadFile(output + ".dat")
	if headerErr != nil || dataErr != nil {
		return fmt.Errorf("external wavtool produced neither WAV nor whd/dat pair")
	}
	combined := make([]byte, 0, len(header)+len(data))
	combined = append(combined, header...)
	combined = append(combined, data...)
	if err := os.WriteFile(output, combined, 0o600); err != nil {
		return fmt.Errorf("combine external wavtool output: %w", err)
	}
	return nil
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
