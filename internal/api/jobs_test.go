package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yh2237/UtauTTS/internal/synth"
)

func TestExpiredSynthesisJobIsNotServed(t *testing.T) {
	for _, suffix := range []string{"", "/audio"} {
		t.Run(suffix, func(t *testing.T) {
			server := mustNewServer(t, Config{VoiceDir: t.TempDir()})
			id, ok := server.jobs.create()
			if !ok {
				t.Fatal("could not create job")
			}
			server.jobs.update(id, func(job *synthesisJob) {
				job.status = jobSucceeded
				job.finishedAt = time.Now().Add(-synthesisJobTTL - time.Minute)
				job.report = &synth.Report{}
				job.audio = []byte("RIFF")
			})
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, localRequest(http.MethodGet, "/api/synthesize/jobs/"+id+suffix, nil))
			assertAPIError(t, response, http.StatusNotFound, codeNotFound)
			if len(server.jobs.jobs) != 0 {
				t.Fatal("expired job is still retained")
			}
		})
	}
}

func TestReadingFullJobStorePreservesUnexpiredJobs(t *testing.T) {
	store := newJobStore()
	ids := make([]string, maxSynthesisJobs)
	for i := range ids {
		id, ok := store.create()
		if !ok {
			t.Fatalf("could not create job %d", i)
		}
		ids[i] = id
		store.update(id, func(job *synthesisJob) {
			job.status = jobSucceeded
			job.finishedAt = time.Now()
			job.report = &synth.Report{}
			job.audio = []byte("RIFF")
		})
	}
	for _, id := range ids {
		if _, ok := store.response(id); !ok {
			t.Fatalf("unexpired job %s was removed", id)
		}
		if _, _, found, ready := store.audioOf(id); !found || !ready {
			t.Fatalf("unexpired audio %s was removed", id)
		}
	}
}

func TestJobStoreRetainsActiveJobsAndEvictsOldestFinished(t *testing.T) {
	store := newJobStore()
	ids := make([]string, maxSynthesisJobs)
	for i := range ids {
		id, ok := store.create()
		if !ok {
			t.Fatalf("could not create job %d", i)
		}
		ids[i] = id
	}
	if _, ok := store.create(); ok {
		t.Fatal("accepted a job while all slots are active")
	}
	store.update(ids[0], func(job *synthesisJob) {
		job.createdAt = time.Now().Add(-2 * synthesisJobTTL)
	})
	oldest, recent := ids[1], ids[2]
	for i, id := range []string{oldest, recent} {
		store.update(id, func(job *synthesisJob) {
			job.status = jobFailed
			job.finishedAt = time.Now().Add(time.Duration(i-2) * time.Minute)
		})
	}
	if _, ok := store.create(); !ok {
		t.Fatal("did not make room for a new job")
	}
	if _, ok := store.response(oldest); ok {
		t.Fatal("oldest finished job was not evicted")
	}
	for _, id := range []string{ids[0], recent} {
		if _, ok := store.response(id); !ok {
			t.Fatalf("job %s was removed instead of the oldest finished job", id)
		}
	}
}
