// Package latency provides the fixed-size latency histogram shared by the Go and
// Rust load drivers.
//
// Keeping every sample makes RSS a function of how many operations the run
// completed, which quietly turns "which implementation uses less memory" into
// "which implementation was slower". The histogram is a few hundred KB regardless
// of run length, so the memory numbers describe the library instead of the
// harness.
package latency

import "math/bits"

// Buckets are log-linear: exact below subBucketCount nanoseconds, then
// subBucketCount buckets per power of two. That is ~0.1% worst-case error, far
// below the run-to-run noise the gates are read against. 24 magnitudes reach ~17s,
// well past the slowest statement; anything beyond lands in the last bucket and is
// still exact in Max.
const (
	subBucketBits  = 10
	subBucketCount = 1 << subBucketBits
	bucketCount    = subBucketCount * 24
)

// Histogram records nanosecond latencies. It is not safe for concurrent use; each
// worker owns one and they are merged at the end.
type Histogram struct {
	counts [bucketCount]uint64
	total  uint64
	max    uint64
}

func index(ns uint64) int {
	if ns < subBucketCount {
		return int(ns)
	}
	magnitude := bits.Len64(ns) - 1
	shift := magnitude - subBucketBits
	return (shift+1)*subBucketCount + int((ns>>shift)-subBucketCount)
}

func value(i int) uint64 {
	if i < subBucketCount {
		return uint64(i)
	}
	shift := i/subBucketCount - 1
	// The bucket's upper bound, so percentiles never read low.
	return (uint64(i%subBucketCount) + subBucketCount + 1) << shift
}

// Add records one sample.
func (h *Histogram) Add(ns uint64) {
	i := index(ns)
	if i >= bucketCount {
		i = bucketCount - 1
	}
	h.counts[i]++
	h.total++
	if ns > h.max {
		h.max = ns
	}
}

// Merge folds another histogram into this one.
func (h *Histogram) Merge(other *Histogram) {
	for i, c := range other.counts {
		h.counts[i] += c
	}
	h.total += other.total
	if other.max > h.max {
		h.max = other.max
	}
}

// Count returns the number of recorded samples.
func (h *Histogram) Count() uint64 { return h.total }

// Max returns the largest recorded sample, exactly.
func (h *Histogram) Max() uint64 { return h.max }

// Mean returns the average, computed from bucket bounds.
func (h *Histogram) Mean() float64 {
	if h.total == 0 {
		return 0
	}
	var sum float64
	for i, c := range h.counts {
		if c != 0 {
			sum += float64(value(i)) * float64(c)
		}
	}
	return sum / float64(h.total)
}

// Quantile returns the value at q (0..1).
func (h *Histogram) Quantile(q float64) uint64 {
	if h.total == 0 {
		return 0
	}
	target := uint64(float64(h.total-1) * q)
	var seen uint64
	for i, c := range h.counts {
		seen += c
		if seen > target {
			if v := value(i); v < h.max {
				return v
			}
			return h.max
		}
	}
	return h.max
}
