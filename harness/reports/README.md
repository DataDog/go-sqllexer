# Measured baseline

Three engines, one corpus, one host, sequential runs. Reproduce with
[`run.sh`](run.sh); the JSON files next to it are the raw output.

- **go** — the current implementation, one obfuscator/normalizer pair per worker.
- **rust** — the same Rust core reached through cgo, one handle per worker.
- **rust-native** — the Rust core with no FFI in the path.

Host: 8 vCPU Intel Xeon Platinum 8559C, 31 GB, Linux x86_64; Go 1.25.7, Rust 1.98
release profile (`lto=fat`, `codegen-units=1`), system allocator on both sides.
20s measured after 5s warmup. This is **not** the authoritative hardware — the
agreed arbiter is the dedicated `arm-8core-linux` runner — so treat the ratios as
solid and the last few percent as noise.

Latency percentiles come from a fixed-size log-linear histogram (~0.1% bucket
error) rather than stored samples, in both drivers. That matters for more than
memory footprint: keeping every sample made the harness's own RSS a function of how
many operations an implementation completed, which turned the faster engine into
the one that appeared to use more memory.

## Mixed workload corpus (`workloads.jsonl`, 199 statements from the benchmark files)

| Engine | ops/s (1 worker) | ops/s (8 workers) | B/op | allocs/op | GC cycles | RSS peak |
| --- | --- | --- | --- | --- | --- | --- |
| go | 269,682 | 1,729,014 | 905 | 11.21 | 2,104 | 39 MB |
| rust (cgo) | 415,374 (1.54×) | 2,473,473 (1.43×) | 526 | 7.21 | 1,587 | 60 MB |
| rust-native | 554,752 (2.06×) | 3,783,176 (2.19×) | 26 | 3.63 | none | 6 MB |

Latency by workload class, 8 workers:

| Class | go p50 / p99 | rust (cgo) p50 / p99 | rust-native p50 / p99 |
| --- | --- | --- | --- |
| W1-short (≤256B) | 1.245µs / 4.948µs | 1.025µs / 3.482µs | 0.684µs / 4.668µs |
| W2-medium (≤2KB) | 7.340µs / 25.632µs | 4.816µs / 14.392µs | 4.200µs / 12.048µs |
| W3-large (≤16KB) | 46.240µs / 182.528µs | 25.120µs / 41.920µs | 24.672µs / 30.704µs |

## Pathological corpus (`pathological.jsonl`)

| Engine | ops/s (8 workers) | B/op | allocs/op | RSS peak |
| --- | --- | --- | --- | --- |
| go | 17,170 | 77,288 | 9.12 | 64 MB |
| rust (cgo) | 28,724 (1.67×) | 18,957 | 5.81 | 63 MB |
| rust-native | 31,905 (1.86×) | 291 | 43.07 | 19 MB |

## Reading the numbers

**The FFI tax is real and bounded.** Native is 2.06–2.19× Go; through cgo it is
1.43–1.54×. The gap is per-call: the cgo transition plus copying the result into Go
memory, which is why it costs proportionally most on short statements (native p50
0.684µs vs 1.025µs through cgo) and almost nothing on large ones (24.7µs vs
25.1µs). A batched binding would recover most of it, at the cost of an API that no
longer looks like `ObfuscateAndNormalize`.

**Allocation, not scanning, is where Go loses.** 11.21 allocs/op and 905 B/op
against 3.63 and 26 for the reusable Rust handle. Through cgo the 526 B/op is
entirely on the Go side — the output string and the metadata slices copied out of
Rust-owned memory — so it is a property of the binding, not the port.

**The GC cost shows up as tail latency, not throughput.** At 8 workers the Go run
does 2,104 collections; its W3-large p999 is 1.457ms against 34.8µs native. That
ratio, not the mean, is the argument for the rewrite in a latency-sensitive agent.

**Pathological allocations invert.** rust-native reports 43 allocs/op there against
Go's 9.12 — an artifact of very large statements repeatedly growing the reusable
buffers and of obfuscation materializing replaced token values, while Go allocates
far fewer but far larger blocks (77KB/op against 291B/op). Total bytes is the
honest comparison in this class, and the Rust side allocates ~265× less.
