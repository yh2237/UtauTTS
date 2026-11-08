package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/synth"
)

const (
	maxSynthesisJobs = 32
	synthesisJobTTL  = 10 * time.Minute
)

// ジョブの状態。queuedは実行枠待ち、runningは合成中。
const (
	jobQueued    = "queued"
	jobRunning   = "running"
	jobSucceeded = "succeeded"
	jobFailed    = "failed"
)

type synthesisJob struct {
	id         string
	sequence   uint64
	status     string
	createdAt  time.Time
	startedAt  time.Time
	finishedAt time.Time
	report     *synth.Report
	audio      []byte
	statusCode int
	err        error
}

// synthesisJobResponseは、ポーリングで返すジョブの状態。
// 失敗はトップレベルのエラー応答と同じ形で返す。
type synthesisJobResponse struct {
	JobID         string        `json:"job_id"`
	Status        string        `json:"status"`
	QueuePosition int           `json:"queue_position"`
	CreatedAt     time.Time     `json:"created_at"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	FinishedAt    *time.Time    `json:"finished_at,omitempty"`
	ElapsedMS     int64         `json:"elapsed_ms"`
	Result        *synth.Report `json:"result,omitempty"`
	Error         string        `json:"error,omitempty"`
	Code          string        `json:"code,omitempty"`
	Field         string        `json:"field,omitempty"`
	Expected      string        `json:"expected,omitempty"`
}

type jobStore struct {
	mu      sync.Mutex
	jobs    map[string]*synthesisJob
	nextSeq uint64
}

func newJobStore() *jobStore {
	return &jobStore{jobs: map[string]*synthesisJob{}}
}

// createはジョブをqueuedで登録する。保持数を超えたらfalseを返す。
func (s *jobStore) create() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now())
	if len(s.jobs) >= maxSynthesisJobs {
		return "", false
	}
	s.nextSeq++
	job := &synthesisJob{id: newJobID(), sequence: s.nextSeq, status: jobQueued, createdAt: time.Now()}
	s.jobs[job.id] = job
	return job.id, true
}

func (s *jobStore) update(id string, apply func(*synthesisJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		apply(job)
	}
}

// responseはジョブの状態を組み立てる。見つからなければfalseを返す。
func (s *jobStore) response(id string) (synthesisJobResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return synthesisJobResponse{}, false
	}
	response := synthesisJobResponse{
		JobID: job.id, Status: job.status, CreatedAt: job.createdAt,
		ElapsedMS: job.elapsedMS(time.Now()),
	}
	if job.status == jobQueued {
		for _, other := range s.jobs {
			if other.status == jobQueued && other.sequence < job.sequence {
				response.QueuePosition++
			}
		}
	}
	if !job.startedAt.IsZero() {
		started := job.startedAt
		response.StartedAt = &started
	}
	if !job.finishedAt.IsZero() {
		finished := job.finishedAt
		response.FinishedAt = &finished
	}
	if job.report != nil {
		response.Result = job.report
	}
	if job.err != nil {
		_, failure := apiError(job.statusCode, job.err)
		response.Error, response.Code = failure.Error, failure.Code
		response.Field, response.Expected = failure.Field, failure.Expected
	}
	return response, true
}

// audioOfは完了したジョブのWAVを返す。foundはジョブの有無、readyはWAVの有無。
func (s *jobStore) audioOf(id string) (wav []byte, report *synth.Report, found, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, nil, false, false
	}
	if job.status != jobSucceeded {
		return nil, nil, true, false
	}
	return job.audio, job.report, true, true
}

func (s *jobStore) pruneLocked(now time.Time) {
	for id, job := range s.jobs {
		if !job.finishedAt.IsZero() && now.Sub(job.finishedAt) > synthesisJobTTL {
			delete(s.jobs, id)
		}
	}
	for len(s.jobs) >= maxSynthesisJobs {
		oldest := ""
		var oldestTime time.Time
		for id, job := range s.jobs {
			if job.finishedAt.IsZero() {
				continue
			}
			if oldest == "" || job.finishedAt.Before(oldestTime) {
				oldest, oldestTime = id, job.finishedAt
			}
		}
		if oldest == "" {
			return
		}
		delete(s.jobs, oldest)
	}
}

func (j *synthesisJob) elapsedMS(now time.Time) int64 {
	start := j.createdAt
	if !j.startedAt.IsZero() {
		start = j.startedAt
	}
	end := now
	if !j.finishedAt.IsZero() {
		end = j.finishedAt
	}
	return end.Sub(start).Milliseconds()
}

func newJobID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buffer[:])
}

func (s *Server) handleCreateSynthesisJob(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	// 入力の誤りはジョブを作らずに返す。
	if status, err := validateSynthesisRequest(request); err != nil {
		writeAPIError(w, status, err)
		return
	}
	id, ok := s.jobs.create()
	if !ok {
		writeAPIError(w, http.StatusTooManyRequests, fmt.Errorf("at most %d synthesis jobs are kept; retry after the finished ones expire", maxSynthesisJobs))
		return
	}
	go s.runSynthesisJob(id, request)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id": id, "status": jobQueued, "status_url": "/api/synthesize/jobs/" + id,
	})
}

func (s *Server) runSynthesisJob(id string, request SynthesisRequest) {
	release, status, err := s.acquire(s.jobCtx, s.synthesisSem, &s.synthesisWaiting)
	if err != nil {
		s.finishSynthesisJob(id, status, err)
		return
	}
	defer release()
	s.jobs.update(id, func(job *synthesisJob) {
		job.status, job.startedAt = jobRunning, time.Now()
	})
	result, status, err := s.synthesizeAdmitted(s.jobCtx, request)
	if err != nil {
		s.finishSynthesisJob(id, status, err)
		return
	}
	report := result.Report()
	wav := audio.PCMToWavBytes(result.Audio)
	s.jobs.update(id, func(job *synthesisJob) {
		job.status, job.report, job.audio, job.finishedAt = jobSucceeded, &report, wav, time.Now()
	})
}

func (s *Server) finishSynthesisJob(id string, status int, err error) {
	s.jobs.update(id, func(job *synthesisJob) {
		job.status, job.statusCode, job.err, job.finishedAt = jobFailed, status, err, time.Now()
	})
}

func (s *Server) handleSynthesisJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("job_id")
	response, ok := s.jobs.response(id)
	if !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Errorf("synthesis job %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSynthesisJobAudio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("job_id")
	wav, report, found, ready := s.jobs.audioOf(id)
	if !found {
		writeAPIError(w, http.StatusNotFound, fmt.Errorf("synthesis job %q not found", id))
		return
	}
	if !ready {
		writeAPIError(w, http.StatusConflict, errors.New("synthesis job has no audio yet"))
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-UtauTTS-Method", report.Method)
	w.Header().Set("X-UtauTTS-Reading", report.Reading)
	w.Header().Set("X-UtauTTS-Diagnostic-Count", strconv.Itoa(len(report.Diagnostics)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(wav); err != nil {
		log.Printf("write job audio response: %v", err)
	}
}
