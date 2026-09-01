# Acceptance criteria for the Rust rewrite

What the Rust implementation has to prove before it can be considered a faithful,
faster equivalent of the Go core, and how each item is measured. Everything here is
checkable from the shell; nothing depends on judgement about whether the port
"looks" faithful.

The Go implementation is the oracle. Existing tests and `testdata/` are frozen: a
disagreement is a bug in the port, never a reason to edit a fixture.

**Scope.** The two implementations are validated as parallel implementations of the
same specification. Consuming the Rust core *from* Go — cgo, a binding, a shared
library — is explicitly a later phase, so no criterion here includes integration
overhead. See [section D](#d-what-is-explicitly-out-of-scope).

## A. Correctness

| # | Criterion | How it is checked | Status |
| --- | --- | --- | --- |
| A1 | Byte-identical normalized SQL for every fixture and every supported option/DBMS combination | `differ` over `testdata.jsonl` and `matrix.jsonl` | met — 0 mismatches over 20,441 requests |
| A2 | Metadata values identical and deduplicated, `size` identical | same runs; the differ compares values as sets and reports order-only differences separately | met — 0 mismatches, 0 order-only differences |
| A3 | Token stream identical (type and value) in `tokenize` mode | `matrix.jsonl` includes tokenize requests across all DBMS modes | met |
| A4 | Same error / no-error outcome | compared per request by the differ | met |
| A5 | Arbitrary bytes accepted, including invalid UTF-8, with byte-exact output | corpora carry non-UTF-8 through the `{"b64": …}` encoding | met — `pathological.jsonl`, `fuzzseeds.jsonl` |
| A6 | Truncated and malformed input (unterminated strings, comments, identifiers, dollar-quoted bodies) behaves identically | `pathological.jsonl` | met — 0 mismatches over 288 requests |
| A7 | No divergence under continuous differential fuzzing | `FuzzParity` answers each generated input from the Go oracle in-process and from the Rust runner over the protocol | met — 2.6M executions locally, plus the CI runs |
| A8 | Parity holds on both architectures | every corpus is re-diffed on the ARM and x86 runners before their benchmark matrices | met — `.github/workflows/harness-throughput.yml` |
| A9 | The frozen Go suite still passes unmodified | `go test ./...` | met |

Corpora used (regenerate with `harness/cmd/corpusgen`):

| Corpus | Requests | Purpose |
| --- | --- | --- |
| `testdata.jsonl` | 441 | Every repository fixture with that fixture's exact config |
| `workloads.jsonl` | 199 | SQL extracted from the benchmark files |
| `pathological.jsonl` | 288 | Unterminated constructs, deep nesting, huge parameter lists, invalid UTF-8, empty/comment-only |
| `matrix.jsonl` | 20,000 | Seeded sample of 2^7 obfuscator × 2^9 normalizer × 6 DBMS × 4 modes |
| `fuzzseeds.jsonl` | 662 | Inputs imported from the Go fuzz corpus |

Keyword parity is checked separately and exactly: `harness/keywordparity` compares
every table the port had to transcribe (21 commands, 78 keywords, the table
indicators, booleans, nulls, procedures, CTE and alias indicators) against the Go
source, so a table can never drift silently between the two.

## B. Safety

| # | Criterion | How it is checked | Status |
| --- | --- | --- | --- |
| B1 | No undefined behavior in the Rust core | `cargo +nightly miri test -p sqllexer` over the parity suite | met — 18 tests, clean |
| B2 | No panic on any input the library accepts, including malformed and non-UTF-8 | every corpus and the fuzzer would surface a panic as a runner crash | met |
| B3 | The library crate contains no `unsafe` | `rg unsafe rust/sqllexer` | met — the only `unsafe` in the workspace is the benchmark driver's allocation counter and its `sysconf` call, neither of which is library code |
| B4 | Clippy clean, `cargo fmt` clean, `go vet` clean | CI-equivalent commands | met |

There is no C ABI to audit any more: with the cgo binding out of scope, the Rust
side is a plain safe-Rust crate and the harness talks to it over a pipe. The
sanitizer/valgrind audit of the previous FFI boundary was removed along with the
binding; it is in the branch history if the integration phase revives it, and its
one real finding (a shared handle used from two goroutines corrupting results) was
a property of that binding, not of the core.

## C. Performance

Gates were deliberately not fixed before there was data. They are ratified from the
measured baseline in [`reports/`](reports/README.md) and are expressed as ratios
against the Go implementation **on the same host, same corpus, same worker count,
same run**. Absolute throughput is not comparable across machines; the ratio is.

Both architectures count. A gate holds only if it holds on ARM *and* x86 — see
[`reports/CROSS-PLATFORM.md`](reports/CROSS-PLATFORM.md). Worker counts follow each
host's core count so that neither side is measured oversubscribed.

<!-- Ratified from the ARM and x86 CI matrix; see reports/CROSS-PLATFORM.md. -->

| # | Gate | Rationale |
| --- | --- | --- |
| C1 | Rust ≥ 1.8× Go throughput on the mixed workload corpus, at 1 worker and at core-count workers | the rewrite has to be worth maintaining a second implementation |
| C2 | Rust ≥ 1.5× Go throughput on the pathological corpus | the win must not depend on well-formed input |
| C3 | p50 and p99 not worse than Go in any workload class | a throughput win that regresses tails is not a win |
| C4 | Short statements (≤256B) ≥ 1.3× Go | this is the class where per-statement fixed costs dominate |
| C5 | Bytes allocated per statement ≤ 25% of Go on the mixed workload corpus | the allocation profile is the headline claim, and Go is at 905 B/op |
| C6 | Allocations per statement ≤ Go on the mixed workload corpus | counting allocations is only meaningful where sizes are comparable — see the pathological note below |
| C7 | Steady-state RSS ≤ Go | a GC-less implementation that used more memory would be a bad trade |
| C8 | No workload class regresses by >5% in throughput between two runs on the same host | protects against silent drift once the gates are green |

Two measurement rules that came out of the earlier rounds and still apply:

- **Allocation counts are not comparable on the pathological corpus.** Rust reports
  ~43 allocs/op there against Go's ~9, while allocating ~266 B/op against Go's
  ~77,300: an arena-style implementation grows reusable buffers in many small steps
  where Go takes a few huge ones. Total bytes is the honest measure in that class,
  which is why C6 is scoped to the mixed corpus.
- **Tail latency beyond p99 is not gateable at n=3.** Run-to-run drift on p999 has
  been observed at up to 80% on identical code. C3 stops at p99 deliberately.

## D. What is explicitly out of scope

- **Consuming the Rust core from Go.** cgo, the reusable-handle binding, its
  zero-copy variants and the sanitizer audit of that boundary were removed from
  this branch so that every number here is Rust-vs-Go and nothing else. That work
  is in the branch history and is the input to a later integration decision, which
  brings its own questions: FFI overhead per statement, handle lifetime, and the
  fact that a shared handle is not safe to use from two goroutines while the Go
  `Obfuscator`/`Normalizer` are.
- **`CGO_ENABLED=0`** and any pure-Go-fallback question, for the same reason.
- **Shipping the Rust CLI or the harness runners** as supported artifacts. They
  exist to produce this evidence.
- **Production-traffic shadow validation.** No sanitized production corpus was
  available, so the synthetic generator (seeded from testdata and the fuzz corpus)
  stands in for it. This is the weakest evidence in the set and worth revisiting if
  a real sample can be sourced.

## E. Reproducing the whole thing

```sh
# corpora
go run ./harness/cmd/corpusgen -out harness/corpus \
  -fuzz-corpus "$(go env GOCACHE)/fuzz/github.com/DataDog/go-sqllexer/FuzzObfuscatorAndNormalizer"

# build the Rust side
(cd rust && cargo build --release)

# A1-A6, A8: the Rust core against the Go oracle
go build -o /tmp/gorunner ./harness/cmd/gorunner
for c in testdata workloads pathological matrix fuzzseeds; do
  go run ./harness/cmd/differ -corpus harness/corpus/$c.jsonl \
    -reference /tmp/gorunner \
    -candidate rust/target/release/sqllexer-runner
done

# A7: continuous differential fuzzing
go test -run xxx -fuzz FuzzParity -fuzztime 10m ./harness/cmd/gorunner/

# A9 + keyword tables
go test ./...

# B1-B4
cargo +nightly miri test -p sqllexer   # in rust/
cargo clippy --all-targets --all-features -- -D warnings
cargo fmt --all --check
go vet ./...

# C1-C8. The published matrix is DURATION=60s WARMUP=10s, three full runs, on both
# the arm-8core-linux runner and an x86_64 runner, via
# .github/workflows/harness-throughput.yml.
DURATION=60s WARMUP=10s harness/reports/run.sh
python3 harness/reports/summarize.py harness/reports/ci-arm
```
