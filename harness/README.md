# Rust vs Go validation harness

Evidence that the Rust rewrite of the lexer/obfuscator/normalizer is a faithful and
faster equivalent of the Go one. The two are compared as **parallel
implementations**: same inputs, same outputs, same measurements, no binding between
them. How the Rust core would eventually be consumed from Go (cgo or otherwise) is
a separate decision and deliberately out of scope — it would put integration
overhead inside every number here.

Nothing in `harness/` is part of the library: it is additive, the existing tests and
`testdata/` are never modified, and the Go implementation is the oracle.

Results: [`reports/benchmarks.html`](reports/benchmarks.html) (side by side, ARM and
x86). Gates: [ACCEPTANCE.md](ACCEPTANCE.md).

## Reproduce the feature parity tests

Every command below runs from the repository root and needs only Go and a stable
Rust toolchain. Copy-paste the block; it is the whole parity suite.

```sh
# 1. build both sides
(cd rust && cargo build --release)
go build -o /tmp/gorunner ./harness/cmd/gorunner

# 2. generate the corpora (~26MB, written to harness/corpus/)
go run ./harness/cmd/corpusgen -out harness/corpus

# 3. diff the Rust implementation against the Go oracle, corpus by corpus
for c in testdata workloads pathological matrix fuzzseeds; do
  [ -f "harness/corpus/$c.jsonl" ] || continue   # fuzzseeds is optional, see below
  go run ./harness/cmd/differ \
    -corpus "harness/corpus/$c.jsonl" \
    -reference /tmp/gorunner \
    -candidate rust/target/release/sqllexer-runner \
    -report "/tmp/mismatches-$c.jsonl"
done
```

Each corpus prints `mismatches 0`; the driver exits non-zero on the first corpus
that does not, and `/tmp/mismatches-$c.jsonl` holds the offending request/response
triples, replayable straight back through either runner.

`fuzzseeds.jsonl` only exists if a Go fuzz corpus has been generated locally. To
include it:

```sh
go test -run xxx -fuzz FuzzObfuscatorAndNormalizer -fuzztime 60s .
go run ./harness/cmd/corpusgen -out harness/corpus \
  -fuzz-corpus "$(go env GOCACHE)/fuzz/github.com/DataDog/go-sqllexer/FuzzObfuscatorAndNormalizer"
```

Three more checks complete the correctness picture:

```sh
# differential fuzzing: Go in-process vs the Rust runner over a pipe
go test -run xxx -fuzz FuzzParity -fuzztime 10m ./harness/cmd/gorunner/

# the keyword tables the port transcribed by hand, compared against the Go source
go test ./harness/keywordparity/

# the frozen library suite, unmodified
go test ./...
```

| Corpus | Requests | Contents |
| --- | --- | --- |
| `testdata.jsonl` | 441 | One request per fixture output, with that fixture's exact config |
| `workloads.jsonl` | 199 | SQL literals extracted from the benchmark files by parsing their AST |
| `pathological.jsonl` | 288 | Huge parameter lists, deep nesting, unterminated constructs, invalid UTF-8, comment-only, empty — across every DBMS and mode |
| `matrix.jsonl` | 20,000 | Seeded sample of the option space: 2^7 obfuscator × 2^9 normalizer × 6 DBMS × 4 modes |
| `fuzzseeds.jsonl` | 662 | Inputs imported from a Go fuzz corpus directory |

## Reproduce the benchmarks

`run.sh` builds both drivers and runs them back to back on the current host, once
per worker count and corpus. Runs are sequential by construction: two load drivers
on one host would measure each other.

```sh
# one matrix into harness/reports/local/ (~8 minutes at these settings)
DURATION=60s WARMUP=10s OUT=harness/reports/local harness/reports/run.sh

# the published evidence is three repeats per host
for i in 1 2 3; do
  DURATION=60s WARMUP=10s OUT=harness/reports/local/run$i harness/reports/run.sh
done

# tables, and the side-by-side HTML artifact
python3 harness/reports/summarize.py harness/reports/local
python3 harness/reports/summarize.py --html harness/reports/benchmarks.html \
  harness/reports/ci-arm harness/reports/ci-x86
```

Worker counts default to 1 and the host's core count (`WORKERS="1 4"` to override),
so a 4-vCPU runner is never asked to run 8 workers while an 8-core one runs the
same 8 — comparing an oversubscribed host with a saturated one measures the
scheduler, not the library. Only the Go-vs-Rust ratio on one host is comparable
across machines; absolute ops/s is not.

Both architectures are produced by
[`harness-throughput.yml`](../.github/workflows/harness-throughput.yml), which
re-proves parity on the runner before measuring it and uploads the raw JSON plus
the HTML report as artifacts. Committed raw output is in
[`reports/ci-arm/`](reports/ci-arm) and [`reports/ci-x86/`](reports/ci-x86).

## How it works

Every implementation under validation exposes the same interface: a binary that
reads [`protocol.Request`](internal/protocol/protocol.go) objects as
newline-delimited JSON on stdin and writes one `protocol.Response` per line to
stdout. That is the entire coupling between the harness and an implementation, so
the Rust core needs no Go bindings to be validated.

Two details of the wire format are load-bearing:

- **Text may be non-UTF-8.** The library accepts arbitrary bytes, and JSON encoders
  replace invalid UTF-8 with U+FFFD. `protocol.Text` encodes such values as
  `{"b64": "..."}` instead, so a fuzz-derived input reaches both implementations
  unchanged.
- **Configs are always explicit.** The fixture suite's implicit defaults are not the
  library's defaults (`dbms_test.go` enables `dollar_quoted_func`,
  `replace_positional_parameter` and `collect_procedure`), so generated corpora
  materialize every option rather than letting each implementation fill in its own.

| Tool | Purpose |
| --- | --- |
| `harness/cmd/corpusgen` | Builds the corpora from testdata, benchmark literals, generated pathological inputs, a sampled config matrix, and an optional Go fuzz corpus |
| `harness/cmd/gorunner` | The Go reference implementation of the protocol |
| `harness/cmd/gorunner` (`FuzzParity`) | Differential fuzzing: the Go oracle in-process against the Rust runner |
| `harness/cmd/differ` | Runs a corpus through two implementations and reports every difference |
| `harness/cmd/throughput` | Sustained load for Go: throughput, tail latency, allocation rate, GC pressure, RSS |
| `rust/sqllexer-runner` | The Rust protocol runner, and (`--bin bench`) the same load driver in the same report format |

Latency percentiles come from a fixed-size histogram shared by both drivers
(`internal/latency`, mirrored in `rust/sqllexer-runner/src/histogram.rs`): keeping
every sample would make the harness's own memory a function of how many operations
completed, turning the faster engine into the one that appears hungrier.
