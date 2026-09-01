package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rssSampler polls process memory in the background. Peak RSS is the number that
// matters for a sidecar-style deployment, and it is invisible to Go's MemStats:
// the runtime can hold freed heap without returning it to the OS, so a rewrite that
// halves allocations may not halve the footprint.
type rssSampler struct {
	done     chan struct{}
	wg       sync.WaitGroup
	peak     uint64
	last     uint64
	peakHeap uint64
}

func newRSSSampler() *rssSampler {
	s := &rssSampler{done: make(chan struct{})}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				s.sample()
				return
			case <-ticker.C:
				s.sample()
			}
		}
	}()
	return s
}

func (s *rssSampler) sample() {
	if rss, ok := readRSS(); ok {
		s.last = rss
		if rss > s.peak {
			s.peak = rss
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.HeapInuse > s.peakHeap {
		s.peakHeap = m.HeapInuse
	}
}

func (s *rssSampler) stop() {
	close(s.done)
	s.wg.Wait()
}

// readRSS returns the resident set size in bytes. It reads /proc/self/statm, which
// only exists on Linux; elsewhere the RSS fields are reported as zero rather than
// failing the run.
func readRSS() (uint64, bool) {
	raw, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 2 {
		return 0, false
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return pages * uint64(os.Getpagesize()), true
}
