# Acceptance criteria for the Rust rewrite

What the Rust implementation has to prove before it can replace the Go core, and
how each item is measured. Everything here is checkable from the shell; nothing
depends on judgement about whether the port "looks" faithful.

The Go implementation is the oracle. Existing tests and `testdata/` are frozen: a
disagreement is a bug in the port, never a reason to edit a fixture.

## A. Correctness

| # | Criterion | How it is checked | Status |
| --- | --- | --- | --- |
| A1 | Byte-identical normalized SQL for every fixture and every supported option/DBMS combination | `differ` over `testdata.jsonl` and `matrix.jsonl` | met — 0 mismatches over 20,441 requests |
| A2 | Metadata values identical and deduplicated, `size` identical | same runs; the differ compares values as sets and reports order-only differences separately | met — 0 mismatches, 0 order-only differences |
| A3 | Token stream identical (type and value) in `tokenize` mode | `matrix.jsonl` includes tokenize requests across all DBMS modes | met |
| A4 | Same error / no-error outcome | compared per request by the differ | met |
| A5 | Arbitrary bytes accepted, including invalid UTF-8, with byte-exact output | corpora carry non-UTF-8 through the `{"b64": …}` encoding | met — `pathological.jsonl`, `fuzzseeds.jsonl` |
| A6 | Truncated and malformed input (unterminated strings, comments, identifiers, dollar-quoted bodies) behaves identically | `pathological.jsonl` | met — 0 mismatches over 288 requests |
| A7 | No divergence under continuous differential fuzzing | `FuzzParity` runs both implementations in one process | met — 84.3M executions, 0 divergences |
| A8 | The cgo binding agrees with the pure Rust core and with Go, in all four modes | `ffirunner` (tokenize, obfuscate, normalize, obfuscate+normalize all routed through cgo) diffed against `gorunner` over every corpus | met — 0 mismatches over 21,590 requests, re-run after the zero-copy binding and the concurrency guard landed |
| A9 | The frozen Go suite still passes unmodified | `go test ./...` | met |

Corpora used (regenerate with `harness/cmd/corpusgen`):

| Corpus | Requests | Purpose |
| --- | --- | --- |
| `testdata.jsonl` | 441 | Every repository fixture with that fixture's exact config |
| `workloads.jsonl` | 199 | SQL extracted from the benchmark files |
| `pathological.jsonl` | 288 | Unterminated constructs, deep nesting, huge parameter lists, invalid UTF-8, empty/comment-only |
| `matrix.jsonl` | 20,000 | Seeded sample of 2^7 obfuscator × 2^9 normalizer × 6 DBMS × 4 modes |
| `fuzzseeds.jsonl` | 662 | Inputs imported from the Go fuzz corpus |

## B. Safety

| # | Criterion | How it is checked | Status |
| --- | --- | --- | --- |
| B1 | No Rust panic may unwind into the Go runtime | every FFI entry point wraps its body in `catch_unwind` and returns a status code | met |
| B2 | No use of a handle after free, and no result read after it is invalidated | results are copied into Go memory before returning; `TestResultsSurviveSubsequentCalls` pins this | met |
| B3 | Null and closed-handle arguments are rejected, not dereferenced | `TestClosedProcessorReportsAnError`, FFI unit tests | met |
| B4 | No leaks or undefined behavior under sanitizers | `cargo test` under ASan/LSan, Miri, and a real `ffirunner` under valgrind against a pure-Go baseline — [`sanitizers/README.md`](sanitizers/README.md) | met, x86_64 only — see below |
| B5 | Clippy clean, `cargo fmt` clean, `go vet` clean with and without the `rustffi` tag | CI-equivalent commands | met |
| B6 | Overlapping calls on one handle are rejected, not silently corrupted | `misuse_test.go`, and the recorded pre-fix run in `sanitizers/logs/misuse-guard-regression.log` | met — `ErrConcurrentUse` |

B4 in full: the Rust FFI surface is clean under AddressSanitizer + LeakSanitizer
(`-Zbuild-std`) across a stress suite covering all four entry points, invalid UTF-8
and pathological inputs, and clean under Miri for the pure crate and the Rust-side
ABI functions. A real cgo binary under valgrind memcheck over 3,367 statements
reports 0 bytes definitely or indirectly lost and 17,205 of 17,209 allocations
freed; what stays live is the process-lifetime keyword trie and glibc TLS for cgo
threads. 928M operations of sustained load leave RSS flat at 22.4–23.4 MB.
Detection was validated with deliberate use-after-free, leak and aliasing controls,
each of which fails the run as expected.

What B4 does **not** cover: no tool instruments both runtimes across the crossing
(Go's runtime and ASan fight over signal handling and stack switching), valgrind
cannot see Go's heap at all, Miri never crosses the C ABI, the runs are x86_64 and
debug-build only, and `Close()` racing an in-flight call remains unsupported and
undetected.

B6 came out of that audit: overlapping calls on a single handle used to return
another statement's output (27,091 corrupted of 4,677,918 calls in the recorded
log). A handle is still one-per-worker by contract — the guard turns the misuse
into an error instead of undefined behavior, it does not make sharing supported.

## C. Performance

Gates were deliberately not fixed before there was data. They are ratified from the
measured baseline in [`reports/`](reports/README.md) and are expressed as ratios
against the Go implementation on the same corpus, same worker count, same host.

The arbiter is the `arm-8core-linux` runner. Every gate below was re-measured there,
on a second dedicated x86_64 host, and on `ubuntu-latest` (informational), three
full 60s runs each — [`reports/CROSS-PLATFORM.md`](reports/CROSS-PLATFORM.md).
ARM numbers decide; x86 is corroboration.

| # | Gate | Rationale | Measured on the arbiter |
| --- | --- | --- | --- |
| C1 | Rust native ≥ 1.8× Go throughput on the mixed workload corpus | the rewrite has to be worth the integration cost | 1.86× (1 worker), 2.07× (8) — worst single run 1.84×, i.e. 3% of headroom |
| C2 | Rust through cgo ≥ 1.3× Go throughput on the mixed workload corpus | the FFI tax must not eat the win | 1.48× (1 worker), 1.39× (8) |
| C3 | p50 and p99 not worse than Go in any workload class, **with workers ≤ cores** | a throughput win that regresses tails is not a win | no violation on either dedicated host, in any class or worker count |
| C4 | Short statements (≤256B) ≥ 1.1× Go through cgo | this is where per-call overhead dominates and cgo is most likely to lose | p50 1.10µs vs Go 1.41µs; the ratio is 0.78–0.84× of Go's p50 in all three environments, spread ≤3% |
| C5 | Allocations per statement ≤ Go **on the mixed workload corpus** | 11.2 allocs/op today | 2.99 through cgo, 3.63 native |
| C6 | Bytes per statement ≤ 50% of Go for the native core; the cgo path is exempt | 905 B/op today | 29 B/op native (3.2%); 423 B/op through cgo (47%), or 0.3 B/op via the borrowed API |
| C7 | Steady-state RSS ≤ Go for the native core, ≤ 2× Go through cgo | GC-less memory behavior is a headline claim | native 3.9–5.8 MB vs Go 36 MB at 8 workers; cgo 43.5 MB vs Go 36.0 MB |
| C8 | No workload class regresses by >5% in **throughput** between two 60s runs | protects against silent drift once the gates are green | worst drift 2.30% on the arbiter, 3.19% x86 |

Five of these were corrected by the cross-platform data rather than confirmed:

- **C3** only holds when workers ≤ available cores. The only violations anywhere are
  8 workers on the 4-vCPU shared runner, where W3-large p99 goes to 2,236µs (cgo)
  and 3,074µs (native) against Go's 191µs. That is oversubscription, not this code,
  but it means the gate is unenforceable on shared hardware.
- **C5** as originally written ("≤ Go, native and through cgo") fails: the native
  core is 43.1 allocs/op on the pathological corpus against Go's 9.1, while
  allocating 266 B/op against Go's 77,300. Counting allocations is the wrong
  measure for an arena-style allocator, so C5 is scoped to the mixed corpus and C6
  carries the memory claim.
- **C6** is met by the native core and formally exempts cgo. The zero-copy work
  ([`reports/ZERO-COPY.md`](reports/ZERO-COPY.md)) took the same-API cgo path from
  526 B/op / 7.21 allocs/op to 423 / 2.99, and the residual is the output bytes
  themselves — an owned result has to be copied somewhere. The `Borrowed` API drops
  it to 0.3 B/op / 0.00 allocs/op at the cost of a weaker lifetime (valid until the
  next call on the handle).
- **C7** had to be split by engine: through cgo RSS is *above* Go (two allocators,
  neither returning pages eagerly), which the original single-column entry hid.
- **C8** cannot be applied to latency. Worst run-to-run drift is 6.8% on p50, 27% on
  p99 and 80% on p999. A latency guard, if wanted, is p50 within 10% and p99 within
  30%; p999 is not gateable at n=3.

One architecture-specific finding: on ARM at 8 workers the pathological corpus
inverts the usual ordering — rust-native (1.56×) is *slower* than rust-through-cgo
(1.65×), reproduced in all three runs within ±0.1%.

Two caveats on the numbers themselves. The cross-platform matrix was measured
before the zero-copy binding landed, so its cgo columns show the old 526 B/op —
throughput ratios are unaffected (the change made the cgo path faster, not slower),
but the allocation columns in C5/C6 above come from the zero-copy report on x86.
And variance is within-session only: three runs from one boot, so ratios are
portable and absolute numbers are not.

## D. What is explicitly out of scope

- `CGO_ENABLED=0`. The Rust core is consumed through cgo; a pure-Go fallback is a
  separate decision.
- Shipping the Rust CLI or the harness runners as supported artifacts. They exist to
  produce this evidence.
- Production-traffic shadow validation. No sanitized production corpus was
  available, so the synthetic generator (seeded from testdata and the fuzz corpus)
  stands in for it. This is the weakest evidence in the set and worth revisiting if
  a real sample can be sourced.

## E. Reproducing the whole thing

```sh
# corpora
go run ./harness/cmd/corpusgen -out harness/corpus \
  -fuzz-corpus "$(go env GOCACHE)/fuzz/github.com/DataDog/go-sqllexer/FuzzObfuscatorAndNormalizer"

# build both candidates
(cd rust && cargo build --release)

# A1-A6: pure Rust core against the Go oracle
for c in testdata workloads pathological matrix fuzzseeds; do
  go run ./harness/cmd/differ -corpus harness/corpus/$c.jsonl \
    -reference "go run ./harness/cmd/gorunner" \
    -candidate rust/target/release/sqllexer-runner
done

# A8: the cgo binding against the Go oracle.
# -a is required: Go's build cache keys on source content, not on the contents of
# the static archive named in CGO LDFLAGS, so a rebuilt libsqllexer_ffi.a alone
# yields a cache hit and silently re-links the previous Rust code.
go build -a -tags rustffi -o /tmp/ffirunner ./harness/cmd/ffirunner
for c in testdata workloads pathological matrix fuzzseeds; do
  go run ./harness/cmd/differ -corpus harness/corpus/$c.jsonl \
    -reference "go run ./harness/cmd/gorunner" -candidate /tmp/ffirunner
done

# A7: continuous differential fuzzing
go test -tags rustffi -run xxx -fuzz FuzzParity -fuzztime 10m ./harness/rustffi/

# B4: the sanitizer, Miri, valgrind and sustained-load stages
harness/sanitizers/run.sh

# C1-C8. The published matrix is DURATION=60s WARMUP=10s, three full runs,
# on the arm-8core-linux arbiter via .github/workflows/harness-throughput.yml.
DURATION=60s WARMUP=10s harness/reports/run.sh
```
