package openutau

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// USTX export for UtauTTS projects.
//
// A `.utautts` project (the GUI project file) is converted into an OpenUtau
// `.ustx` file (YAML 1.2, ustx_version 0.6) so the same text, timing, tone
// and pitch parameters can be opened and further edited in OpenUtau.
//
// Mapping rules:
//   - one voice part per utterance card
//   - one track per voicebank used by the project
//   - one note per mora (lyric = mora, tone = card tone, duration = mora duration)
//   - manual pitch edits become note pitch data (cents offsets)
//   - millisecond timing is converted to ticks at the exported BPM (default 120)

const utauTTSProjectFormat = "utautts-project"

// UtauTTSProject mirrors the GUI project file (`projectData()` in Main.qml).
type UtauTTSProject struct {
	Format        string             `json:"format"`
	FormatVersion int                `json:"format_version"`
	AppVersion    string             `json:"app_version,omitempty"`
	Utterances    []UtauTTSUtterance `json:"utterances"`
	SelectedIndex int                `json:"selected_index,omitempty"`
}

// UtauTTSUtterance is one synthesis card in a UtauTTS project.
type UtauTTSUtterance struct {
	Text                 string               `json:"text"`
	VoicebankID          string               `json:"voicebank_id"`
	ModelID              string               `json:"model_id,omitempty"`
	RendererID           string               `json:"renderer_id,omitempty"`
	AliasPolicy          string               `json:"alias_policy,omitempty"`
	Tone                 string               `json:"tone"`
	Color                string               `json:"color,omitempty"`
	MoraDurationMS       float64              `json:"mora_duration_ms"`
	PauseDurationMS      float64              `json:"pause_duration_ms"`
	Intonation           float64              `json:"intonation"`
	ApplyPitch           bool                 `json:"apply_pitch"`
	PitchPoints          []float64            `json:"pitch_points,omitempty"`
	MoraDurationsMS      []float64            `json:"mora_durations_ms,omitempty"`
	MoraPositionsMS      []float64            `json:"mora_positions_ms,omitempty"`
	AutomaticPitchPoints []float64            `json:"automatic_pitch_points,omitempty"`
	AutomaticMoraDurMS   []float64            `json:"automatic_mora_durations_ms,omitempty"`
	AutomaticMoraPosMS   []float64            `json:"automatic_mora_positions_ms,omitempty"`
	ManualPitchEdited    bool                 `json:"manual_pitch_edited"`
	ManualMoraDurEdited  bool                 `json:"manual_mora_duration_edited"`
	AnalysisCache        UtauTTSAnalysisCache `json:"analysis_cache"`
}

// UtauTTSAnalysisCache holds the cached reading and mora analysis of a card.
type UtauTTSAnalysisCache struct {
	Reading string        `json:"reading"`
	Morae   []UtauTTSMora `json:"morae"`
}

// UtauTTSMora is a single mora of the analyzed reading.
type UtauTTSMora struct {
	Position  int    `json:"position"`
	Mora      string `json:"mora"`
	Pause     bool   `json:"pause"`
	Consonant string `json:"consonant,omitempty"`
	Vowel     string `json:"vowel,omitempty"`
}

// LoadUtauTTSProject reads and validates a `.utautts` project file.
func LoadUtauTTSProject(path string) (*UtauTTSProject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read UtauTTS project: %w", err)
	}
	return ParseUtauTTSProject(data)
}

// ParseUtauTTSProject parses and validates `.utautts` project JSON.
func ParseUtauTTSProject(data []byte) (*UtauTTSProject, error) {
	var project UtauTTSProject
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parse UtauTTS project: %w", err)
	}
	if project.Format != utauTTSProjectFormat {
		return nil, fmt.Errorf("not a UtauTTS project file (format %q)", project.Format)
	}
	if project.FormatVersion < 1 {
		return nil, fmt.Errorf("unsupported UtauTTS project format version %d", project.FormatVersion)
	}
	for index := range project.Utterances {
		utterance := &project.Utterances[index]
		if utterance.VoicebankID == "" {
			return nil, fmt.Errorf("utterance %d: missing voicebank_id", index)
		}
		if utterance.Tone == "" {
			utterance.Tone = "C4"
		}
		if _, err := ToneToMIDI(utterance.Tone); err != nil {
			return nil, fmt.Errorf("utterance %d: tone %q: %w", index, utterance.Tone, err)
		}
	}
	return &project, nil
}

// ExportOptions controls USTX export.
type ExportOptions struct {
	// ProjectName is the USTX project name (default "UtauTTS Project").
	ProjectName string
	// BPM is the tempo used to convert millisecond timing to ticks (default 120).
	BPM float64
	// Curves holds optional 10ms frame-level intonation contours aligned with
	// project.Utterances (nil entries fall back to mora-level pitch data).
	// When present, note pitch is sampled from the smooth contour instead of
	// emitting a flat line per mora.
	Curves []FrameCurve
}

// FrameCurve is a frame-level pitch contour (cents at FrameMS intervals),
// independent of the render package to keep the exporter self-contained.
type FrameCurve struct {
	FrameMS float64
	Cents   []float64
}

// curveCentsAt linearly interpolates the contour at time tMS.
func curveCentsAt(curve *FrameCurve, tMS float64) float64 {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return 0
	}
	position := tMS / curve.FrameMS
	left := int(math.Floor(position))
	if left < 0 {
		return curve.Cents[0]
	}
	if left >= len(curve.Cents)-1 {
		return curve.Cents[len(curve.Cents)-1]
	}
	progress := position - float64(left)
	return curve.Cents[left]*(1-progress) + curve.Cents[left+1]*progress
}

// ExportUSTX converts a UtauTTS project into a USTX YAML document.
func ExportUSTX(project *UtauTTSProject, opts ExportOptions) ([]byte, error) {
	if opts.ProjectName == "" {
		opts.ProjectName = "UtauTTS Project"
	}
	if opts.BPM <= 0 {
		opts.BPM = 120
	}
	msToTicks := func(ms float64) int {
		return int(math.Round(ms * float64(ustxResolution) / (60000.0 / opts.BPM)))
	}

	document := USTXProject{
		Name:           opts.ProjectName,
		Comment:        "Exported by UtauTTS",
		OutputDir:      "Vocal",
		CacheDir:       "UCache",
		USTXVersion:    "0.6",
		BPM:            opts.BPM,
		BeatPerBar:     4,
		BeatUnit:       4,
		Resolution:     ustxResolution,
		TimeSignatures: []USTXTimeSignature{{BarPosition: 0, BeatPerBar: 4, BeatUnit: 4}},
		Tempos:         []USTXTempo{{Position: 0, BPM: opts.BPM}},
		Expressions:    defaultUSTXExpressions(),
		ExpSelectors:   []string{"dyn", "pitd", "clr", "eng", "vel", "vol", "atk", "dec", "gen", "bre"},
	}

	trackByVoicebank := make(map[string]int)
	nextPartPosition := 0
	for utteranceIndex, utterance := range project.Utterances {
		if !hasVoicedMora(utterance) {
			continue
		}
		var curve *FrameCurve
		if utteranceIndex < len(opts.Curves) {
			curve = &opts.Curves[utteranceIndex]
			if curve.FrameMS <= 0 || len(curve.Cents) == 0 {
				curve = nil
			}
		}
		voicebank := utterance.VoicebankID
		trackIndex, ok := trackByVoicebank[voicebank]
		if !ok {
			trackIndex = len(document.Tracks)
			trackByVoicebank[voicebank] = trackIndex
			document.Tracks = append(document.Tracks, USTXTrack{
				TrackName:   voicebank,
				Singer:      voicebank,
				Phonemizer:  "OpenUtau.Core.DefaultPhonemizer",
				TrackColor:  "Blue",
				VoiceColors: []string{""},
			})
		}

		part, err := utteranceToVoicePart(utterance, trackIndex, msToTicks, curve)
		if err != nil {
			return nil, fmt.Errorf("utterance %q: %w", utterance.Text, err)
		}
		// カードの順番を保ち、音源が異なるカードも同時再生されないように並べる。
		part.Position = nextPartPosition
		var partEnd int
		if len(part.Notes) > 0 {
			last := part.Notes[len(part.Notes)-1]
			partEnd = last.Position + last.Duration
		}
		document.VoiceParts = append(document.VoiceParts, part)
		nextPartPosition = part.Position + partEnd + ustxResolution
	}
	if len(document.VoiceParts) == 0 {
		return nil, errors.New("project contains no utterances with notes to export")
	}

	data, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal USTX: %w", err)
	}
	return data, nil
}

// hasVoicedMora reports whether the utterance has at least one non-pause mora.
func hasVoicedMora(utterance UtauTTSUtterance) bool {
	for _, mora := range utterance.AnalysisCache.Morae {
		if !mora.Pause && mora.Mora != "" {
			return true
		}
	}
	return false
}

func utteranceToVoicePart(utterance UtauTTSUtterance, trackIndex int, msToTicks func(float64) int, curve *FrameCurve) (USTXVoicePart, error) {
	tone, err := ToneToMIDI(utterance.Tone)
	if err != nil {
		return USTXVoicePart{}, err
	}
	part := USTXVoicePart{
		Name:     firstLine(utterance.Text),
		Comment:  utterance.AnalysisCache.Reading,
		TrackNo:  trackIndex,
		Position: 0,
	}
	moraDuration := utterance.MoraDurationMS
	if moraDuration <= 0 {
		moraDuration = 140
	}
	pauseDuration := utterance.PauseDurationMS
	if pauseDuration <= 0 {
		pauseDuration = 180
	}

	// Prefer manual mora durations, then automatic durations, then the base.
	durations := utterance.MoraDurationsMS
	if !utterance.ManualMoraDurEdited || len(durations) == 0 {
		durations = utterance.AutomaticMoraDurMS
	}
	// Prefer explicit mora positions when present.
	positions := utterance.MoraPositionsMS
	if !utterance.ManualMoraDurEdited || len(positions) == 0 {
		if len(utterance.AutomaticMoraPosMS) > 0 {
			positions = utterance.AutomaticMoraPosMS
		} else {
			positions = nil
		}
	}

	cursorTicks := 0
	cursorMS := 0.0
	usedPositions := len(positions) > 0
	for index, mora := range utterance.AnalysisCache.Morae {
		if mora.Pause {
			if !usedPositions {
				cursorTicks += msToTicks(pauseDuration)
				cursorMS += pauseDuration
			}
			continue
		}
		durationMS := moraDuration
		if index < len(durations) && durations[index] > 0 {
			durationMS = durations[index]
		}
		positionMS := cursorMS
		if usedPositions && index < len(positions) {
			positionMS = positions[index]
		}
		positionTicks := msToTicks(positionMS)
		durationTicks := msToTicks(durationMS)

		manualOffset := 0.0
		if utterance.ManualPitchEdited && index < len(utterance.PitchPoints) {
			manualOffset = utterance.PitchPoints[index]
		}
		var pitchData []USTXPitchPoint
		if curve != nil {
			// Sample the smooth 10ms contour across the note span. Manual
			// offsets are per-mora relative corrections on top of the model
			// contour (matching synthesis behavior).
			span := math.Max(1, durationMS)
			for t := 0.0; t < span; t += curve.FrameMS {
				cents := curveCentsAt(curve, positionMS+t) + manualOffset
				pitchData = append(pitchData, USTXPitchPoint{X: t, Y: cents / 10, Shape: "io"})
			}
			cents := curveCentsAt(curve, positionMS+span) + manualOffset
			if len(pitchData) == 0 || pitchData[len(pitchData)-1].X < span-0.5 {
				pitchData = append(pitchData, USTXPitchPoint{X: span, Y: cents / 10, Shape: "io"})
			}
		} else {
			cents := manualOffset
			if !utterance.ManualPitchEdited && index < len(utterance.AutomaticPitchPoints) {
				cents = utterance.AutomaticPitchPoints[index]
			}
			pitchData = []USTXPitchPoint{
				{X: 0, Y: cents / 10, Shape: "io"},
				{X: durationMS, Y: cents / 10, Shape: "io"},
			}
		}
		// USTX pitch semantics (see OpenUtau UNotePitch.Sample):
		//   - X is in milliseconds relative to the note start
		//   - Y is in 0.1 semitones (10 cents per unit)
		//   - snap_first must be false, otherwise OpenUtau resets the first
		//     point to 0 on load and the exported pitch is lost.
		note := USTXNote{
			Position: positionTicks,
			Duration: durationTicks,
			Tone:     tone,
			Lyric:    exportLyric(mora, part.Notes),
			Vibrato:  USTXVibrato{Length: 0, Period: 175, Depth: 25, In: 10, Out: 10, Shift: 0, Drift: 0},
			Pitch: USTXPitch{
				Data:      pitchData,
				SnapFirst: false,
			},
		}
		part.Notes = append(part.Notes, note)
		cursorTicks = positionTicks + durationTicks
		cursorMS = positionMS + durationMS
	}
	if len(part.Notes) == 0 {
		return USTXVoicePart{}, fmt.Errorf("no notes to export")
	}
	return part, nil
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexAny(value, "\n\r"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	if value == "" {
		return "New Part"
	}
	return value
}

// exportLyric decides the USTX note lyric for a mora. A long vowel mark (ー)
// has no phoneme of its own in the voicebank, so it is exported as a UTAU
// extension note ("+vowel", e.g. "+お") which OpenUtau renders as an
// extension of the previous note. A sokuon (っ) stays as-is; only the long
// vowel mark is converted.
func exportLyric(mora UtauTTSMora, notes []USTXNote) string {
	if mora.Mora != "ー" {
		return mora.Mora
	}
	switch mora.Vowel {
	case "a":
		return "+あ"
	case "i":
		return "+い"
	case "u":
		return "+う"
	case "e":
		return "+え"
	case "o":
		return "+お"
	}
	if len(notes) > 0 {
		runes := []rune(notes[len(notes)-1].Lyric)
		if len(runes) > 0 && runes[len(runes)-1] != '+' {
			return "+" + string(runes[len(runes)-1])
		}
	}
	return "+"
}

// ToneToMIDI converts a note name such as "C4", "B3" or "F#5" to a MIDI
// note number (C4 = 60).
func ToneToMIDI(tone string) (int, error) {
	tone = strings.TrimSpace(tone)
	if number, err := strconv.Atoi(tone); err == nil {
		if number < 0 || number > 127 {
			return 0, fmt.Errorf("midi note %d out of range", number)
		}
		return number, nil
	}
	if len(tone) < 2 {
		return 0, fmt.Errorf("expected note name like C4")
	}
	letter := strings.ToUpper(tone[:1])
	semitone, ok := midiSemitones[letter]
	if !ok {
		return 0, fmt.Errorf("unknown note letter %q", letter)
	}
	rest := tone[1:]
	switch {
	case strings.HasPrefix(rest, "#"):
		semitone++
		rest = rest[1:]
	case strings.HasPrefix(rest, "b"):
		semitone--
		rest = rest[1:]
	}
	octave, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf("invalid octave in %q", tone)
	}
	midi := (octave+1)*12 + semitone
	if midi < 0 || midi > 127 {
		return 0, fmt.Errorf("note %q out of MIDI range", tone)
	}
	return midi, nil
}

var midiSemitones = map[string]int{
	"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11,
}

// USTX document model. YAML field names follow the OpenUtau `.ustx` format
// (see OpenUtau.Core/Ustx and the USTX file format wiki).
const ustxResolution = 480

type USTXProject struct {
	Name           string              `yaml:"name"`
	Comment        string              `yaml:"comment,omitempty"`
	OutputDir      string              `yaml:"output_dir,omitempty"`
	CacheDir       string              `yaml:"cache_dir,omitempty"`
	USTXVersion    string              `yaml:"ustx_version"`
	BPM            float64             `yaml:"bpm,omitempty"`
	BeatPerBar     int                 `yaml:"beat_per_bar,omitempty"`
	BeatUnit       int                 `yaml:"beat_unit,omitempty"`
	Resolution     int                 `yaml:"resolution,omitempty"`
	TimeSignatures []USTXTimeSignature `yaml:"time_signatures,omitempty"`
	Tempos         []USTXTempo         `yaml:"tempos,omitempty"`
	Expressions    map[string]USTXExpr `yaml:"expressions,omitempty"`
	ExpSelectors   []string            `yaml:"exp_selectors,omitempty"`
	ExpPrimary     *int                `yaml:"exp_primary,omitempty"`
	ExpSecondary   *int                `yaml:"exp_secondary,omitempty"`
	Key            *int                `yaml:"key,omitempty"`
	Tracks         []USTXTrack         `yaml:"tracks,omitempty"`
	VoiceParts     []USTXVoicePart     `yaml:"voice_parts,omitempty"`
	WaveParts      []any               `yaml:"wave_parts,omitempty"`
}

type USTXTimeSignature struct {
	BarPosition int `yaml:"bar_position"`
	BeatPerBar  int `yaml:"beat_per_bar"`
	BeatUnit    int `yaml:"beat_unit"`
}

type USTXTempo struct {
	Position int     `yaml:"position"`
	BPM      float64 `yaml:"bpm"`
}

type USTXExpr struct {
	Name         string   `yaml:"name"`
	Abbr         string   `yaml:"abbr"`
	Type         string   `yaml:"type"`
	Min          float64  `yaml:"min"`
	Max          float64  `yaml:"max"`
	DefaultValue float64  `yaml:"default_value"`
	IsFlag       bool     `yaml:"is_flag"`
	Flag         string   `yaml:"flag,omitempty"`
	Options      []string `yaml:"options,omitempty"`
}

type USTXTrack struct {
	TrackName   string   `yaml:"track_name,omitempty"`
	TrackColor  string   `yaml:"track_color,omitempty"`
	VoiceColors []string `yaml:"voice_color_names,omitempty"`
	Singer      string   `yaml:"singer,omitempty"`
	Phonemizer  string   `yaml:"phonemizer,omitempty"`
	Mute        bool     `yaml:"mute"`
	Solo        bool     `yaml:"solo"`
	Volume      float64  `yaml:"volume"`
	TrackExps   []any    `yaml:"track_expressions,omitempty"`
}

type USTXVoicePart struct {
	Name     string     `yaml:"name"`
	Comment  string     `yaml:"comment,omitempty"`
	TrackNo  int        `yaml:"track_no"`
	Position int        `yaml:"position"`
	Notes    []USTXNote `yaml:"notes"`
	Curves   []any      `yaml:"curves,omitempty"`
}

type USTXNote struct {
	Position           int         `yaml:"position"`
	Duration           int         `yaml:"duration"`
	Tone               int         `yaml:"tone"`
	Lyric              string      `yaml:"lyric"`
	Pitch              USTXPitch   `yaml:"pitch,omitempty"`
	Vibrato            USTXVibrato `yaml:"vibrato,omitempty"`
	PhonemeExpressions []any       `yaml:"phoneme_expressions,omitempty"`
	PhonemeOverrides   []any       `yaml:"phoneme_overrides,omitempty"`
}

type USTXPitch struct {
	Data      []USTXPitchPoint `yaml:"data"`
	SnapFirst bool             `yaml:"snap_first"`
}

type USTXPitchPoint struct {
	X     float64 `yaml:"x"`
	Y     float64 `yaml:"y"`
	Shape string  `yaml:"shape"`
}

type USTXVibrato struct {
	Length float64 `yaml:"length"`
	Period float64 `yaml:"period"`
	Depth  float64 `yaml:"depth"`
	In     float64 `yaml:"in"`
	Out    float64 `yaml:"out"`
	Shift  float64 `yaml:"shift"`
	Drift  float64 `yaml:"drift"`
}

// defaultUSTXExpressions returns the standard OpenUtau expression set, matching
// the template used by UtaFormatix so the project opens with familiar defaults.
func defaultUSTXExpressions() map[string]USTXExpr {
	return map[string]USTXExpr{
		"dyn":  {Name: "dynamics (curve)", Abbr: "dyn", Type: "Curve", Min: -240, Max: 120, DefaultValue: 0},
		"pitd": {Name: "pitch deviation (curve)", Abbr: "pitd", Type: "Curve", Min: -1200, Max: 1200, DefaultValue: 0},
		"clr":  {Name: "voice color", Abbr: "clr", Type: "Options", Min: 0, Max: -1, DefaultValue: 0, Options: []string{}},
		"eng":  {Name: "resampler engine", Abbr: "eng", Type: "Options", Min: 0, Max: 1, DefaultValue: 0, Options: []string{"", "worldline"}},
		"vel":  {Name: "velocity", Abbr: "vel", Type: "Numerical", Min: 0, Max: 200, DefaultValue: 100},
		"vol":  {Name: "volume", Abbr: "vol", Type: "Numerical", Min: 0, Max: 200, DefaultValue: 100},
		"atk":  {Name: "attack", Abbr: "atk", Type: "Numerical", Min: 0, Max: 200, DefaultValue: 100},
		"dec":  {Name: "decay", Abbr: "dec", Type: "Numerical", Min: 0, Max: 100, DefaultValue: 0},
		"gen":  {Name: "gender", Abbr: "gen", Type: "Numerical", Min: -100, Max: 100, DefaultValue: 0, IsFlag: true, Flag: "g"},
		"genc": {Name: "gender (curve)", Abbr: "genc", Type: "Curve", Min: -100, Max: 100, DefaultValue: 0},
		"bre":  {Name: "breath", Abbr: "bre", Type: "Numerical", Min: 0, Max: 100, DefaultValue: 0, IsFlag: true, Flag: "B"},
		"brec": {Name: "breathiness (curve)", Abbr: "brec", Type: "Curve", Min: -100, Max: 100, DefaultValue: 0},
		"lpf":  {Name: "lowpass", Abbr: "lpf", Type: "Numerical", Min: 0, Max: 100, DefaultValue: 0, IsFlag: true, Flag: "H"},
		"mod":  {Name: "modulation", Abbr: "mod", Type: "Numerical", Min: 0, Max: 100, DefaultValue: 0},
		"alt":  {Name: "alternate", Abbr: "alt", Type: "Numerical", Min: 0, Max: 16, DefaultValue: 0},
		"shft": {Name: "tone shift", Abbr: "shft", Type: "Numerical", Min: -36, Max: 36, DefaultValue: 0},
		"shfc": {Name: "tone shift (curve)", Abbr: "shfc", Type: "Curve", Min: -1200, Max: 1200, DefaultValue: 0},
		"tenc": {Name: "tension (curve)", Abbr: "tenc", Type: "Curve", Min: -100, Max: 100, DefaultValue: 0},
		"voic": {Name: "voicing (curve)", Abbr: "voic", Type: "Curve", Min: 0, Max: 100, DefaultValue: 100},
	}
}
