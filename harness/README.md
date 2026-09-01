# Migration validation harness

Tooling to decide, with evidence, whether an alternative implementation of the
lexer/obfuscator/normalizer (initially a Rust rewrite consumed through cgo) is a
safe replacement for the Go one.

Nothing here is part of the library: `harness/` is additive, the existing tests and
`testdata/` are never modified, and the Go implementation is the oracle every
candidate is measured against.

## Design

Every implementation under validation exposes the same interface: a binary that
reads [`protocol.Request`](internal/protocol/protocol.go) objects as
newline-delimited JSON on stdin and writes one `protocol.Response` per line to
stdout. That is the entire coupling between the harness and an implementation, so
the Rust core needs no Go bindings to be validated, and the FFI layer can be
validated separately as just another runner.

Two details of the wire format are load-bearing:

- **Text may be non-UTF-8.** The library accepts arbitrary bytes, and JSON encoders
  replace invalid UTF-8 with U+FFFD. `protocol.Text` encodes such values as
  `{"b64": "..."}` instead, so a fuzz-derived input reaches both implementations
  unchanged. Non-Go implementations must implement the same rule.
- **Configs are always explicit.** The fixture suite's implicit defaults are not the
  library's defaults (`dbms_test.go` enables `dollar_quoted_func`,
  `replace_positional_parameter` and `collect_procedure`), so generated corpora
  materialize every option rather than letting each implementation fill in its own.

## Tools

| Tool | Purpose |
| --- | --- |
| `harness/cmd/corpusgen` | Builds the shared corpora from testdata, benchmark literals, generated pathological inputs, a sampled config matrix, and an optional Go fuzz corpus |
| `harness/cmd/gorunner` | The Go reference implementation of the protocol |
| `harness/cmd/differ` | Runs a corpus through two implementations and reports every difference |
| `harness/cmd/ffirunner` | The same protocol backed by the Rust core through cgo, so the binding is validated as just another runner (`-tags rustffi`) |
| `harness/cmd/throughput` | Sustained load: throughput, tail latency, allocation rate, GC pressure, RSS. `-impl go` or `-impl rust` (the latter needs `-tags rustffi`) |
| `rust/sqllexer-runner --bin bench` | The same load driver for the Rust core with no FFI in the path |
| `harness/rustffi` | The cgo binding itself, plus `FuzzParity`, which runs Go and Rust against the same input in one process |

## Usage

Generate the corpora (the fuzz corpus is optional; it appears only after
`go test -fuzz` has been run locally or restored from CI):

```sh
go run ./harness/cmd/corpusgen -out harness/corpus \
  -fuzz-corpus "$(go env GOCACHE)/fuzz/github.com/DataDog/go-sqllexer/FuzzObfuscatorAndNormalizer"
```

| Corpus | Contents |
| --- | --- |
| `testdata.jsonl` | One request per fixture output, with that fixture's exact config |
| `workloads.jsonl` | SQL literals extracted from the benchmark files by parsing their AST |
| `pathological.jsonl` | Generated stress inputs (huge parameter lists, deep nesting, unterminated constructs, invalid UTF-8, comment-only, empty) across every DBMS and mode |
| `matrix.jsonl` | Seeded random sampling of the option space: 2^7 obfuscator x 2^9 normalizer x 6 DBMS x 4 modes |
| `fuzzseeds.jsonl` | Inputs imported from a Go fuzz corpus directory |

Compare an implementation against the Go oracle. Zero mismatches is the acceptance
bar; the driver exits non-zero if any are found and writes a replayable report:

```sh
go run ./harness/cmd/differ \
  -corpus harness/corpus/matrix.jsonl \
  -reference "go run ./harness/cmd/gorunner" \
  -candidate ./target/release/sqllexer-runner \
  -report /tmp/mismatches.jsonl
```

Anything built with `-tags rustffi` must be built with `go build -a`. Go's build
cache keys on source content, so rebuilding `libsqllexer_ffi.a` alone is a cache hit
and the binary silently keeps linking the previous Rust code:

```sh
(cd rust && cargo build --release)
go build -a -tags rustffi -o /tmp/ffirunner ./harness/cmd/ffirunner
```

Measure sustained load. Reporting is split by workload class (short, medium, large,
pathological) because short statements are dominated by per-call overhead — the
place where cgo is most likely to erase a win — while large ones are dominated by
scanning throughput:

```sh
go run ./harness/cmd/throughput \
  -corpus harness/corpus/workloads.jsonl \
  -workers 8 -duration 30s -json /tmp/tier4.json
```

`-reuse=false` constructs the obfuscator and normalizer per call instead of once per
worker. The gap between the two runs is the cost the FFI object model has to avoid.

All three engines at once, into [`reports/`](reports/README.md):

```sh
harness/reports/run.sh
```

Runs are sequential by construction: two load drivers on one host measure each
other. Latency percentiles come from a fixed-size histogram shared by both drivers
(`internal/latency`, mirrored in `rust/sqllexer-runner/src/histogram.rs`) so the
harness's own memory does not grow with the number of operations completed — which
would otherwise make the faster engine look like the hungrier one.

## What a candidate has to satisfy

See [ACCEPTANCE.md](ACCEPTANCE.md) for the full criteria. In short: zero mismatches
on every corpus, no panics or sanitizer findings, and performance gates that are
ratified from measured baselines rather than assumed up front.
