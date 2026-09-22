// intonation-preferenceは人の選好から韻律パラメータを調整するローカルA/B聴取ツール。外部教師モデルは使わない。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"utautts/internal/atomicfile"
	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/synth"
	"utautts/internal/tts"
)

const sessionVersion = 2

type prompt struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Focus string `json:"focus,omitempty"`
}

type candidate struct {
	ID                 string  `json:"id"`
	Strength           float64 `json:"strength"`
	Contrast           float64 `json:"contrast,omitempty"`
	OffsetCents        float64 `json:"offset_cents,omitempty"`
	DeclinationCents   float64 `json:"declination_cents,omitempty"`
	StatementTailCents float64 `json:"statement_tail_cents,omitempty"`
	QuestionTailCents  float64 `json:"question_tail_cents,omitempty"`
}

type manifest struct {
	Version    int         `json:"version"`
	Mode       string      `json:"mode"`
	CreatedAt  time.Time   `json:"created_at"`
	Voicebank  string      `json:"voicebank"`
	ModelFile  string      `json:"model_file"`
	Renderer   string      `json:"renderer"`
	MoraMS     float64     `json:"mora_ms"`
	PauseMS    float64     `json:"pause_ms"`
	Candidates []candidate `json:"candidates"`
	Prompts    []prompt    `json:"prompts"`
}

type vote struct {
	At       time.Time `json:"at"`
	PromptID string    `json:"prompt_id"`
	Left     string    `json:"left"`
	Right    string    `json:"right"`
	Choice   string    `json:"choice"`
}

type pairResponse struct {
	Prompt prompt `json:"prompt"`
	Left   side   `json:"left"`
	Right  side   `json:"right"`
	Votes  int    `json:"votes"`
}

type side struct {
	ID    string `json:"id"`
	Audio string `json:"audio"`
}

type voteRequest struct {
	PromptID string `json:"prompt_id"`
	Left     string `json:"left"`
	Right    string `json:"right"`
	Choice   string `json:"choice"`
}

type ranking struct {
	ID          string  `json:"id"`
	Strength    float64 `json:"strength"`
	Score       float64 `json:"score"`
	Comparisons int     `json:"comparisons"`
}

type server struct {
	manifest  manifest
	votesPath string
	audioDir  string
	bridge    string
	catalog   *plugin.Catalog
	mu        sync.Mutex
	votes     []vote
	random    *rand.Rand
	rendering map[string]bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	voicebank := flag.String("voicebank", "", "voicebank directory (required)")
	modelFile := flag.String("model-file", "", "explicit prosody model JSON (required)")
	corpus := flag.String("corpus", "tools/evaluation/japanese-v1.json", "JSON listening corpus")
	out := flag.String("out", "out/intonation-preference", "session output directory")
	renderer := flag.String("renderer", "utautts-world-phrase", "renderer ID")
	bridge := flag.String("bridge", "", "override WORLD bridge executable")
	strengths := flag.String("strengths", "1.0,1.2,1.4,1.6,1.8", "comma-separated intonation strengths")
	mode := flag.String("mode", "strength", "comparison mode: strength or contour")
	moraMS := flag.Float64("mora-ms", 120, "base mora duration in milliseconds")
	pauseMS := flag.Float64("pause-ms", 180, "pause duration in milliseconds")
	address := flag.String("address", "127.0.0.1:8765", "local listen address")
	flag.Parse()
	if *voicebank == "" || *modelFile == "" {
		return errors.New("voicebank and model-file are required")
	}
	if *moraMS <= 0 || *pauseMS <= 0 || math.IsNaN(*moraMS) || math.IsNaN(*pauseMS) || math.IsInf(*moraMS, 0) || math.IsInf(*pauseMS, 0) {
		return errors.New("mora-ms and pause-ms must be positive finite values")
	}
	var candidates []candidate
	var modeLabel string
	switch *mode {
	case "strength":
		values, err := parseStrengths(*strengths)
		if err != nil {
			return err
		}
		candidates = makeCandidates(values)
		modeLabel = formatStrengths(values)
	case "contour":
		candidates = makeContourCandidates()
		modeLabel = formatCandidateIDs(candidates)
	default:
		return fmt.Errorf("invalid mode %q; expected strength or contour", *mode)
	}
	prompts, err := readPrompts(*corpus)
	if err != nil {
		return err
	}
	voicebankPath, err := filepath.Abs(*voicebank)
	if err != nil {
		return err
	}
	modelPath, err := filepath.Abs(*modelFile)
	if err != nil {
		return err
	}
	if _, err := os.Stat(voicebankPath); err != nil {
		return fmt.Errorf("voicebank: %w", err)
	}
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model-file: %w", err)
	}
	catalog, err := plugin.DiscoverWithDefaults(nil, nil, render.IsKnownRenderer)
	if err != nil {
		return err
	}
	if _, ok := catalog.Renderer(*renderer); !ok {
		return fmt.Errorf("unknown renderer %q", *renderer)
	}
	root, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	m := manifest{
		Version: sessionVersion, Mode: *mode, CreatedAt: time.Now().UTC(), Voicebank: voicebankPath, ModelFile: modelPath,
		Renderer: *renderer, MoraMS: *moraMS, PauseMS: *pauseMS, Prompts: prompts, Candidates: candidates,
	}
	if err := writeManifest(filepath.Join(root, "session.json"), m); err != nil {
		return err
	}
	votesPath := filepath.Join(root, "votes.jsonl")
	votes, err := readVotes(votesPath)
	if err != nil {
		return err
	}
	s := &server{
		manifest: m, votesPath: votesPath, audioDir: filepath.Join(root, "audio"), bridge: *bridge, catalog: catalog,
		votes: votes, random: rand.New(rand.NewSource(time.Now().UnixNano())), rendering: make(map[string]bool),
	}
	if err := os.MkdirAll(s.audioDir, 0755); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/pair", s.handlePair)
	mux.HandleFunc("/api/vote", s.handleVote)
	mux.HandleFunc("/api/result", s.handleResult)
	mux.Handle("/audio/", http.StripPrefix("/audio/", http.FileServer(http.Dir(s.audioDir))))
	fmt.Printf("Preference listening session: http://%s\n", *address)
	fmt.Printf("Model: %s; mode: %s; candidates: %s\n", filepath.Base(modelPath), *mode, modeLabel)
	return http.ListenAndServe(*address, mux)
}

func parseStrengths(value string) ([]float64, error) {
	seen := make(map[float64]bool)
	var result []float64
	for _, item := range strings.Split(value, ",") {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(item), 64)
		if err != nil || parsed <= 0 || parsed > 4 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("invalid intonation strength %q; expected a value in (0, 4]", item)
		}
		if !seen[parsed] {
			seen[parsed] = true
			result = append(result, parsed)
		}
	}
	if len(result) < 2 {
		return nil, errors.New("at least two distinct strengths are required")
	}
	sort.Float64s(result)
	return result, nil
}

func makeCandidates(strengths []float64) []candidate {
	result := make([]candidate, len(strengths))
	for index, strength := range strengths {
		result[index] = candidate{ID: candidateID(strength), Strength: strength}
	}
	return result
}

func candidateID(strength float64) string {
	return "strength-" + strings.ReplaceAll(strconv.FormatFloat(strength, 'f', -1, 64), ".", "_")
}

func formatStrengths(values []float64) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strings.Join(parts, ", ")
}

func readPrompts(path string) ([]prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var prompts []prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt.ID) == "" || strings.TrimSpace(prompt.Text) == "" || seen[prompt.ID] {
			return nil, errors.New("corpus requires unique non-empty id and text fields")
		}
		seen[prompt.ID] = true
	}
	if len(prompts) == 0 {
		return nil, errors.New("corpus is empty")
	}
	return prompts, nil
}

func writeManifest(path string, value manifest) error {
	if existing, err := os.ReadFile(path); err == nil {
		var loaded manifest
		if json.Unmarshal(existing, &loaded) == nil && loaded.Version == sessionVersion && loaded.Mode == value.Mode {
			return nil
		}
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data)
}

func readVotes(path string) ([]vote, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []vote
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry vote
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("read votes: %w", err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *server) handleIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(indexHTML))
}

func (s *server) handlePair(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	prompt, left, right := s.nextPair()
	if err := s.ensureAudio(prompt, left); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.ensureAudio(prompt, right); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	response := pairResponse{Prompt: prompt, Left: side{ID: left.ID, Audio: "/audio/" + audioName(prompt, left)}, Right: side{ID: right.ID, Audio: "/audio/" + audioName(prompt, right)}, Votes: len(s.votes)}
	s.mu.Unlock()
	writeJSON(writer, response)
}

func (s *server) handleVote(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input voteRequest
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096)).Decode(&input); err != nil {
		http.Error(writer, "invalid vote", http.StatusBadRequest)
		return
	}
	if !s.validVote(input) {
		http.Error(writer, "invalid vote", http.StatusBadRequest)
		return
	}
	entry := vote{At: time.Now().UTC(), PromptID: input.PromptID, Left: input.Left, Right: input.Right, Choice: input.Choice}
	data, err := json.Marshal(entry)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	file, err := os.OpenFile(s.votesPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, err = file.Write(append(data, '\n'))
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err == nil {
		s.votes = append(s.votes, entry)
	}
	s.mu.Unlock()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, map[string]bool{"ok": true})
}

func (s *server) handleResult(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	result := rankCandidates(s.manifest.Candidates, s.votes)
	votes := len(s.votes)
	s.mu.Unlock()
	writeJSON(writer, map[string]any{"votes": votes, "ranking": result})
}

func (s *server) validVote(input voteRequest) bool {
	if input.Choice != "left" && input.Choice != "right" && input.Choice != "tie" && input.Choice != "neither" && input.Choice != "skip" {
		return false
	}
	if input.Left == input.Right || !s.hasPrompt(input.PromptID) || !s.hasCandidate(input.Left) || !s.hasCandidate(input.Right) {
		return false
	}
	return true
}

func (s *server) hasPrompt(id string) bool {
	for _, item := range s.manifest.Prompts {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (s *server) hasCandidate(id string) bool {
	for _, item := range s.manifest.Candidates {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (s *server) nextPair() (prompt, candidate, candidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	used := make(map[string]int)
	for _, vote := range s.votes {
		used[pairKey(vote.PromptID, vote.Left, vote.Right)]++
	}
	type option struct {
		prompt prompt
		left   candidate
		right  candidate
		used   int
	}
	var options []option
	for _, item := range s.manifest.Prompts {
		for left := 0; left < len(s.manifest.Candidates); left++ {
			for right := left + 1; right < len(s.manifest.Candidates); right++ {
				first, second := s.manifest.Candidates[left], s.manifest.Candidates[right]
				options = append(options, option{prompt: item, left: first, right: second, used: used[pairKey(item.ID, first.ID, second.ID)]})
			}
		}
	}
	minimum := options[0].used
	for _, item := range options[1:] {
		if item.used < minimum {
			minimum = item.used
		}
	}
	var choices []option
	for _, item := range options {
		if item.used == minimum {
			choices = append(choices, item)
		}
	}
	selected := choices[s.random.Intn(len(choices))]
	if s.random.Intn(2) == 0 {
		selected.left, selected.right = selected.right, selected.left
	}
	return selected.prompt, selected.left, selected.right
}

func pairKey(promptID, left, right string) string {
	if left > right {
		left, right = right, left
	}
	return promptID + "\x00" + left + "\x00" + right
}

func (s *server) ensureAudio(p prompt, c candidate) error {
	path := filepath.Join(s.audioDir, audioName(p, c))
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	s.mu.Lock()
	if s.rendering[path] {
		s.mu.Unlock()
		for {
			time.Sleep(50 * time.Millisecond)
			if _, err := os.Stat(path); err == nil {
				return nil
			}
			s.mu.Lock()
			busy := s.rendering[path]
			s.mu.Unlock()
			if !busy {
				break
			}
		}
	} else {
		s.rendering[path] = true
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(s.rendering, path)
			s.mu.Unlock()
		}()
		strength := c.Strength
		if s.manifest.Mode == "contour" {
			strength = 1
		}
		cfg := tts.Config{VoicebankPath: s.manifest.Voicebank, Text: p.Text, Tone: "C4", MoraDurationMS: s.manifest.MoraMS, PauseDurationMS: s.manifest.PauseMS, ApplyPitch: true, IntonationStrength: strength, ProsodyModelPath: s.manifest.ModelFile, Context: context.Background()}
		resolved, err := tts.ApplyRenderer(&cfg, s.catalog, s.manifest.Renderer, s.bridge)
		if err != nil {
			return err
		}
		if s.manifest.Mode == "contour" {
			preview, err := tts.PredictProsody(cfg)
			if err != nil {
				return err
			}
			curve, err := transformContour(preview, p.Text, c)
			if err != nil {
				return err
			}
			cfg.PitchCurve = curve
			cfg.MoraDurationsMS = append([]float64(nil), preview.MoraDurationsMS...)
			cfg.ProsodyModelPath = ""
		}
		result, err := synth.SynthesizeConfig(cfg, resolved)
		if err != nil {
			return err
		}
		return synth.WriteFiles(path, result, synth.ExportOptions{})
	}
	return fmt.Errorf("audio rendering did not produce %s", filepath.Base(path))
}

func audioName(p prompt, c candidate) string {
	return p.ID + "-" + c.ID + ".wav"
}

func rankCandidates(candidates []candidate, votes []vote) []ranking {
	type score struct{ wins, comparisons float64 }
	values := make(map[string]*score, len(candidates))
	strengths := make(map[string]float64, len(candidates))
	for _, item := range candidates {
		values[item.ID] = &score{}
		strengths[item.ID] = item.Strength
	}
	for _, vote := range votes {
		left, leftOK := values[vote.Left]
		right, rightOK := values[vote.Right]
		if !leftOK || !rightOK || vote.Choice == "skip" || vote.Choice == "neither" {
			continue
		}
		left.comparisons++
		right.comparisons++
		switch vote.Choice {
		case "left":
			left.wins++
		case "right":
			right.wins++
		case "tie":
			left.wins += .5
			right.wins += .5
		}
	}
	result := make([]ranking, 0, len(candidates))
	for _, item := range candidates {
		entry := values[item.ID]
		score := .5
		if entry.comparisons > 0 {
			score = entry.wins / entry.comparisons
		}
		result = append(result, ranking{ID: item.ID, Strength: strengths[item.ID], Score: score, Comparisons: int(entry.comparisons)})
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Score == result[right].Score {
			return result[left].Strength < result[right].Strength
		}
		return result[left].Score > result[right].Score
	})
	return result
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(value)
}

const indexHTML = `<!doctype html>
<html lang="ja"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>UtauTTS 抑揚の聴取比較</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#f7f8fa;color:#1f2933}main{max-width:720px;margin:0 auto;padding:32px 20px}.prompt{font-size:1.25rem;line-height:1.7;margin:20px 0 24px}.pairs{display:grid;grid-template-columns:1fr 1fr;gap:16px}.side{background:#fff;border:1px solid #d7dde4;border-radius:10px;padding:16px}.side h2{font-size:1rem;margin:0 0 12px}audio{width:100%}.choices{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-top:12px}button{min-height:42px;border:1px solid #b9c4d0;border-radius:7px;background:#fff;font:inherit;cursor:pointer}button:hover{background:#edf5ff}.other{display:flex;gap:8px;margin-top:16px}.other button{padding:0 14px}.status{color:#667085;font-size:.9rem;margin-top:20px}.result{margin-top:20px;white-space:pre-wrap}.loading{opacity:.6;pointer-events:none}@media(max-width:560px){.pairs{grid-template-columns:1fr}}
</style><main><h1>抑揚の聴取比較</h1><p>同じ文章のA/Bを聴き、自然に聞こえる方を選んでください。候補の設定値は表示しません。</p><div id="app" class="loading"><div id="prompt" class="prompt">読み込み中…</div><div class="pairs"><section class="side"><h2>A</h2><audio id="left" controls preload="auto"></audio><div class="choices"><button onclick="vote('left')">Aを選ぶ</button></div></section><section class="side"><h2>B</h2><audio id="right" controls preload="auto"></audio><div class="choices"><button onclick="vote('right')">Bを選ぶ</button></div></section></div><div class="other"><button onclick="vote('tie')">同じくらい</button><button onclick="vote('neither')">どちらも自然ではない</button><button onclick="vote('skip')">判断できない</button><button onclick="showResult()">途中結果を見る</button></div><div id="status" class="status"></div><div id="result" class="result"></div></div></main>
<script>
let pair=null;const app=document.getElementById('app');
async function next(){app.classList.add('loading');document.getElementById('result').textContent='';const r=await fetch('/api/pair');if(!r.ok){document.getElementById('prompt').textContent=await r.text();return}pair=await r.json();document.getElementById('prompt').textContent=pair.prompt.text;const a=document.getElementById('left'),b=document.getElementById('right');a.src=pair.left.audio;b.src=pair.right.audio;a.load();b.load();document.getElementById('status').textContent=pair.votes+' 件の選択を記録済み';app.classList.remove('loading')}
async function vote(choice){if(!pair)return;app.classList.add('loading');await fetch('/api/vote',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({prompt_id:pair.prompt.id,left:pair.left.id,right:pair.right.id,choice})});await next()}
async function showResult(){const r=await fetch('/api/result');const data=await r.json();const lines=['選択数: '+data.votes];for(const x of data.ranking){lines.push('候補 '+x.id+' — 選好率 '+Math.round(x.score*100)+'% ('+x.comparisons+' 比較)')}document.getElementById('result').textContent=lines.join('\n')}
next();
</script></html>`
