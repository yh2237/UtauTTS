package sourceaudio

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"utautts/internal/audio"
)

func writeBenchmarkSources(b *testing.B, count int) []string {
	b.Helper()
	dir := b.TempDir()
	paths := make([]string, count)
	for i := range paths {
		pcm := &audio.PCM{SampleRate: 48000, Channels: 2, Data: make([]int16, 48000*2)}
		for j := range pcm.Data {
			pcm.Data[j] = int16((j*31 + i*7) % 20000)
		}
		paths[i] = filepath.Join(dir, fmt.Sprintf("source-%d.wav", i))
		if err := audio.WriteWav(paths[i], pcm); err != nil {
			b.Fatal(err)
		}
	}
	return paths
}

// 別々の原音を同時に初回読込する。音源の事前検査と合成が重なる場合を想定する。
func BenchmarkCacheConcurrentMiss(b *testing.B) {
	paths := writeBenchmarkSources(b, 8)
	b.ReportAllocs()
	for b.Loop() {
		c := newCache(256 << 20)
		var group sync.WaitGroup
		for _, path := range paths {
			group.Add(1)
			go func() {
				defer group.Done()
				if _, _, err := c.readMono(path); err != nil {
					b.Error(err)
				}
			}()
		}
		group.Wait()
	}
}

// 他の原音の初回読込が続く間に、読込済みの原音を取得する。
func BenchmarkCacheHitDuringMiss(b *testing.B) {
	paths := writeBenchmarkSources(b, 9)
	c := newCache(256 << 20)
	if _, err := c.load(paths[0]); err != nil {
		b.Fatal(err)
	}
	stop := make(chan struct{})
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			path := paths[1+i%8]
			c.mu.Lock()
			if element := c.byPath[filepath.Clean(path)]; element != nil {
				c.remove(element)
			}
			c.mu.Unlock()
			if _, _, err := c.readMono(path); err != nil {
				b.Error(err)
				return
			}
		}
	}()
	b.ResetTimer()
	for b.Loop() {
		if _, err := c.load(paths[0]); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(stop)
	group.Wait()
}
