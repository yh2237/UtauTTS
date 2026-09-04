package diffsinger

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SingerKind = "diffsinger"
	HeadFrames = 8
	TailFrames = 8
)

type Config struct {
	Phonemes                  string   `yaml:"phonemes"`
	Languages                 string   `yaml:"languages"`
	Acoustic                  string   `yaml:"acoustic"`
	Vocoder                   string   `yaml:"vocoder"`
	Speakers                  []string `yaml:"speakers"`
	HiddenSize                int      `yaml:"hidden_size"`
	UseKeyShiftEmbed          bool     `yaml:"use_key_shift_embed"`
	UseSpeedEmbed             bool     `yaml:"use_speed_embed"`
	UseEnergyEmbed            bool     `yaml:"use_energy_embed"`
	UseBreathinessEmbed       bool     `yaml:"use_breathiness_embed"`
	UseVoicingEmbed           bool     `yaml:"use_voicing_embed"`
	UseTensionEmbed           bool     `yaml:"use_tension_embed"`
	UseContinuousAcceleration bool     `yaml:"use_continuous_acceleration"`
	UseLangID                 bool     `yaml:"use_lang_id"`
	UseShallowDiffusion       *bool    `yaml:"use_shallow_diffusion"`
	UseVariableDepth          *bool    `yaml:"use_variable_depth"`
	MaxDepth                  float64  `yaml:"max_depth"`
	SampleRate                int      `yaml:"sample_rate"`
	HopSize                   int      `yaml:"hop_size"`
	WinSize                   int      `yaml:"win_size"`
	FFTSize                   int      `yaml:"fft_size"`
	NumMelBins                int      `yaml:"num_mel_bins"`
	MelFMin                   float64  `yaml:"mel_fmin"`
	MelFMax                   float64  `yaml:"mel_fmax"`
	MelBase                   string   `yaml:"mel_base"`
	MelScale                  string   `yaml:"mel_scale"`
}

type DurationConfig struct {
	Phonemes   string   `yaml:"phonemes"`
	Languages  string   `yaml:"languages"`
	Linguistic string   `yaml:"linguistic"`
	Dur        string   `yaml:"dur"`
	Speakers   []string `yaml:"speakers"`
	HiddenSize int      `yaml:"hidden_size"`
	UseLangID  bool     `yaml:"use_lang_id"`
	PredictDur *bool    `yaml:"predict_dur"`
	SampleRate int      `yaml:"sample_rate"`
	HopSize    int      `yaml:"hop_size"`
}

type DurationModel struct {
	Config         DurationConfig
	Tokens         map[string]int64
	LanguageIDs    map[string]int64
	LinguisticPath string
	PredictorPath  string
	SpeakerEmbed   []float32
}

type PitchConfig struct {
	Phonemes                  string   `yaml:"phonemes"`
	Languages                 string   `yaml:"languages"`
	Linguistic                string   `yaml:"linguistic"`
	Pitch                     string   `yaml:"pitch"`
	Speakers                  []string `yaml:"speakers"`
	HiddenSize                int      `yaml:"hidden_size"`
	UseLangID                 bool     `yaml:"use_lang_id"`
	PredictDur                *bool    `yaml:"predict_dur"`
	UseContinuousAcceleration bool     `yaml:"use_continuous_acceleration"`
	UseExpr                   bool     `yaml:"use_expr"`
	UseNoteRest               bool     `yaml:"use_note_rest"`
	SampleRate                int      `yaml:"sample_rate"`
	HopSize                   int      `yaml:"hop_size"`
}

type PitchModel struct {
	Config         PitchConfig
	Tokens         map[string]int64
	LanguageIDs    map[string]int64
	LinguisticPath string
	PredictorPath  string
	SpeakerEmbed   []float32
}

type VarianceConfig struct {
	Phonemes                  string   `yaml:"phonemes"`
	Languages                 string   `yaml:"languages"`
	Linguistic                string   `yaml:"linguistic"`
	Variance                  string   `yaml:"variance"`
	Speakers                  []string `yaml:"speakers"`
	HiddenSize                int      `yaml:"hidden_size"`
	UseLangID                 bool     `yaml:"use_lang_id"`
	PredictDur                bool     `yaml:"predict_dur"`
	PredictEnergy             bool     `yaml:"predict_energy"`
	PredictBreathiness        bool     `yaml:"predict_breathiness"`
	PredictVoicing            bool     `yaml:"predict_voicing"`
	PredictTension            bool     `yaml:"predict_tension"`
	UseContinuousAcceleration bool     `yaml:"use_continuous_acceleration"`
	SampleRate                int      `yaml:"sample_rate"`
	HopSize                   int      `yaml:"hop_size"`
}

type VarianceModel struct {
	Config         VarianceConfig
	Tokens         map[string]int64
	LanguageIDs    map[string]int64
	LinguisticPath string
	PredictorPath  string
	SpeakerEmbed   []float32
}

type VocoderConfig struct {
	Model             string  `yaml:"model"`
	SampleRate        int     `yaml:"sample_rate"`
	HopSize           int     `yaml:"hop_size"`
	WinSize           int     `yaml:"win_size"`
	FFTSize           int     `yaml:"fft_size"`
	NumMelBins        int     `yaml:"num_mel_bins"`
	MelFMin           float64 `yaml:"mel_fmin"`
	MelFMax           float64 `yaml:"mel_fmax"`
	MelBase           string  `yaml:"mel_base"`
	MelScale          string  `yaml:"mel_scale"`
	PitchControllable bool    `yaml:"pitch_controllable"`
}

type Singer struct {
	Root               string
	Config             Config
	Vocoder            VocoderConfig
	Tokens             map[string]int64
	LanguageIDs        map[string]int64
	AcousticPath       string
	VocoderPath        string
	SpeakerEmbed       []float32
	JapaneseDictionary map[string][]string
	Duration           *DurationModel
	Pitch              *PitchModel
	Variance           *VarianceModel
}

func IsSinger(root string) bool {
	info, err := os.Stat(filepath.Join(root, "dsconfig.yaml"))
	return err == nil && !info.IsDir()
}

func Load(root string) (*Singer, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := readYAML(filepath.Join(absRoot, "dsconfig.yaml"), &cfg); err != nil {
		return nil, fmt.Errorf("read dsconfig.yaml: %w", err)
	}
	applyConfigDefaults(&cfg)
	packageRoot := filepath.Dir(absRoot)
	if strings.TrimSpace(cfg.Acoustic) == "" {
		return nil, fmt.Errorf("dsconfig.yaml has no acoustic model")
	}
	if strings.TrimSpace(cfg.Phonemes) == "" {
		return nil, fmt.Errorf("dsconfig.yaml has no phonemes file")
	}
	acousticPath, err := scopedFile(packageRoot, absRoot, cfg.Acoustic)
	if err != nil {
		return nil, fmt.Errorf("acoustic model: %w", err)
	}
	phonemesPath, err := scopedFile(packageRoot, absRoot, cfg.Phonemes)
	if err != nil {
		return nil, fmt.Errorf("phonemes: %w", err)
	}
	tokens, err := loadTokens(phonemesPath)
	if err != nil {
		return nil, err
	}
	japaneseDictionary, err := loadJapaneseDictionary(absRoot, tokens)
	if err != nil {
		return nil, err
	}
	vocoderRoot, localVocoder, err := findVocoderRoot(absRoot, cfg.Vocoder)
	if err != nil {
		return nil, err
	}
	var vocoder VocoderConfig
	if err := readYAML(filepath.Join(vocoderRoot, "vocoder.yaml"), &vocoder); err != nil {
		return nil, fmt.Errorf("read vocoder config: %w", err)
	}
	applyVocoderDefaults(&vocoder, cfg)
	if vocoder.Model == "" {
		vocoder.Model = "model.onnx"
	}
	vocoderScope := vocoderRoot
	if localVocoder {
		vocoderScope = packageRoot
	}
	vocoderPath, err := scopedFile(vocoderScope, vocoderRoot, vocoder.Model)
	if err != nil {
		return nil, fmt.Errorf("vocoder model: %w", err)
	}
	if vocoder.SampleRate != cfg.SampleRate || vocoder.HopSize != cfg.HopSize || vocoder.NumMelBins != cfg.NumMelBins {
		return nil, fmt.Errorf("acoustic model and vocoder settings do not match")
	}
	if vocoder.WinSize != cfg.WinSize || vocoder.FFTSize != cfg.FFTSize || vocoder.MelFMin != cfg.MelFMin || vocoder.MelFMax != cfg.MelFMax || vocoder.MelScale != cfg.MelScale {
		return nil, fmt.Errorf("acoustic model and vocoder mel settings do not match")
	}
	if (cfg.MelBase != "10" && cfg.MelBase != "e") || (vocoder.MelBase != "10" && vocoder.MelBase != "e") {
		return nil, fmt.Errorf("mel_base must be 10 or e")
	}
	languageIDs := map[string]int64{}
	if cfg.UseLangID {
		languagesPath, pathErr := scopedFile(packageRoot, absRoot, cfg.Languages)
		if pathErr != nil {
			return nil, fmt.Errorf("languages: %w", pathErr)
		}
		data, readErr := os.ReadFile(languagesPath)
		if readErr != nil {
			return nil, readErr
		}
		if jsonErr := json.Unmarshal(data, &languageIDs); jsonErr != nil {
			return nil, fmt.Errorf("read language IDs: %w", jsonErr)
		}
	}
	var speakerEmbed []float32
	if len(cfg.Speakers) > 0 {
		speakerPath, pathErr := scopedFile(packageRoot, absRoot, cfg.Speakers[0]+".emb")
		if pathErr != nil {
			return nil, fmt.Errorf("speaker embedding: %w", pathErr)
		}
		speakerEmbed, err = loadFloat32(speakerPath, cfg.HiddenSize)
		if err != nil {
			return nil, fmt.Errorf("speaker embedding: %w", err)
		}
	}
	duration, err := loadDurationModel(packageRoot, absRoot)
	if err != nil {
		return nil, err
	}
	if duration != nil && (duration.Config.SampleRate != cfg.SampleRate || duration.Config.HopSize != cfg.HopSize) {
		return nil, fmt.Errorf("acoustic model and duration model frame settings do not match")
	}
	pitch, err := loadPitchModel(packageRoot, absRoot)
	if err != nil {
		return nil, err
	}
	if pitch != nil && (pitch.Config.SampleRate != cfg.SampleRate || pitch.Config.HopSize != cfg.HopSize) {
		return nil, fmt.Errorf("acoustic model and pitch model frame settings do not match")
	}
	variance, err := loadVarianceModel(packageRoot, absRoot)
	if err != nil {
		return nil, err
	}
	if variance != nil && (variance.Config.SampleRate != cfg.SampleRate || variance.Config.HopSize != cfg.HopSize) {
		return nil, fmt.Errorf("acoustic model and variance model frame settings do not match")
	}
	return &Singer{Root: absRoot, Config: cfg, Vocoder: vocoder, Tokens: tokens, LanguageIDs: languageIDs, AcousticPath: acousticPath, VocoderPath: vocoderPath, SpeakerEmbed: speakerEmbed, JapaneseDictionary: japaneseDictionary, Duration: duration, Pitch: pitch, Variance: variance}, nil
}

func (s *Singer) Token(symbol string) (int64, error) {
	token, ok := s.Tokens[symbol]
	if !ok {
		return 0, fmt.Errorf("phoneme %q is not in %s", symbol, s.Config.Phonemes)
	}
	return token, nil
}

func (s *Singer) FrameMS() float64 {
	return 1000 * float64(s.Config.HopSize) / float64(s.Config.SampleRate)
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

func scopedFile(scope, base, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("absolute path is not allowed: %s", relative)
	}
	candidate := filepath.Clean(filepath.Join(base, filepath.FromSlash(relative)))
	rel, err := filepath.Rel(filepath.Clean(scope), candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes singer package: %s", relative)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("expected a file: %s", candidate)
	}
	return candidate, nil
}

func findVocoderRoot(singerRoot, name string) (string, bool, error) {
	local := filepath.Join(singerRoot, "dsvocoder")
	if info, err := os.Stat(filepath.Join(local, "vocoder.yaml")); err == nil && !info.IsDir() {
		return local, true, nil
	}
	name = strings.TrimSpace(name)
	if name == "" || filepath.IsAbs(name) || filepath.Base(name) != name {
		return "", false, fmt.Errorf("DiffSinger vocoder dependency name is invalid: %q", name)
	}
	var roots []string
	add := func(path string) {
		path = filepath.Clean(path)
		for _, existing := range roots {
			if strings.EqualFold(existing, path) {
				return
			}
		}
		roots = append(roots, path)
	}
	add(filepath.Join(singerRoot, "Dependencies"))
	add(filepath.Join(filepath.Dir(singerRoot), "Dependencies"))
	if current, err := os.Getwd(); err == nil {
		add(filepath.Join(current, "Dependencies"))
	}
	if executable, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(executable), "Dependencies"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case "windows":
			add(filepath.Join(home, "Documents", "OpenUtau", "Dependencies"))
		case "darwin":
			add(filepath.Join(home, "Library", "OpenUtau", "Dependencies"))
		default:
			dataHome := os.Getenv("XDG_DATA_HOME")
			if dataHome == "" {
				dataHome = filepath.Join(home, ".local", "share")
			}
			add(filepath.Join(dataHome, "OpenUtau", "Dependencies"))
		}
	}
	searched := make([]string, 0, len(roots))
	for _, root := range roots {
		candidate := filepath.Join(root, name)
		searched = append(searched, candidate)
		if info, err := os.Stat(filepath.Join(candidate, "vocoder.yaml")); err == nil && !info.IsDir() {
			return candidate, false, nil
		}
	}
	return "", false, fmt.Errorf("DiffSinger vocoder dependency %q was not found in %s", name, strings.Join(searched, ", "))
}

func loadFloat32(path string, count int) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != count*4 {
		return nil, fmt.Errorf("%s has %d bytes; expected %d", path, len(data), count*4)
	}
	result := make([]float32, count)
	for index := range result {
		result[index] = math.Float32frombits(binary.LittleEndian.Uint32(data[index*4:]))
	}
	return result, nil
}

func loadDurationModel(packageRoot, singerRoot string) (*DurationModel, error) {
	root := filepath.Join(singerRoot, "dsdur")
	configPath := filepath.Join(root, "dsconfig.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	var cfg DurationConfig
	if err := readYAML(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("read dsdur/dsconfig.yaml: %w", err)
	}
	if cfg.Phonemes == "" {
		cfg.Phonemes = "phonemes.txt"
	}
	if cfg.HiddenSize == 0 {
		cfg.HiddenSize = 256
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.HopSize == 0 {
		cfg.HopSize = 512
	}
	if cfg.PredictDur != nil && !*cfg.PredictDur {
		return nil, nil
	}
	linguisticPath, err := scopedFile(packageRoot, root, cfg.Linguistic)
	if err != nil {
		return nil, fmt.Errorf("duration linguistic model: %w", err)
	}
	predictorPath, err := scopedFile(packageRoot, root, cfg.Dur)
	if err != nil {
		return nil, fmt.Errorf("duration predictor model: %w", err)
	}
	phonemesPath, err := scopedFile(packageRoot, root, cfg.Phonemes)
	if err != nil {
		return nil, fmt.Errorf("duration phonemes: %w", err)
	}
	tokens, err := loadTokens(phonemesPath)
	if err != nil {
		return nil, fmt.Errorf("duration model: %w", err)
	}
	languageIDs := map[string]int64{}
	if cfg.UseLangID {
		languagesPath, pathErr := scopedFile(packageRoot, root, cfg.Languages)
		if pathErr != nil {
			return nil, fmt.Errorf("duration languages: %w", pathErr)
		}
		data, readErr := os.ReadFile(languagesPath)
		if readErr != nil {
			return nil, fmt.Errorf("duration languages: %w", readErr)
		}
		if jsonErr := json.Unmarshal(data, &languageIDs); jsonErr != nil {
			return nil, fmt.Errorf("read duration language IDs: %w", jsonErr)
		}
	}
	var speakerEmbed []float32
	if len(cfg.Speakers) > 0 {
		speakerPath, pathErr := scopedFile(packageRoot, root, cfg.Speakers[0]+".emb")
		if pathErr != nil {
			return nil, fmt.Errorf("duration speaker embedding: %w", pathErr)
		}
		speakerEmbed, err = loadFloat32(speakerPath, cfg.HiddenSize)
		if err != nil {
			return nil, fmt.Errorf("duration speaker embedding: %w", err)
		}
	}
	return &DurationModel{Config: cfg, Tokens: tokens, LanguageIDs: languageIDs, LinguisticPath: linguisticPath, PredictorPath: predictorPath, SpeakerEmbed: speakerEmbed}, nil
}

func loadPitchModel(packageRoot, singerRoot string) (*PitchModel, error) {
	root := filepath.Join(singerRoot, "dspitch")
	configPath := filepath.Join(root, "dsconfig.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	var cfg PitchConfig
	if err := readYAML(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("read dspitch/dsconfig.yaml: %w", err)
	}
	if cfg.Phonemes == "" {
		cfg.Phonemes = "phonemes.txt"
	}
	if cfg.HiddenSize == 0 {
		cfg.HiddenSize = 256
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.HopSize == 0 {
		cfg.HopSize = 512
	}
	linguisticPath, err := scopedFile(packageRoot, root, cfg.Linguistic)
	if err != nil {
		return nil, fmt.Errorf("pitch linguistic model: %w", err)
	}
	predictorPath, err := scopedFile(packageRoot, root, cfg.Pitch)
	if err != nil {
		return nil, fmt.Errorf("pitch predictor model: %w", err)
	}
	phonemesPath, err := scopedFile(packageRoot, root, cfg.Phonemes)
	if err != nil {
		return nil, fmt.Errorf("pitch phonemes: %w", err)
	}
	tokens, err := loadTokens(phonemesPath)
	if err != nil {
		return nil, fmt.Errorf("pitch model: %w", err)
	}
	languageIDs := map[string]int64{}
	if cfg.UseLangID {
		languagesPath, pathErr := scopedFile(packageRoot, root, cfg.Languages)
		if pathErr != nil {
			return nil, fmt.Errorf("pitch languages: %w", pathErr)
		}
		data, readErr := os.ReadFile(languagesPath)
		if readErr != nil {
			return nil, fmt.Errorf("pitch languages: %w", readErr)
		}
		if jsonErr := json.Unmarshal(data, &languageIDs); jsonErr != nil {
			return nil, fmt.Errorf("read pitch language IDs: %w", jsonErr)
		}
	}
	var speakerEmbed []float32
	if len(cfg.Speakers) > 0 {
		speakerPath, pathErr := scopedFile(packageRoot, root, cfg.Speakers[0]+".emb")
		if pathErr != nil {
			return nil, fmt.Errorf("pitch speaker embedding: %w", pathErr)
		}
		speakerEmbed, err = loadFloat32(speakerPath, cfg.HiddenSize)
		if err != nil {
			return nil, fmt.Errorf("pitch speaker embedding: %w", err)
		}
	}
	return &PitchModel{Config: cfg, Tokens: tokens, LanguageIDs: languageIDs, LinguisticPath: linguisticPath, PredictorPath: predictorPath, SpeakerEmbed: speakerEmbed}, nil
}

func loadVarianceModel(packageRoot, singerRoot string) (*VarianceModel, error) {
	root := filepath.Join(singerRoot, "dsvariance")
	configPath := filepath.Join(root, "dsconfig.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	var cfg VarianceConfig
	if err := readYAML(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("read dsvariance/dsconfig.yaml: %w", err)
	}
	if cfg.Phonemes == "" {
		cfg.Phonemes = "phonemes.txt"
	}
	if cfg.HiddenSize == 0 {
		cfg.HiddenSize = 256
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.HopSize == 0 {
		cfg.HopSize = 512
	}
	linguisticPath, err := scopedFile(packageRoot, root, cfg.Linguistic)
	if err != nil {
		return nil, fmt.Errorf("variance linguistic model: %w", err)
	}
	predictorPath, err := scopedFile(packageRoot, root, cfg.Variance)
	if err != nil {
		return nil, fmt.Errorf("variance predictor model: %w", err)
	}
	phonemesPath, err := scopedFile(packageRoot, root, cfg.Phonemes)
	if err != nil {
		return nil, fmt.Errorf("variance phonemes: %w", err)
	}
	tokens, err := loadTokens(phonemesPath)
	if err != nil {
		return nil, fmt.Errorf("variance model: %w", err)
	}
	languageIDs := map[string]int64{}
	if cfg.UseLangID {
		languagesPath, pathErr := scopedFile(packageRoot, root, cfg.Languages)
		if pathErr != nil {
			return nil, fmt.Errorf("variance languages: %w", pathErr)
		}
		data, readErr := os.ReadFile(languagesPath)
		if readErr != nil {
			return nil, fmt.Errorf("variance languages: %w", readErr)
		}
		if jsonErr := json.Unmarshal(data, &languageIDs); jsonErr != nil {
			return nil, fmt.Errorf("read variance language IDs: %w", jsonErr)
		}
	}
	var speakerEmbed []float32
	if len(cfg.Speakers) > 0 {
		speakerPath, pathErr := scopedFile(packageRoot, root, cfg.Speakers[0]+".emb")
		if pathErr != nil {
			return nil, fmt.Errorf("variance speaker embedding: %w", pathErr)
		}
		speakerEmbed, err = loadFloat32(speakerPath, cfg.HiddenSize)
		if err != nil {
			return nil, fmt.Errorf("variance speaker embedding: %w", err)
		}
	}
	return &VarianceModel{Config: cfg, Tokens: tokens, LanguageIDs: languageIDs, LinguisticPath: linguisticPath, PredictorPath: predictorPath, SpeakerEmbed: speakerEmbed}, nil
}

func loadTokens(path string) (map[string]int64, error) {

	if strings.EqualFold(filepath.Ext(path), ".json") {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read phonemes: %w", err)
		}
		var tokens map[string]int64
		if err := json.Unmarshal(data, &tokens); err != nil {
			if retryErr := json.Unmarshal(removeTrailingJSONCommas(data), &tokens); retryErr != nil {
				return nil, fmt.Errorf("read phonemes: %w", err)
			}
		}
		if _, ok := tokens["SP"]; !ok {
			return nil, fmt.Errorf("phonemes file has no SP token: %s", path)
		}
		return tokens, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read phonemes: %w", err)
	}
	defer file.Close()
	tokens := map[string]int64{}
	scanner := bufio.NewScanner(file)
	line := int64(0)
	for scanner.Scan() {
		symbol := strings.TrimSpace(scanner.Text())
		if symbol == "" {
			line++
			continue
		}
		if _, exists := tokens[symbol]; exists {
			return nil, fmt.Errorf("duplicate phoneme %q in %s", symbol, path)
		}
		tokens[symbol] = line
		line++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read phonemes: %w", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("phonemes file is empty: %s", path)
	}
	if _, ok := tokens["SP"]; !ok {
		return nil, fmt.Errorf("phonemes file has no SP token: %s", path)
	}
	return tokens, nil
}

func removeTrailingJSONCommas(data []byte) []byte {
	result := make([]byte, 0, len(data))
	inString := false
	escaped := false
	for index, value := range data {
		if inString {
			result = append(result, value)
			if escaped {
				escaped = false
			} else if value == '\\' {
				escaped = true
			} else if value == '"' {
				inString = false
			}
			continue
		}
		if value == '"' {
			inString = true
			result = append(result, value)
			continue
		}
		if value == ',' {
			next := index + 1
			for next < len(data) && (data[next] == ' ' || data[next] == '\t' || data[next] == '\r' || data[next] == '\n') {
				next++
			}
			if next < len(data) && (data[next] == '}' || data[next] == ']') {
				continue
			}
		}
		result = append(result, value)
	}
	return result
}

type dictionaryFile struct {
	Entries []dictionaryEntry `yaml:"entries"`
}

type dictionaryEntry struct {
	Grapheme string   `yaml:"grapheme"`
	Phonemes []string `yaml:"phonemes"`
}

func loadJapaneseDictionary(root string, tokens map[string]int64) (map[string][]string, error) {
	candidates := []string{
		filepath.Join(root, "dsdur", "dsdict-ja.yaml"),
		filepath.Join(root, "dsdict-ja.yaml"),
		filepath.Join(root, "dsdur", "dsdict.yaml"),
	}
	for _, path := range candidates {
		var dictionary dictionaryFile
		if err := readYAML(path, &dictionary); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read Japanese DiffSinger dictionary: %w", err)
		}
		result := make(map[string][]string)
		for _, entry := range dictionary.Entries {
			if entry.Grapheme == "" || len(entry.Phonemes) == 0 {
				continue
			}
			valid := true
			for _, symbol := range entry.Phonemes {
				if _, ok := tokens[symbol]; !ok {
					valid = false
					break
				}
			}
			if valid {
				result[entry.Grapheme] = append([]string(nil), entry.Phonemes...)
			}
		}
		return result, nil
	}
	return nil, nil
}

func applyConfigDefaults(cfg *Config) {
	if cfg.Phonemes == "" {
		cfg.Phonemes = "phonemes.txt"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.HiddenSize == 0 {
		cfg.HiddenSize = 256
	}
	if cfg.HopSize == 0 {
		cfg.HopSize = 512
	}
	if cfg.WinSize == 0 {
		cfg.WinSize = 2048
	}
	if cfg.FFTSize == 0 {
		cfg.FFTSize = 2048
	}
	if cfg.NumMelBins == 0 {
		cfg.NumMelBins = 128
	}
	if cfg.MelFMin == 0 {
		cfg.MelFMin = 40
	}
	if cfg.MelFMax == 0 {
		cfg.MelFMax = 16000
	}
	if cfg.MelBase == "" {
		cfg.MelBase = "10"
	}
	if cfg.MelScale == "" {
		cfg.MelScale = "slaney"
	}
}

func applyVocoderDefaults(vocoder *VocoderConfig, acoustic Config) {
	if vocoder.SampleRate == 0 {
		vocoder.SampleRate = acoustic.SampleRate
	}
	if vocoder.HopSize == 0 {
		vocoder.HopSize = acoustic.HopSize
	}
	if vocoder.WinSize == 0 {
		vocoder.WinSize = acoustic.WinSize
	}
	if vocoder.FFTSize == 0 {
		vocoder.FFTSize = acoustic.FFTSize
	}
	if vocoder.NumMelBins == 0 {
		vocoder.NumMelBins = acoustic.NumMelBins
	}
	if vocoder.MelFMin == 0 {
		vocoder.MelFMin = acoustic.MelFMin
	}
	if vocoder.MelFMax == 0 {
		vocoder.MelFMax = acoustic.MelFMax
	}
	if vocoder.MelBase == "" {
		vocoder.MelBase = acoustic.MelBase
	}
	if vocoder.MelScale == "" {
		vocoder.MelScale = acoustic.MelScale
	}
}
