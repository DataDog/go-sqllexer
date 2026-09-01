package latency

import (
	"math/rand"
	"sort"
	"testing"
)

func TestQuantilesAreWithinBucketError(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var h Histogram
	samples := make([]uint64, 0, 200000)
	for i := 0; i < 200000; i++ {
		// A realistic shape: mostly sub-microsecond with a long tail.
		ns := uint64(rng.ExpFloat64() * 800)
		if i%1000 == 0 {
			ns *= 500
		}
		h.Add(ns)
		samples = append(samples, ns)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	for _, q := range []float64{0.5, 0.9, 0.99, 0.999} {
		want := samples[int(float64(len(samples)-1)*q)]
		got := h.Quantile(q)
		// Buckets round up, and the error is bounded by 1/1024 of the value.
		if got < want || float64(got-want) > float64(want)/1024+1 {
			t.Errorf("q%.3f: got %d, want %d (+0.1%%)", q, got, want)
		}
	}
	if h.Count() != uint64(len(samples)) {
		t.Errorf("count = %d, want %d", h.Count(), len(samples))
	}
	if h.Max() != samples[len(samples)-1] {
		t.Errorf("max = %d, want %d", h.Max(), samples[len(samples)-1])
	}
}

func TestMergeIsAdditive(t *testing.T) {
	var a, b, both Histogram
	for i := uint64(1); i <= 1000; i++ {
		a.Add(i)
		both.Add(i)
	}
	for i := uint64(5000); i <= 6000; i++ {
		b.Add(i)
		both.Add(i)
	}
	a.Merge(&b)
	if a.Count() != both.Count() || a.Max() != both.Max() {
		t.Fatalf("merged count/max = %d/%d, want %d/%d", a.Count(), a.Max(), both.Count(), both.Max())
	}
	for _, q := range []float64{0.1, 0.5, 0.99} {
		if a.Quantile(q) != both.Quantile(q) {
			t.Errorf("q%.2f: merged %d, direct %d", q, a.Quantile(q), both.Quantile(q))
		}
	}
}

func TestEmptyHistogram(t *testing.T) {
	var h Histogram
	if h.Quantile(0.5) != 0 || h.Mean() != 0 || h.Count() != 0 {
		t.Fatal("empty histogram should report zeros")
	}
}
