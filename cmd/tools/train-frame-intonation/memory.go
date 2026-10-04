package main

import (
	"fmt"
	"github.com/yh2237/gograd/cuda"
)

func logDeviceMemory(epoch int) {
	s := cuda.MemoryStats()
	fmt.Printf("cuda_memory epoch=%d live=%d peak_live=%d cached=%d reserved=%d peak_reserved=%d cache_limit=%d\n", epoch, s.LiveBytes, s.PeakLiveBytes, s.CachedBytes, s.ReservedBytes, s.PeakReservedBytes, s.CacheLimitBytes)
}
