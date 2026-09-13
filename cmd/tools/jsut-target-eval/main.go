// jsut-target-evalは発話単位のホールドアウトで継続時間を評価する
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/jsut"
)

type evaluationReport struct {
	Version             int              `json:"version"`
	Kind                string           `json:"kind"`
	SplitMethod         string           `json:"split_method"`
	TestPercent         int              `json:"test_percent"`
	MinContextCount     int              `json:"min_context_count"`
	InputRecords        int              `json:"input_records"`
	TrainUtterances     int              `json:"train_utterances"`
	TestUtterances      int              `json:"test_utterances"`
	TrainPhones         int              `json:"train_phones"`
	TestPhones          int              `json:"test_phones"`
	UnseenPhones        int              `json:"unseen_phones"`
	EligibleTestPhones  int              `json:"eligible_test_phones"`
	ContextUsed         int              `json:"context_used"`
	ContextFallback     int              `json:"context_fallback"`
	GlobalBaseline      metricReport     `json:"global_baseline"`
	PhonePrior          metricReport     `json:"phone_prior"`
	ContextBackoffPrior metricReport     `json:"context_backoff_prior"`
	ContextMAEGainMS    float64          `json:"context_mae_gain_ms"`
	Allocation          allocationReport `json:"mora_phone_allocation"`
	TrainBoundaries     boundaryReport   `json:"train_boundaries"`
	TestBoundaries      boundaryReport   `json:"test_boundaries"`
	Notes               []string         `json:"notes"`
}

type metricReport struct {
	Count int     `json:"count"`
	MAE   float64 `json:"mae_ms"`
	RMSE  float64 `json:"rmse_ms"`
}

type boundaryReport struct {
	NaturalBoundaries int `json:"natural_boundaries"`
	WithFeatures      int `json:"with_features"`
}

// allocationReportはモーラ内の音素配分を比較する。モーラ長の誤差と切り分ける。
type allocationReport struct {
	MoraGroups             int                              `json:"mora_groups"`
	IncompleteGroups       int                              `json:"incomplete_groups"`
	Phones                 int                              `json:"phones"`
	CurrentPhoneSpans      metricReport                     `json:"current_phone_spans"`
	PhonePriorNormalized   metricReport                     `json:"phone_prior_normalized"`
	ContextPriorNormalized metricReport                     `json:"context_prior_normalized"`
	ContextMAEGainMS       float64                          `json:"context_mae_gain_ms"`
	CurrentByRole          map[string]metricReport          `json:"current_by_role,omitempty"`
	ContextByRole          map[string]metricReport          `json:"context_by_role,omitempty"`
	ByPhone                map[string]allocationPhoneReport `json:"by_phone,omitempty"`
}

type allocationPhoneReport struct {
	Count          int          `json:"count"`
	Current        metricReport `json:"current_phone_spans"`
	Context        metricReport `json:"context_prior_normalized"`
	ContextMAEGain float64      `json:"context_mae_gain_ms"`
}

type allocationAccumulator struct {
	current durationAccumulator
	phone   durationAccumulator
	context durationAccumulator
	byRole  map[string]*roleAllocationAccumulator
	byPhone map[string]*phoneAllocationAccumulator
}

type roleAllocationAccumulator struct {
	current durationAccumulator
	context durationAccumulator
}

type phoneAllocationAccumulator struct {
	current durationAccumulator
	context durationAccumulator
}

func (acc *allocationAccumulator) add(group []jsut.Phone, actualTotal, globalMean float64, prior *jsut.Prior, minContextCount int) {
	if len(group) == 0 || actualTotal <= 0 || !finite(actualTotal) {
		return
	}
	frontendPhones := make([]frontend.Phone, len(group))
	for index, phone := range group {
		role := "onset"
		if index == len(group)-1 {
			role = "nucleus"
		}
		frontendPhones[index] = frontend.Phone{Symbol: phone.Symbol, Role: role}
	}
	current := frontend.PhoneSpans(frontendPhones, actualTotal)
	phonePrior := make([]float64, len(group))
	contextPrior := make([]float64, len(group))
	for index, phone := range group {
		phoneMean := globalMean
		if stats, ok := prior.Phones[phone.Symbol]; ok && stats.DurationMS.Count > 0 && stats.DurationMS.Mean > 0 {
			phoneMean = stats.DurationMS.Mean
		}
		phonePrior[index] = phoneMean
		contextPrior[index] = phoneMean
		key := phone.Previous + "|" + phone.Symbol + "|" + phone.Next
		if stats, ok := prior.Contexts[key]; ok && stats.Count >= minContextCount && stats.DurationMS.Count > 0 && stats.DurationMS.Mean > 0 {
			contextPrior[index] = stats.DurationMS.Mean
		}
	}
	normalizeDurations(phonePrior, actualTotal)
	normalizeDurations(contextPrior, actualTotal)
	acc.byRole = ensureRoleMap(acc.byRole)
	acc.byPhone = ensurePhoneMap(acc.byPhone)
	for index, phone := range group {
		acc.current.add(phone.DurationMS, current[index])
		acc.phone.add(phone.DurationMS, phonePrior[index])
		acc.context.add(phone.DurationMS, contextPrior[index])
		role := frontendPhones[index].Role
		roleAcc := acc.byRole[role]
		if roleAcc == nil {
			roleAcc = &roleAllocationAccumulator{}
			acc.byRole[role] = roleAcc
		}
		roleAcc.current.add(phone.DurationMS, current[index])
		roleAcc.context.add(phone.DurationMS, contextPrior[index])
		phoneAcc := acc.byPhone[phone.Symbol]
		if phoneAcc == nil {
			phoneAcc = &phoneAllocationAccumulator{}
			acc.byPhone[phone.Symbol] = phoneAcc
		}
		phoneAcc.current.add(phone.DurationMS, current[index])
		phoneAcc.context.add(phone.DurationMS, contextPrior[index])
	}
}

func ensureRoleMap(value map[string]*roleAllocationAccumulator) map[string]*roleAllocationAccumulator {
	if value == nil {
		return make(map[string]*roleAllocationAccumulator)
	}
	return value
}

func ensurePhoneMap(value map[string]*phoneAllocationAccumulator) map[string]*phoneAllocationAccumulator {
	if value == nil {
		return make(map[string]*phoneAllocationAccumulator)
	}
	return value
}

func normalizeDurations(values []float64, total float64) {
	sum := 0.0
	for _, value := range values {
		if finite(value) && value > 0 {
			sum += value
		}
	}
	if sum <= 0 || !finite(sum) || total <= 0 || !finite(total) {
		return
	}
	for index, value := range values {
		if !finite(value) || value < 0 {
			values[index] = 0
			continue
		}
		values[index] = value * total / sum
	}
}

func (acc allocationAccumulator) report(groups, incomplete int) allocationReport {
	result := allocationReport{
		MoraGroups: groups, IncompleteGroups: incomplete,
		CurrentPhoneSpans:      acc.current.report(),
		PhonePriorNormalized:   acc.phone.report(),
		ContextPriorNormalized: acc.context.report(),
		CurrentByRole:          make(map[string]metricReport),
		ContextByRole:          make(map[string]metricReport),
		ByPhone:                make(map[string]allocationPhoneReport),
	}
	result.Phones = result.CurrentPhoneSpans.Count
	result.ContextMAEGainMS = result.PhonePriorNormalized.MAE - result.ContextPriorNormalized.MAE
	for role, roleAcc := range acc.byRole {
		result.CurrentByRole[role] = roleAcc.current.report()
		result.ContextByRole[role] = roleAcc.context.report()
	}
	for symbol, phoneAcc := range acc.byPhone {
		current := phoneAcc.current.report()
		context := phoneAcc.context.report()
		result.ByPhone[symbol] = allocationPhoneReport{
			Count: current.Count, Current: current, Context: context,
			ContextMAEGain: phoneAcc.current.report().MAE - context.MAE,
		}
	}
	return result
}

// groupJapanesePhonesはJSUTの音素列をモーラへまとめる。無音・休止で区切り、不完全な末尾を除外する。
func groupJapanesePhones(phones []jsut.Phone) (groups [][]jsut.Phone, incomplete int) {
	var current []jsut.Phone
	flush := func() {
		if len(current) == 0 {
			return
		}
		if isJapaneseVowel(current[len(current)-1].Symbol) {
			groups = append(groups, current)
		} else {
			incomplete++
		}
		current = nil
	}
	for _, phone := range phones {
		if phone.Silence || phone.Pause {
			flush()
			continue
		}
		if phone.Symbol == "N" || phone.Symbol == "cl" {
			flush()
			groups = append(groups, []jsut.Phone{phone})
			continue
		}
		current = append(current, phone)
		if isJapaneseVowel(phone.Symbol) {
			flush()
		}
	}
	flush()
	return groups, incomplete
}

func isJapaneseVowel(symbol string) bool {
	switch strings.ToLower(symbol) {
	case "a", "i", "u", "e", "o":
		return true
	default:
		return false
	}
}

type durationAccumulator struct {
	count  int
	abs    float64
	square float64
}

func (acc *durationAccumulator) add(actual, predicted float64) {
	if !finite(actual) || !finite(predicted) {
		return
	}
	error := predicted - actual
	acc.count++
	acc.abs += math.Abs(error)
	acc.square += error * error
}

func (acc durationAccumulator) report() metricReport {
	if acc.count == 0 {
		return metricReport{}
	}
	return metricReport{Count: acc.count, MAE: acc.abs / float64(acc.count), RMSE: math.Sqrt(acc.square / float64(acc.count))}
}

type scalarAccumulator struct {
	total float64
	count int
}

func (acc *scalarAccumulator) add(value float64) {
	if finite(value) {
		acc.total += value
		acc.count++
	}
}

func (acc scalarAccumulator) mean(fallback float64) float64 {
	if acc.count == 0 {
		return fallback
	}
	return acc.total / float64(acc.count)
}

func main() {
	var inputs inputPaths
	var (
		outPath         string
		testPercent     int
		minContextCount int
		force           bool
	)
	flag.Var(&inputs, "input", "alignment JSONL path (repeatable)")
	flag.StringVar(&outPath, "out", "", "evaluation JSON path, or - for stdout")
	flag.IntVar(&testPercent, "test-percent", 20, "percentage of utterances reserved for the holdout")
	flag.IntVar(&minContextCount, "min-context-count", 5, "minimum training examples before using a context prior")
	flag.BoolVar(&force, "force", false, "overwrite an existing output file")
	flag.Parse()
	if len(inputs) == 0 || outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if testPercent < 1 || testPercent > 99 {
		fail(fmt.Errorf("test-percent must be between 1 and 99"))
	}
	if minContextCount < 1 {
		fail(fmt.Errorf("min-context-count must be positive"))
	}
	records := make([]jsut.Alignment, 0)
	for _, path := range inputs {
		loaded, err := jsut.ReadAlignments(path)
		if err != nil {
			fail(err)
		}
		records = append(records, loaded...)
	}
	train, test := splitRecords(records, testPercent)
	if len(train) == 0 || len(test) == 0 {
		fail(fmt.Errorf("holdout split produced an empty side (train=%d test=%d)", len(train), len(test)))
	}
	prior, err := jsut.BuildPrior(train, false, "jsut-target-eval-train", "")
	if err != nil {
		fail(err)
	}
	report := evaluate(train, test, prior, testPercent, minContextCount)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	writer, closeWriter, err := openOutput(outPath, force)
	if err != nil {
		fail(err)
	}
	if _, err := writer.Write(data); err != nil {
		_ = closeWriter()
		fail(fmt.Errorf("write %s: %w", outPath, err))
	}
	if err := closeWriter(); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "jsut target eval: train=%d test=%d phones=%d/%d context-used=%d -> %s\n", report.TrainUtterances, report.TestUtterances, report.TrainPhones, report.TestPhones, report.ContextUsed, outPath)
}

type inputPaths []string

func (paths *inputPaths) String() string { return strings.Join(*paths, ", ") }

func (paths *inputPaths) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("input path must not be empty")
	}
	*paths = append(*paths, value)
	return nil
}

func splitRecords(records []jsut.Alignment, testPercent int) (train, test []jsut.Alignment) {
	for _, record := range records {
		sum := sha256.Sum256([]byte(record.ID))
		bucket := int(binary.BigEndian.Uint32(sum[:4]) % 100)
		if bucket < testPercent {
			test = append(test, record)
		} else {
			train = append(train, record)
		}
	}
	return train, test
}

func evaluate(train, test []jsut.Alignment, prior *jsut.Prior, testPercent, minContextCount int) evaluationReport {
	report := evaluationReport{
		Version: 1, Kind: "jsut_target_prior_evaluation",
		SplitMethod: "sha256(id) mod 100",
		TestPercent: testPercent, MinContextCount: minContextCount,
		InputRecords: len(train) + len(test), TrainUtterances: len(train), TestUtterances: len(test),
		Notes: []string{
			"評価対象は音素継続時間であり自然さの主観評価ではない",
			"JSUTは単一話者のため話者一般化の評価ではない",
		},
	}
	var globalDuration scalarAccumulator
	var globalMetric, phoneMetric, contextMetric durationAccumulator
	for _, record := range train {
		for _, phone := range record.Phones {
			if phone.Silence || phone.Pause {
				continue
			}
			globalDuration.add(phone.DurationMS)
			report.TrainPhones++
		}
		report.TrainBoundaries = addBoundaryReport(report.TrainBoundaries, record)
	}
	for _, record := range test {
		for _, phone := range record.Phones {
			if phone.Silence || phone.Pause {
				continue
			}
			report.TestPhones++
		}
		report.TestBoundaries = addBoundaryReport(report.TestBoundaries, record)
	}
	globalMean := globalDuration.mean(0)
	var allocation allocationAccumulator
	allocationGroups, allocationIncomplete := 0, 0
	for _, record := range test {
		groups, incomplete := groupJapanesePhones(record.Phones)
		allocationIncomplete += incomplete
		for _, group := range groups {
			actualTotal := 0.0
			for _, phone := range group {
				actualTotal += phone.DurationMS
			}
			allocation.add(group, actualTotal, globalMean, prior, minContextCount)
			allocationGroups++
		}
	}
	for _, record := range test {
		for _, phone := range record.Phones {
			if phone.Silence || phone.Pause {
				continue
			}
			report.EligibleTestPhones++
			globalMetric.add(phone.DurationMS, globalMean)
			phoneMean := globalMean
			if stats, ok := prior.Phones[phone.Symbol]; ok && stats.DurationMS.Count > 0 {
				phoneMean = stats.DurationMS.Mean
			} else {
				report.UnseenPhones++
			}
			phoneMetric.add(phone.DurationMS, phoneMean)
			contextMean := phoneMean
			contextUsed := false
			key := phone.Previous + "|" + phone.Symbol + "|" + phone.Next
			if stats, ok := prior.Contexts[key]; ok && stats.Count >= minContextCount && stats.DurationMS.Count > 0 {
				contextMean = stats.DurationMS.Mean
				contextUsed = true
			}
			if contextUsed {
				report.ContextUsed++
			} else {
				report.ContextFallback++
			}
			contextMetric.add(phone.DurationMS, contextMean)
		}
	}
	report.GlobalBaseline = globalMetric.report()
	report.PhonePrior = phoneMetric.report()
	report.ContextBackoffPrior = contextMetric.report()
	report.ContextMAEGainMS = report.PhonePrior.MAE - report.ContextBackoffPrior.MAE
	report.Allocation = allocation.report(allocationGroups, allocationIncomplete)
	return report
}

func addBoundaryReport(report boundaryReport, record jsut.Alignment) boundaryReport {
	for _, boundary := range record.Boundaries {
		if !boundary.Trainable {
			continue
		}
		report.NaturalBoundaries++
		if boundary.Features != nil {
			report.WithFeatures++
		}
	}
	return report
}

func openOutput(path string, force bool) (io.Writer, func() error, error) {
	if path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, nil, err
		}
	}
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open output %s: %w (use --force to overwrite)", path, err)
	}
	return file, file.Close, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
