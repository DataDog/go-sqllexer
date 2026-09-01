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
| A7 | No divergence under continuous differential fuzzing | `FuzzParity` runs both implementations in one process | met — 59.4M executions, 0 divergences |
| A8 | The cgo binding agrees with the pure Rust core and with Go, in all four modes | `ffirunner` (tokenize, obfuscate, normalize, obfuscate+normalize all routed through cgo) diffed against `gorunner` over every corpus | met — 0 mismatches over 21,590 requests |
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
| B4 | No leaks or undefined behavior under sanitizers | `cargo test` under ASan/LSan, plus a long `ffirunner` run under valgrind | **pending** |
| B5 | Clippy clean, `cargo fmt` clean, `go vet` clean with and without the `rustffi` tag | CI-equivalent commands | met |

## C. Performance

Gates were deliberately not fixed before there was data. They are ratified from the
measured baseline in [`reports/`](reports/README.md) and are expressed as ratios
against the Go implementation on the same corpus, same worker count, same host.

| # | Gate | Rationale | Measured |
| --- | --- | --- | --- |
| C1 | Rust native ≥ 1.8× Go throughput on the mixed workload corpus | the rewrite has to be worth the integration cost | 2.06× (1 worker), 2.19× (8 workers) |
| C2 | Rust through cgo ≥ 1.3× Go throughput on the mixed workload corpus | the FFI tax must not eat the win | 1.54× (1 worker), 1.43× (8 workers) |
| C3 | p50 and p99 not worse than Go in any workload class, native or through cgo | a throughput win that regresses tails is not a win | met in every class |
| C4 | Short statements (≤256B) at least at parity through cgo | this is where per-call overhead dominates and cgo is most likely to lose | p50 1.025µs vs Go 1.245µs through cgo; 0.684µs native |
| C5 | Allocations per statement ≤ Go, native and through cgo | 11.2 allocs/op today | 3.63 native, 7.21 through cgo |
| C6 | Bytes allocated per statement ≤ 50% of Go | 905 B/op today | 26 B/op native (2.9%), 526 B/op through cgo (58% — **see note**) |
| C7 | Steady-state RSS ≤ Go | GC-less memory behavior is a headline claim | 6MB vs 39MB at 8 workers |
| C8 | No workload class regresses by >5% between two runs of the same implementation | protects against silent drift once the gates are green | tracked per run in `reports/` |

Note on C6: through cgo, the 526 B/op is entirely Go-side — the output string and
metadata slices copied out of Rust memory. The Rust side allocates 26 B/op. Closing
that gap means changing the binding (returning into caller-provided buffers), not
the port, and it is the obvious next optimization if the cgo path is the one that
ships.

C1–C7 are met on the measurement host described in `reports/README.md`. They are
**not yet ratified on the authoritative hardware**: the agreed arbiter is the
dedicated `arm-8core-linux` runner used by the benchmark workflow, and every number
here comes from an 8-vCPU x86_64 cloud VM. Re-run before treating the gates as
final; the ratios are large enough that the verdict is unlikely to change, but the
margins on C4 are not.

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

# C1-C8
harness/reports/run.sh
```
