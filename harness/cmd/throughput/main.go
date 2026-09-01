// Command throughput is the tier-4 load harness: it measures the library the way a
// high-volume agent uses it — many statements processed concurrently and
// continuously — rather than one statement at a time in isolation.
//
// Per-op microbenchmarks hide the behavior that decides whether a rewrite is
// actually better under load: allocation rate, GC pressure, steady-state RSS, and
// tail latency. Those are what this harness reports, broken down by workload class
// so short statements and large statements are judged separately.
//
//	go run ./harness/cmd/throughput \
//	  -corpus harness/corpus/workloads.jsonl -workers 8 -duration 30s -json /tmp/tier4.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/internal/corpus"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// workloadClass buckets statements by size. The boundaries follow the shape of the
// existing benchmark corpus: short statements are dominated by per-call overhead,
// large ones by scanning throughput, and the pathological bucket exists to catch
// accidental super-linear behavior.
type workloadClass struct {
	name    string
	maxSize int
}

var classes = []workloadClass{
	{"W1-short", 256},
	{"W2-medium", 2048},
	{"W3-large", 16384},
	{"W4-pathological", 1 << 30},
}

func classify(size int) string {
	for _, c := range classes {
		if size <= c.maxSize {
			return c.name
		}
	}
	return classes[len(classes)-1].name
}

// classStats accumulates latencies for one workload class. Latencies are kept in
// full (not bucketed) so percentiles are exact; at ~1M ops that is a few MB.
type classStats struct {
	mu        sync.Mutex
	latencies []time.Duration
	bytes     int64
	ops       int64
}

func (s *classStats) add(d time.Duration, size int) {
	s.mu.Lock()
	s.latencies = append(s.latencies, d)
	s.bytes += int64(size)
	s.ops++
	s.mu.Unlock()
}

// Report is the machine-readable output, written with -json so runs can be diffed
// across implementations and over time.
type Report struct {
	Implementation string        `json:"implementation"`
	Corpus         string        `json:"corpus"`
	Workers        int           `json:"workers"`
	Duration       string        `json:"duration"`
	ReuseInstances bool          `json:"reuse_instances"`
	GOMAXPROCS     int           `json:"gomaxprocs"`
	TotalOps       int64         `json:"total_ops"`
	OpsPerSecond   float64       `json:"ops_per_second"`
	BytesPerSecond float64       `json:"bytes_per_second"`
	Classes        []ClassReport `json:"classes"`
	Memory         MemoryReport  `json:"memory"`
}

// ClassReport holds the per-workload-class latency picture.
type ClassReport struct {
	Class        string  `json:"class"`
	Ops          int64   `json:"ops"`
	OpsPerSecond float64 `json:"ops_per_second"`
	MeanNs       float64 `json:"mean_ns"`
	P50Ns        int64   `json:"p50_ns"`
	P90Ns        int64   `json:"p90_ns"`
	P99Ns        int64   `json:"p99_ns"`
	P999Ns       int64   `json:"p999_ns"`
	MaxNs        int64   `json:"max_ns"`
}

// MemoryReport captures allocation and GC behavior over the run. These are the
// numbers a Rust implementation has to beat on heap and allocation count, and the
// GC fields are the cost that disappears entirely on the Rust side.
type MemoryReport struct {
	BytesAllocatedTotal  uint64  `json:"bytes_allocated_total"`
	BytesPerOp           float64 `json:"bytes_per_op"`
	AllocsTotal          uint64  `json:"allocs_total"`
	AllocsPerOp          float64 `json:"allocs_per_op"`
	AllocRateMBPerSecond float64 `json:"alloc_rate_mb_per_second"`
	NumGC                uint32  `json:"num_gc"`
	GCPauseTotalMs       float64 `json:"gc_pause_total_ms"`
	GCCPUFraction        float64 `json:"gc_cpu_fraction"`
	HeapInUsePeakMB      float64 `json:"heap_in_use_peak_mb"`
	RSSPeakMB            float64 `json:"rss_peak_mb"`
	RSSFinalMB           float64 `json:"rss_final_mb"`
}

func main() {
	var (
		corpusPath = flag.String("corpus", "harness/corpus/workloads.jsonl", "JSONL corpus to replay")
		workers    = flag.Int("workers", runtime.GOMAXPROCS(0), "concurrent workers")
		duration   = flag.Duration("duration", 30*time.Second, "how long to sustain the load")
		warmup     = flag.Duration("warmup", 3*time.Second, "warmup period excluded from the measurements")
		reuse      = flag.Bool("reuse", true, "reuse one obfuscator/normalizer per worker instead of constructing per call")
		jsonOut    = flag.String("json", "", "optional path for the JSON report")
		label      = flag.String("label", "go", "implementation label recorded in the report")
	)
	flag.Parse()

	requests, err := corpus.Read(*corpusPath)
	if err != nil {
		log.Fatalf("read corpus: %v", err)
	}
	if len(requests) == 0 {
		log.Fatalf("corpus %s is empty", *corpusPath)
	}

	stats := map[string]*classStats{}
	for _, c := range classes {
		stats[c.name] = &classStats{}
	}

	// Warmup lets the allocator and (for the Go side) the GC reach steady state, so
	// the measured window reflects sustained behavior rather than startup.
	runLoad(requests, *workers, *warmup, *reuse, nil)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	peakRSS := newRSSSampler()
	start := time.Now()
	totalOps := runLoad(requests, *workers, *duration, *reuse, stats)
	elapsed := time.Since(start)
	peakRSS.stop()
	runtime.ReadMemStats(&after)

	report := buildReport(*label, *corpusPath, *workers, *duration, *reuse, totalOps, elapsed, stats, before, after, peakRSS)
	printReport(report)

	if *jsonOut != "" {
		f, err := os.Create(*jsonOut)
		if err != nil {
			log.Fatalf("create json report: %v", err)
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			log.Fatalf("write json report: %v", err)
		}
		fmt.Printf("\njson report: %s\n", *jsonOut)
	}
}

// runLoad drives the corpus in a round-robin across workers until the deadline.
// Passing nil stats runs the load without recording (used for warmup).
func runLoad(requests []protocol.Request, workers int, duration time.Duration, reuse bool, stats map[string]*classStats) int64 {
	var ops int64
	deadline := time.Now().Add(duration)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			var obfuscator *sqllexer.Obfuscator
			var normalizer *sqllexer.Normalizer
			if reuse {
				obfuscator, normalizer = defaultObfuscator(), defaultNormalizer()
			}

			// Workers start at different offsets so they do not march through the
			// corpus in lockstep and share cache lines on the same input.
			i := worker * len(requests) / workers
			for {
				if time.Now().After(deadline) {
					return
				}
				// Check the clock every 64 iterations: time.Now is cheap but not free
				// relative to a 600ns operation.
				for n := 0; n < 64; n++ {
					req := requests[i%len(requests)]
					i++

					o, nrm := obfuscator, normalizer
					if !reuse {
						o, nrm = defaultObfuscator(), defaultNormalizer()
					}

					var started time.Time
					if stats != nil {
						started = time.Now()
					}
					_, _, err := sqllexer.ObfuscateAndNormalize(string(req.SQL), o, nrm, sqllexer.WithDBMS(sqllexer.DBMSType(req.DBMS)))
					if err != nil {
						continue
					}
					if stats != nil {
						stats[classify(len(req.SQL))].add(time.Since(started), len(req.SQL))
						atomic.AddInt64(&ops, 1)
					}
				}
			}
		}(w)
	}
	wg.Wait()
	return atomic.LoadInt64(&ops)
}

func defaultObfuscator() *sqllexer.Obfuscator {
	return sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(true),
		sqllexer.WithReplaceBoolean(true),
		sqllexer.WithReplaceNull(true),
	)
}

func defaultNormalizer() *sqllexer.Normalizer {
	return sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
		sqllexer.WithCollectCommands(true),
		sqllexer.WithCollectComments(true),
	)
}

func buildReport(
	label, corpusPath string, workers int, duration time.Duration, reuse bool,
	totalOps int64, elapsed time.Duration, stats map[string]*classStats,
	before, after runtime.MemStats, rss *rssSampler,
) Report {
	seconds := elapsed.Seconds()
	report := Report{
		Implementation: label,
		Corpus:         corpusPath,
		Workers:        workers,
		Duration:       duration.String(),
		ReuseInstances: reuse,
		GOMAXPROCS:     runtime.GOMAXPROCS(0),
		TotalOps:       totalOps,
		OpsPerSecond:   float64(totalOps) / seconds,
	}

	var totalBytes int64
	for _, c := range classes {
		s := stats[c.name]
		if s.ops == 0 {
			continue
		}
		totalBytes += s.bytes
		sort.Slice(s.latencies, func(i, j int) bool { return s.latencies[i] < s.latencies[j] })

		var sum time.Duration
		for _, d := range s.latencies {
			sum += d
		}
		report.Classes = append(report.Classes, ClassReport{
			Class:        c.name,
			Ops:          s.ops,
			OpsPerSecond: float64(s.ops) / seconds,
			MeanNs:       float64(sum.Nanoseconds()) / float64(s.ops),
			P50Ns:        percentile(s.latencies, 0.50),
			P90Ns:        percentile(s.latencies, 0.90),
			P99Ns:        percentile(s.latencies, 0.99),
			P999Ns:       percentile(s.latencies, 0.999),
			MaxNs:        s.latencies[len(s.latencies)-1].Nanoseconds(),
		})
	}
	report.BytesPerSecond = float64(totalBytes) / seconds

	allocBytes := after.TotalAlloc - before.TotalAlloc
	allocCount := after.Mallocs - before.Mallocs
	report.Memory = MemoryReport{
		BytesAllocatedTotal:  allocBytes,
		BytesPerOp:           safeDiv(float64(allocBytes), float64(totalOps)),
		AllocsTotal:          allocCount,
		AllocsPerOp:          safeDiv(float64(allocCount), float64(totalOps)),
		AllocRateMBPerSecond: float64(allocBytes) / (1 << 20) / seconds,
		NumGC:                after.NumGC - before.NumGC,
		GCPauseTotalMs:       float64(after.PauseTotalNs-before.PauseTotalNs) / 1e6,
		GCCPUFraction:        after.GCCPUFraction,
		HeapInUsePeakMB:      float64(rss.peakHeap) / (1 << 20),
		RSSPeakMB:            float64(rss.peak) / (1 << 20),
		RSSFinalMB:           float64(rss.last) / (1 << 20),
	}
	return report
}

func printReport(r Report) {
	fmt.Printf("implementation  %s\n", r.Implementation)
	fmt.Printf("corpus          %s\n", r.Corpus)
	fmt.Printf("workers         %d (GOMAXPROCS=%d, reuse=%v)\n", r.Workers, r.GOMAXPROCS, r.ReuseInstances)
	fmt.Printf("throughput      %.0f ops/s  (%.1f MB/s of SQL)\n", r.OpsPerSecond, r.BytesPerSecond/(1<<20))
	fmt.Printf("total ops       %d\n\n", r.TotalOps)

	fmt.Printf("%-18s %10s %12s %10s %10s %10s %10s\n", "class", "ops", "ops/s", "p50", "p90", "p99", "p99.9")
	for _, c := range r.Classes {
		fmt.Printf("%-18s %10d %12.0f %10s %10s %10s %10s\n", c.Class, c.Ops, c.OpsPerSecond,
			dur(c.P50Ns), dur(c.P90Ns), dur(c.P99Ns), dur(c.P999Ns))
	}

	m := r.Memory
	fmt.Printf("\nallocation      %.0f B/op, %.1f allocs/op, %.1f MB/s allocated\n", m.BytesPerOp, m.AllocsPerOp, m.AllocRateMBPerSecond)
	fmt.Printf("gc              %d cycles, %.1f ms total pause, %.2f%% of CPU\n", m.NumGC, m.GCPauseTotalMs, m.GCCPUFraction*100)
	fmt.Printf("memory          heap peak %.1f MB, RSS peak %.1f MB, RSS final %.1f MB\n", m.HeapInUsePeakMB, m.RSSPeakMB, m.RSSFinalMB)
}

func percentile(sorted []time.Duration, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)-1) * p)
	return sorted[i].Nanoseconds()
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func dur(ns int64) string {
	return time.Duration(ns).String()
}
