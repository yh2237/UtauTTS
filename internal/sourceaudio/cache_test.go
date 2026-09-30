package sourceaudio

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
)

func writeFixture(t *testing.T, path string, value int16) *audio.PCM {
	t.Helper()
	pcm := &audio.PCM{SampleRate: 1000, Channels: 2, Data: make([]int16, 200)}
	for i := range pcm.Data {
		pcm.Data[i] = value + int16(i%2)
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	return pcm
}

func TestSharedSourceTrimMatchesIndependentConversionAndDoesNotMutate(t *testing.T) {
	Clear()
	t.Cleanup(Clear)
	path := filepath.Join(t.TempDir(), "source.wav")
	original := writeFixture(t, path, -1)
	want, err := audio.TrimPCM(original, 10, -30)
	if err != nil {
		t.Fatal(err)
	}
	got, wave, err := TrimmedMono(path, 10, -30)
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(wave, acoustic.Mono(want)) {
		t.Fatalf("trim/float channel mixing changed: %v, %v", got, err)
	}
	_, whole, err := ReadMono(path)
	if err != nil || &wave[0] != &whole[10] {
		t.Fatal("mono conversion was not reused")
	}
	copyPCM, err := audio.TrimPCM(got, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	copyPCM.Data[0] = 30000
	raw, err := ReadWav(path)
	if err != nil || !reflect.DeepEqual(raw, original) {
		t.Fatal("owned trim modified the shared source")
	}
}

func TestCacheInvalidatesModifiedDeletedAndClearedSources(t *testing.T) {
	Clear()
	t.Cleanup(Clear)
	path := filepath.Join(t.TempDir(), "source.wav")
	writeFixture(t, path, 100)
	first, mono, err := ReadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, path, 200)
	stamp := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	second, secondMono, err := ReadMono(path)
	if err != nil || first == second || second.Data[0] != 200 || mono[0] == secondMono[0] {
		t.Fatal("same-size replacement did not invalidate raw and mono data")
	}
	Clear()
	third, err := ReadWav(path)
	if err != nil || third == second {
		t.Fatal("clear retained the source")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWav(path); err == nil {
		t.Fatal("deleted source remained readable")
	}
}

func TestCacheCountsMonoMemoryAndSerializesConcurrentLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	writeFixture(t, path, 100)
	c := newCache(400 + 800)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, wave, err := c.readMono(path)
			if err != nil || len(wave) != 100 {
				t.Errorf("concurrent read: %v", err)
			}
		}()
	}
	workers.Wait()
	if c.bytes != 1200 || c.order.Len() != 1 {
		t.Fatalf("shared allocation counted repeatedly: bytes=%d entries=%d", c.bytes, c.order.Len())
	}
	c.maxBytes = 1199
	c.evict()
	if c.bytes != 0 || c.order.Len() != 0 {
		t.Fatal("mono memory was not included in eviction")
	}
}
