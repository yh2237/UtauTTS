package refspeech

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauWLD/worldrender"
)

type IndexState string

const (
	IndexMissing  IndexState = ""
	IndexBuilding IndexState = "building"
	IndexReady    IndexState = "ready"
	IndexFailed   IndexState = "failed"
)

type IndexStatus struct {
	State    IndexState `json:"state"`
	Progress float64    `json:"progress,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// Serviceはモデルと音源ごとの索引を持ち、索引を裏で作る。
type Service struct {
	models   Models
	cacheDir string

	mu       sync.Mutex
	buildMu  sync.Mutex
	engineMu sync.Mutex
	worldMu  sync.Mutex
	// foregroundは読み上げの間に持つ。索引の作成は、ファイルごとに読み上げが終わるのを待つ。
	foreground sync.RWMutex
	engine     *Engine
	worlds     map[string]*worldrender.World
	indexes    map[string]*indexEntry

	// cleanupOnceは、キャッシュの整理を1プロセスにつき1度だけ行うためのもの。
	cleanupOnce sync.Once
	cleanupLog  string
}

type indexEntry struct {
	status IndexStatus
	index  *index.Index
	done   chan struct{}
	// logsは索引を作ったときの途中経過。待っていた側へまとめて渡す。
	logs []string
	// builtはこのプロセスで索引を作ったかどうか（キャッシュから読んだだけならfalse）。
	built bool
	// needは、索引を作るのに外の推論が要るときの頼みごと。
	need *NeedInference
}

var (
	sharedMu       sync.Mutex
	sharedServices = map[string]*Service{}

	batchMu sync.Mutex
	batch   indexBatch

	stageMu      sync.Mutex
	currentStage string
)

// indexBatchは、作成中の索引がなくなるまでに作り始めた索引の数と、作り終えた数。
type indexBatch struct {
	total, done int
	current     float64
	failed      []string
}

type Progress struct {
	Building bool     `json:"building"`
	Progress float64  `json:"progress"`
	Failed   []string `json:"failed,omitempty"`
	// Stageは、参照照合の合成がいまどの段階にあるか。止まったときに手掛かりにする。
	Stage string `json:"stage,omitempty"`
	// Deviceは、直近の特徴量計算に使ったデバイス。計算が始まるまで空。
	Device string `json:"device,omitempty"`
}

// CurrentProgressは、作成中の索引がなくなるまでにまとめて作った索引全体の進み具合を返す。
// 作成中の索引がなくなると、それまでに失敗した索引を返して数え直す。
func CurrentProgress() Progress {
	stageMu.Lock()
	stage := currentStage
	stageMu.Unlock()
	batchMu.Lock()
	defer batchMu.Unlock()
	if batch.total == 0 || batch.done >= batch.total {
		progress := Progress{Failed: batch.failed, Stage: stage, Device: encoder.LastDevice()}
		batch = indexBatch{}
		return progress
	}
	return Progress{
		Building: true, Progress: (float64(batch.done) + batch.current) / float64(batch.total),
		Stage: stage, Device: encoder.LastDevice(),
	}
}

// SetStageは、参照照合の合成の段階を記録する。空にすると消える。
func SetStage(stage string) {
	stageMu.Lock()
	currentStage = stage
	stageMu.Unlock()
}

// Sharedはモデルの置き場所ごとに1つのServiceを返す。
func Shared(models Models, cacheDir string) *Service {
	key := strings.Join([]string{models.KokoroWeights, models.WavLM, models.Espeak, models.EspeakConfig,
		models.EspeakPreprocessor, models.Whisper, models.WhisperConfig, strings.Join(models.ExternalEncoders, ","), cacheDir}, "\x00")
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if service, ok := sharedServices[key]; ok {
		return service
	}
	service := &Service{models: models, cacheDir: cacheDir, worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	sharedServices[key] = service
	return service
}

// CleanupCacheは、使えなくなった索引と、しばらく使われていない部分キャッシュを削除する。
// 索引を作る前に1度だけ呼ぶ。削除の結果を1行で返す。
func (s *Service) CleanupCache() string {
	s.cleanupOnce.Do(func() {
		result, err := Cleanup(s.cacheDir)
		if err != nil {
			s.cleanupLog = fmt.Sprintf("索引のキャッシュを整理できませんでした: %v", err)
			return
		}
		s.cleanupLog = result.Summary()
	})
	return s.cleanupLog
}

// DefaultCacheDirは利用者ごとのキャッシュに置く索引のディレクトリ。
// 索引は作り直せる大きな派生データなので、アプリのフォルダではなくキャッシュに置く。
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "UtauTTS", "reference-index")
	migrateLegacyCacheDir(base, dir)
	return dir, nil
}

// legacyCacheNameは、中国語を足す前の索引のフォルダ名。
const legacyCacheName = "english-reference"

// migrateLegacyCacheDirは、古い名前のフォルダがあれば1度だけ今の名前に移す。
// 同じ場所なので、中身の移動は名前の変更だけで済む。
func migrateLegacyCacheDir(base, dir string) {
	if _, err := os.Stat(dir); err == nil {
		return
	}
	legacy := filepath.Join(base, "UtauTTS", legacyCacheName)
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if err := os.Rename(legacy, dir); err != nil {
		// 名前を変えられない環境では、古いフォルダをそのまま使う。
		return
	}
}

func (s *Service) openEngine() (*Engine, error) {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.engine == nil {
		engine, err := Open(s.models)
		if err != nil {
			return nil, err
		}
		s.engine = engine
	}
	return s.engine, nil
}

func (s *Service) open(worldEnginePath string) (*Engine, *worldrender.World, error) {
	engine, err := s.openEngine()
	if err != nil {
		return nil, nil, err
	}
	if worldEnginePath == "" {
		return nil, nil, errors.New("English synthesis needs the UtauTTS WORLD engine")
	}
	s.worldMu.Lock()
	defer s.worldMu.Unlock()
	world := s.worlds[worldEnginePath]
	if world == nil {
		world, err = worldrender.OpenWorld(worldEnginePath)
		if err != nil {
			return nil, nil, err
		}
		s.worlds[worldEnginePath] = world
	}
	return engine, world, nil
}

// Prepareは音源の索引がないか、録音が変わっていれば、裏で作り始める。
func (s *Service) Prepare(bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string) {
	s.dropStale(bankRoot, color, entries)
	s.entry(bankRoot, color, entries, worldEnginePath)
}

// dropStaleは、作った後に録音のファイルが変わった索引を捨てる。
func (s *Service) dropStale(bankRoot string, color string, entries []speechpool.Entry) {
	id := indexID(bankRoot, color, entries)
	s.mu.Lock()
	entry := s.indexes[id]
	var idx *index.Index
	if entry != nil {
		idx = entry.index
	}
	s.mu.Unlock()
	if idx == nil {
		return
	}
	if _, source, err := bankRecordings(bankRoot, color, entries); err != nil || !matchesSource(idx, bankRoot, indexCacheKey(source, indexChannelIDs(idx.Header), configuredPoolMode())) {
		s.mu.Lock()
		if s.indexes[id] == entry {
			delete(s.indexes, id)
		}
		s.mu.Unlock()
	}
}

// Indexは音源の索引を返す。索引がなければ作り、でき上がるまで待つ。
func (s *Service) Index(ctx context.Context, bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string) (*index.Index, error) {
	return s.index(ctx, bankRoot, color, entries, worldEnginePath, nil)
}

// indexは索引を返し、作成の経過をlogfへ伝える。logfがnilなら何も伝えない。
func (s *Service) index(ctx context.Context, bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string, logf func(string)) (*index.Index, error) {
	start := time.Now()
	SetStage("索引を準備しています")
	s.dropStale(bankRoot, color, entries)
	entry := s.entry(bankRoot, color, entries, worldEnginePath)
	select {
	case <-entry.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	elapsed := time.Since(start)
	s.mu.Lock()
	defer s.mu.Unlock()
	// 作成の途中経過は、索引を作った側が記録している。待っていた側へまとめて渡す。
	if logf != nil {
		for _, line := range entry.logs {
			logf(line)
		}
	}
	if entry.need != nil {
		return nil, entry.need
	}
	if entry.index == nil {
		if logf != nil {
			logf(fmt.Sprintf("索引: %s の作成に失敗しました（%.1f 秒）: %s",
				filepath.Base(bankRoot), elapsed.Seconds(), entry.status.Error))
		}
		return nil, fmt.Errorf("voicebank index: %s", entry.status.Error)
	}
	if logf != nil && entry.built {
		logf(fmt.Sprintf("索引: %s の準備ができました（%.1f 秒、フレーム %d）",
			filepath.Base(bankRoot), elapsed.Seconds(), entry.index.Frames))
	}
	return entry.index, nil
}

func (s *Service) entry(bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string) *indexEntry {
	id := indexID(bankRoot, color, entries)
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.indexes[id]; ok && entry.status.State != IndexFailed {
		return entry
	}
	entry := &indexEntry{status: IndexStatus{State: IndexBuilding}, done: make(chan struct{})}
	s.indexes[id] = entry
	batchMu.Lock()
	batch.total++
	batchMu.Unlock()
	go s.build(bankRoot, color, entries, worldEnginePath, entry)
	return entry
}

// buildは索引を1つずつ作る。どの索引の作成も全コアを使うため。
func (s *Service) build(bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string, entry *indexEntry) {
	defer close(entry.done)
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("索引の作成を中止しました: %v", recovered)
			fmt.Fprintln(os.Stderr, "UtauTTS:", message)
			batchMu.Lock()
			batch.done++
			batch.current = 0
			batch.failed = append(batch.failed, fmt.Sprintf("%s: %v", filepath.Base(bankRoot), recovered))
			batchMu.Unlock()
			s.mu.Lock()
			entry.logs = append(entry.logs, message)
			entry.status = IndexStatus{State: IndexFailed, Error: message}
			s.mu.Unlock()
		}
	}()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	var logs []string
	log := func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	// lastStepは、進捗を最後に記録した1割の段階。
	lastStep := -1
	if summary := s.CleanupCache(); summary != "" {
		log("%s", summary)
	}
	index, fromCache, err := s.loadOrBuild(bankRoot, color, entries, worldEnginePath, log, func(done, total int) {
		progress := float64(done) / float64(max(total, 1))
		s.mu.Lock()
		entry.status.Progress = progress
		s.mu.Unlock()
		batchMu.Lock()
		batch.current = progress
		batchMu.Unlock()
		// 1割ごとに経過を残す。時間のかかる音源で、どこまで進んだか分かるようにする。
		if step := done * 10 / max(total, 1); step != lastStep && step < 10 {
			lastStep = step
			log("索引: %s を解析しています（%d%%、%d/%d 件）",
				filepath.Base(bankRoot), step*10, done, total)
		}
	})
	var need *NeedInference
	errors.As(err, &need)
	batchMu.Lock()
	batch.done++
	batch.current = 0
	if err != nil && need == nil {
		batch.failed = append(batch.failed, fmt.Sprintf("%s: %v", filepath.Base(bankRoot), err))
	}
	batchMu.Unlock()
	s.mu.Lock()
	entry.logs = logs
	entry.built = !fromCache
	entry.need = need
	if err != nil {
		entry.status = IndexStatus{State: IndexFailed, Error: err.Error()}
	} else {
		entry.status = IndexStatus{State: IndexReady, Progress: 1}
		entry.index = index
	}
	s.mu.Unlock()
}

func (s *Service) loadOrBuild(bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string,
	logf func(string, ...any), progress func(done, total int)) (*index.Index, bool, error) {
	files, source, err := bankRecordings(bankRoot, color, entries)
	if err != nil {
		return nil, false, err
	}
	engine, err := s.openEngine()
	if err != nil {
		return nil, false, err
	}
	active := engine.registry.List()
	ids := engine.registry.IDs()
	poolMode := configuredPoolMode()
	key := indexCacheKey(source, ids, poolMode)
	path := filepath.Join(s.cacheDir, key+".utsp")
	if idx, err := index.Load(path); err == nil && matchesSource(idx, bankRoot, key) && hasEncoders(idx, active) {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		logf("索引: %s のキャッシュを読み込みました（録音 %d 件、%d フレーム）",
			filepath.Base(bankRoot), len(idx.Files), idx.Frames)
		return idx, true, nil
	}
	if engine.external {
		if err := engine.filesJob(key, files); err != nil {
			return nil, false, err
		}
	}
	_, world, err := s.open(worldEnginePath)
	if err != nil {
		return nil, false, err
	}
	logf("索引: %s を作成します（録音 %d 件、モデル %s）", filepath.Base(bankRoot), len(files), strings.Join(ids, ", "))

	effectiveMode := speechpool.PoolRawOnly
	_, profile, scanErr := speechpool.ScanRawVoicebank(bankRoot)
	canGenerate := scanErr == nil && profile.UsableUnits >= 4
	if canGenerate {
		for i := range files {
			files[i].Penalty += speechpool.CleanFilePenalty(files[i].Path, profile.IsSmallJPCV)
		}
		effectiveMode = poolMode
		if poolMode == speechpool.PoolAuto {
			effectiveMode = speechpool.ResolvePoolMode(poolMode, profile, estimateRawFrames(files), 0)
		}
	}
	encoders := make([]encoder.Encoder, len(active))
	for i, enc := range active {
		encoders[i] = lockedEncoder{enc, &s.foreground}
	}
	opts := index.BuildOptions{Source: filepath.Clean(bankRoot), CacheKey: key, Progress: progress}
	if !engine.external {
		opts.Cache = index.OpenPartCache(filepath.Join(s.cacheDir, "parts"))
	}
	poolEntries := files
	var babbleDir string
	if effectiveMode != speechpool.PoolRawOnly {
		babble, dir, err := s.ensureBabbleEntries(context.Background(), bankRoot, color, source, worldEnginePath, files, engine.external)
		if err != nil {
			return nil, false, err
		}
		babbleDir = dir
		poolEntries = babble
		if effectiveMode == speechpool.PoolMerge {
			poolEntries = append(poolEntries, files...)
		}
	}
	idx, err := index.Build(poolEntries, world, encoders, opts)
	if err != nil {
		return nil, false, wrapNeedInference(err)
	}
	if poolMode == speechpool.PoolAuto && effectiveMode == speechpool.PoolRawOnly && idx.C35 > 1.25 && canGenerate {
		babble, dir, err := s.ensureBabbleEntries(context.Background(), bankRoot, color, source, worldEnginePath, files, engine.external)
		if err != nil {
			return nil, false, err
		}
		babbleDir = dir
		idx, err = index.Build(babble, world, encoders, opts)
		if err != nil {
			return nil, false, wrapNeedInference(err)
		}
	}
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return nil, false, err
	}
	if err := idx.Save(path); err != nil {
		return nil, false, err
	}
	if engine.external {
		s.foreground.Lock()
		engine.externalStore.Clear()
		s.foreground.Unlock()
		if babbleDir != "" {
			_ = os.RemoveAll(babbleDir)
		}
	}
	logf("索引: %s を保存しました（%d フレーム、F0 の中央値 %.0f Hz）",
		filepath.Base(bankRoot), idx.Frames, idx.F0Median)
	return idx, false, nil
}

type lockedEncoder struct {
	encoder.Encoder
	mu *sync.RWMutex
}

func (e lockedEncoder) Extract(wave []float32) ([]float32, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Encoder.Extract(wave)
}

func hasEncoders(idx *index.Index, encoders []encoder.Encoder) bool {
	for _, enc := range encoders {
		if idx.ChannelDims(enc.ID()) != enc.Dims() {
			return false
		}
	}
	return true
}

func indexID(bankRoot string, color string, entries []speechpool.Entry) string {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.Path
	}
	sort.Strings(paths)
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", filepath.Clean(bankRoot), color, configuredPoolMode())
	for _, path := range paths {
		fmt.Fprintf(hash, "%s\x00", path)
	}
	return hex.EncodeToString(hash.Sum(nil))[:32]
}

func bankRecordings(bankRoot string, color string, entries []speechpool.Entry) ([]speechpool.Entry, string, error) {
	sorted := append([]speechpool.Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%s\x00", filepath.Clean(bankRoot), color)
	var files []speechpool.Entry
	for _, entry := range sorted {
		info, err := os.Stat(entry.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, entry)
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00%s\x00%g\x00", entry.Path, info.Size(), info.ModTime().UnixNano(), entry.Layer, entry.Penalty)
		for _, region := range entry.Regions {
			fmt.Fprintf(hash, "%g\x00%g\x00", region.Offset, region.Blank)
		}
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no WAV files in %s", bankRoot)
	}
	return files, hex.EncodeToString(hash.Sum(nil)), nil
}

func indexCacheKey(source string, encoderIDs []string, mode speechpool.PoolMode) string {
	key := sha256.Sum256([]byte("utsp-v1\x00" + strings.Join(encoderIDs, ",") + "\x00" + string(mode) + "\x00" + source))
	return hex.EncodeToString(key[:16])
}

func (s *Service) Speak(ctx context.Context, id, bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string, pieces []Piece, options Options) (Speech, error) {
	index, err := s.index(ctx, bankRoot, color, entries, worldEnginePath, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.Speak(id, index, world, pieces, options)
}

// Readは音素列を参照TTS（Kokoro）で読み、参照の音声とF0を返す。
func (s *Service) Read(id string, pieces []Piece, speed float32, durations []int) (Reference, error) {
	engine, err := s.openEngine()
	if err != nil {
		return Reference{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.Read(id, pieces, speed, durations)
}

// SpeakReferenceは、参照の読みから音源のフレームで合成し直す。参照TTSの種類に依存しない入口。
func (s *Service) SpeakReference(ctx context.Context, bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string, reference Reference, options Options) (Speech, error) {
	index, err := s.index(ctx, bankRoot, color, entries, worldEnginePath, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.SpeakReference(index, world, reference, options)
}

// SpeakAudioは参照の音声から、音源のフレームで合成し直す。言語に依存しない入口。
func (s *Service) SpeakAudio(ctx context.Context, bankRoot string, color string, entries []speechpool.Entry, worldEnginePath string, audio []float64, sampleRate int, options AudioOptions) (Speech, error) {
	index, err := s.index(ctx, bankRoot, color, entries, worldEnginePath, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	engine, world, err := s.open(worldEnginePath)
	if err != nil {
		return Speech{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.SpeakAudio(index, world, audio, sampleRate, options)
}

// ReadProsodyは音素列をKokoroで読み、記号ごとの長さとF0を返す。音源と索引は使わない。
func (s *Service) ReadProsody(id string, pieces []Piece, speed float32, durations []int) (Reference, error) {
	engine, err := s.openEngine()
	if err != nil {
		return Reference{}, err
	}
	s.foreground.RLock()
	defer s.foreground.RUnlock()
	return engine.ReadProsody(id, pieces, speed, durations)
}

// ProvideReadは、外で求めた読みを渡す（Web版）。
func (s *Service) ProvideRead(key string, durations []int, f0, audio []float32) error {
	engine, err := s.openEngine()
	if err != nil {
		return err
	}
	return engine.ProvideRead(key, durations, f0, audio)
}

// ProvidePrefetchedは、索引を作る前に録音を取得し終えたことを渡す（Web版）。
func (s *Service) ProvidePrefetched(key string) error {
	engine, err := s.openEngine()
	if err != nil {
		return err
	}
	engine.ProvidePrefetched(key)
	return nil
}

// ProvideFeaturesは、外で求めた音声の特徴量を渡す（Web版）。
func (s *Service) ProvideFeatures(encoderID, key string, values []float32, frames, dims int) error {
	engine, err := s.openEngine()
	if err != nil {
		return err
	}
	return engine.ProvideFeatures(encoderID, key, values, frames, dims)
}
