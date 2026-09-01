# Removing the Go-side allocation overhead of the cgo binding

The Rust core allocates ~26 B/op, but the binding measured 526 B/op and 7.21
allocs/op on the Go side: the port was not the cost, the marshalling was. This
is what the marshalling now does, what it costs, and what it relies on to be
safe.

Everything below was measured on this VM (8 vCPU Intel Xeon Platinum 8559C,
Linux, Go 1.25.7, Rust 1.98.0) with `harness/reports/zero-copy/run.sh`, which is
`harness/reports/run.sh`'s methodology with the phase in the file name: 20s per
run, 5s warmup, one run at a time, handles reused per worker. The JSON is in
`harness/reports/zero-copy/`.

## Where the 7.21 allocs went

Per `ObfuscateAndNormalize` call, before:

| allocation | count |
| --- | --- |
| `C.GoStringN` of the normalized SQL | 1 |
| `C.GoStringN` of each metadata value | N |
| four `[]string` for tables/comments/commands/procedures | 4 |
| the `StatementMetadata` | 1 |
| the out-parameter struct, moved to the heap because its address is passed to C | 1 |
| the closure passed to the shared `call` helper, which escapes | 1 |

So the floor was 7 allocations even for a statement with no metadata at all, and
it grew with the metadata.

## What changed

**1. One packed result instead of N slices (Rust + Go).**
`sqllexer_process_packed` / `sqllexer_normalize_packed` return the normalized
SQL, one buffer holding every metadata value end to end, a `usize` length per
value, and the per-list counts. The Go side allocates one `[]byte` sized
`len(sql)+len(values)`, fills it with two `copy`s, and hands out `unsafe.String`
windows onto it. N+1 copies and N+1 string headers become 1 copy and one
`[]string`; the four metadata lists are sub-slices of that one `[]string` with
capacity clamped, so they cannot grow into each other.

*Safety:* the strings alias Go memory that was written before it was published
and is never written again, and each string keeps it alive. That is what lets
the results keep the old contract — they survive later calls on the handle and
survive `Close`, which `TestResultsSurviveSubsequentCalls` pins. The Rust
buffers are read only during the call, while the handle is borrowed.

**2. The out-parameters live on the handle, not on the stack.**
Taking the address of a local `C.sqllexer_packed` to pass to C moves it to the
heap: one allocation per statement, invisible in the source. They are now fields
of `Processor`, which is allocated once. None of them contains a Go pointer, so
passing them to C is within the cgo pointer rules, and `Processor` is
single-threaded by contract, so no two calls share them concurrently.

**3. No closure at the call boundary.**
The shared `call(sql, dbms, func(...) C.int32_t)` helper cost one heap-allocated
closure per statement. It is split into `args` (closed-handle check, input
pointer) and `finish` (`runtime.KeepAlive` of the input, status → error), with
the C call written out at each site.

**4. `Tokenize` returns substrings of the caller's input.**
The Rust lexer borrows the input for token values and only materializes a value
when it has to rewrite one — which the raw lexer never does; only the obfuscator
and normalizer do. When a token's pointer lies inside the input string's bytes,
the binding slices the caller's Go string instead of copying: same bytes, no
allocation, and the value owns its memory exactly as much as the input does. A
pointer outside that range still goes through a copy, so the path stays correct
if Rust ever starts materializing values here.

**5. A borrowed API alongside, not instead of, the owning one.**
`ObfuscateAndNormalizeInto(sql, dbms, *Borrowed)` fills a caller-owned struct
with strings that point straight into the handle's buffers, reusing one
`[]string` the caller keeps. Zero allocations, and a strictly weaker lifetime:
the values are invalidated by the next call on that handle and by `Close`. That
contract is documented in capitals on `Borrowed`; the existing methods are
untouched in semantics.

**6. A size-only API.**
`ObfuscateAndNormalizeSize` returns the normalized SQL and
`StatementMetadata.Size` without materializing any list on either side, for
callers that only report the size. One allocation: the SQL copy.

## Measured

`ops/s`, `B/op`, `allocs/op`, GC cycles and pause, peak RSS, and p50/p99 in µs
for the dominant class and the heavy class. `before` is the binding as it was;
`rust` after is the same public call, `rust-borrowed` and `rust-size` are the new
APIs. Go is included as the unchanged control.

### workloads

| phase | impl | workers | ops/s | B/op | allocs/op | GC | pause ms | RSS MB | p50/p99 short | p50/p99 large |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| before | go | 1 | 223,485 | 905.1 | 11.21 | 1084 | 67.5 | 14.3 | 1.17/4.42 | 44.42/105.47 |
| before | rust | 1 | 352,552 | 526.0 | 7.21 | 980 | 39.4 | 14.4 | 0.93/2.92 | 24.10/34.27 |
| after | rust | 1 | 366,266 | **422.6** | **2.99** | 820 | 32.1 | 14.4 | 0.83/2.73 | 24.24/34.66 |
| after | rust-borrowed | 1 | 400,528 | **0.3** | **0.00** | 1 | 0.2 | 8.6 | 0.75/2.54 | 24.02/34.11 |
| after | rust-size | 1 | 385,381 | 252.2 | 0.98 | 508 | 27.4 | 13.8 | 0.69/2.49 | 23.98/33.98 |
| after | go | 1 | 233,013 | 905.1 | 11.21 | 1125 | 79.6 | 14.2 | 1.18/4.55 | 44.51/178.94 |
| before | go | 8 | 761,273 | 905.6 | 11.21 | 947 | 10008.9 | 57.3 | 1.21/5.07 | 45.34/482.82 |
| before | rust | 8 | 1,990,880 | 526.0 | 7.21 | 1117 | 2211.4 | 89.3 | 1.00/3.61 | 24.78/42.82 |
| after | rust | 8 | 2,145,513 | **422.6** | **2.99** | 982 | 1645.6 | 65.0 | 0.93/3.32 | 25.30/41.09 |
| after | rust-borrowed | 8 | 2,849,674 | **0.3** | **0.00** | 2 | 2.2 | 14.4 | 0.80/2.64 | 24.30/34.18 |
| after | rust-size | 8 | 2,567,589 | 252.2 | 0.97 | 731 | 871.1 | 55.8 | 0.75/2.71 | 24.70/37.47 |
| after | go | 8 | 766,038 | 905.6 | 11.21 | 953 | 9905.0 | 49.5 | 1.21/5.03 | 45.31/414.21 |

### pathological

| phase | impl | workers | ops/s | B/op | allocs/op | GC | pause ms | RSS MB | p50/p99 short | p50/p99 pathological |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| before | go | 1 | 1,940 | 77,482.9 | 9.15 | 203 | 18.6 | 35.8 | 0.93/3.89 | 290.56/8732.67 |
| before | rust | 1 | 2,752 | 19,000.4 | 5.84 | 71 | 1.6 | 34.5 | 0.75/3.58 | 164.86/7671.81 |
| after | rust | 1 | 2,955 | 18,896.7 | **2.86** | 75 | 1.5 | 35.6 | 0.67/3.46 | 172.93/7725.06 |
| after | rust-borrowed | 1 | 3,046 | **35.7** | **0.03** | 0 | 0.0 | 20.4 | 0.61/3.02 | 169.73/7692.29 |
| after | rust-size | 1 | 3,347 | 18,733.2 | 0.86 | 85 | 1.9 | 36.2 | 0.55/3.08 | 169.34/6922.24 |
| after | go | 1 | 2,227 | 77,442.2 | 9.15 | 232 | 10.0 | 34.5 | 0.92/4.22 | 288.51/3280.90 |
| before | go | 8 | 10,464 | 77,444.5 | 9.12 | 601 | 4478.7 | 70.0 | 0.94/5.33 | 293.63/21544.96 |
| before | rust | 8 | 25,723 | 18,945.0 | 5.81 | 357 | 26.5 | 63.7 | 0.75/3.19 | 164.99/3014.66 |
| after | rust | 8 | 25,314 | 18,859.6 | **2.84** | 348 | 40.3 | 63.6 | 0.67/3.23 | 172.67/3313.66 |
| after | rust-borrowed | 8 | 26,107 | **28.1** | **0.00** | 1 | 0.0 | 23.8 | 0.62/2.13 | 170.37/2099.20 |
| after | rust-size | 8 | 25,050 | 18,726.1 | 0.84 | 346 | 60.0 | 66.9 | 0.55/2.99 | 170.24/2930.69 |

Microbenchmarks on the same statement (`go test -a -tags rustffi -bench . -benchmem ./harness/rustffi/`):

```
BenchmarkObfuscateAndNormalize-8       1106 ns/op   304 B/op   3 allocs/op
BenchmarkObfuscateAndNormalizeInto-8    880 ns/op     0 B/op   0 allocs/op
BenchmarkObfuscateAndNormalizeSize-8    840 ns/op    64 B/op   1 allocs/op
BenchmarkTokenize-8                     538 ns/op   704 B/op   1 allocs/op
```

### Reading the numbers

- Same API, same semantics: **7.21 → 2.99 allocs/op and 526 → 423 B/op** on
  workloads, i.e. the fixed per-call allocation count is now 3 regardless of how
  much metadata a statement has (the bytes, the string headers, the
  `StatementMetadata`), where it used to be 7 plus one per metadata value.
- The residual 423 B/op is dominated by the bytes themselves: the normalized SQL
  plus the metadata values must be copied somewhere for the result to outlive
  the next call. That is the price of the owning contract, not of the binding.
  On `pathological`, where statements are huge, it is the whole 18.9 KB/op — the
  allocation *count* drops by half, the byte count barely moves, exactly as
  expected.
- Dropping the contract removes it entirely: **0.3 B/op and 0.00 allocs/op** on
  workloads through `ObfuscateAndNormalizeInto`, with GC going from ~1000 cycles
  to 1 and peak RSS from 14.4 MB to 8.6 MB at one worker.
- Throughput follows GC pressure rather than the copy itself. At one worker the
  owning path gains ~4% (353k → 366k ops/s) — the copy is cheap next to the scan
  — but at eight workers, where the collector is contended, the borrowed path is
  **+43%** over the previous binding (1.99M → 2.85M ops/s) with GC pause going
  from 2.2 s to 2 ms across the run. On `pathological` throughput is bounded by
  the scan itself, so all three variants land within noise of each other while
  the borrowed one still eliminates the garbage.
- p50/p99 improve slightly and monotonically for the short class (0.93 → 0.83 →
  0.75 µs p50 at one worker). The heavy classes are unchanged within run-to-run
  noise; their p99.9 swings by milliseconds between runs even for identical
  binaries, so no claim is made there.

## What each change relies on

| change | safety property |
| --- | --- |
| packed decode + `unsafe.String` | the backing `[]byte` is Go-owned, fully written before any string is published, never written again, and kept alive by the strings themselves |
| out-params on the handle | they contain no Go pointers (cgo rule), and a `Processor` is single-threaded by contract — one per worker, as before |
| `args`/`finish` split | `runtime.KeepAlive(sql)` still runs after the C call on every path, so the input cannot be collected while Rust borrows it |
| token substrings | the value pointer is checked to lie inside the input string's bytes before it is treated as a window onto it; anything else is copied |
| `Borrowed` | documented as invalid after the next call and after `Close`; nothing in the package hands a borrowed string to an owning API |
| all new entry points | `catch_unwind` at the boundary and the same null/handle checks as the existing ones, so no panic crosses the C ABI |

## What did not pay off

- **Packing alone was worth 4 of the 7 allocations, not 7.** After the packed
  ABI landed, `AllocsPerRun` still reported 4 — the escaped out-parameter and
  the escaped closure. Both are invisible in the source and were only found via
  `go build -gcflags=-m`. Neither has anything to do with cgo marshalling per
  se; they are Go escape analysis. Worth remembering for anything similar.
- **Caller-provided output buffers (Rust writing into a Go `[]byte`) were not
  implemented, and were not measured.** The design needs either a size query
  followed by a second call, or a short-buffer status and a retry — two cgo
  transitions instead of one, plus a Go-visible capacity protocol — and it can
  only ever reach what the borrowed API already reaches for free, zero copies.
  Its one advantage over `Borrowed` is that the result would outlive the next
  call. If a caller needs that *and* zero allocations, this is the design to
  reach for; as it stands, `Borrowed` plus `strings.Clone` at the one place that
  needs longevity is simpler and no slower.
- **The size-only API is not the win it looks like on paper.** It removes the
  metadata materialization, but the metadata still has to be *collected* on the
  Rust side, since the size is defined by it, and the SQL copy remains. 252
  B/op and ~1 alloc/op versus 423 and 3 — real, but `Borrowed` beats it on every
  axis for a caller that can accept the lifetime, and the throughput difference
  between the two is under 10%.
- **Reusing one scratch `[]byte` per handle for the owning path is impossible by
  construction**, which is worth stating explicitly: the whole point of the
  owning contract is that the result outlives the next call, so the memory it
  points at cannot be recycled by that call. Any owning API allocates at least
  once per statement. The only ways past that are the borrowed API or a caller
  that hands in its own buffer.

## Correctness

All differential runs are through cgo, reference `go run ./harness/cmd/gorunner`,
candidate `/tmp/ffirunner` built with `go build -a -tags rustffi`:

| corpus | requests | mismatches | order-only |
| --- | --- | --- | --- |
| testdata | 441 | 0 | 0 |
| workloads | 199 | 0 | 0 |
| pathological | 288 | 0 | 0 |
| matrix | 20000 | 0 | 0 |
| fuzzseeds | 771 | 0 | 0 |

`go test -a -tags rustffi -run xxx -fuzz FuzzParity -fuzztime 5m
./harness/rustffi/` passed with 84.3M executions. The frozen Go suite
(`go test ./...`) passes unmodified, as do `gofmt -l harness`, `go vet -tags
rustffi ./harness/...`, and in `rust/`: `cargo fmt --all --check`, `cargo clippy
--all-targets --all-features -- -D warnings`, `cargo test`.

`harness/rustffi/zerocopy_test.go` covers the lifetime rules the optimization
depends on rather than only the output bytes: that owning results do not alias
handle memory and survive reuse and `Close`, that the four metadata lists cannot
overwrite each other, that empty lists stay non-nil so the metadata still
compares equal to Go's, that borrowed results *do* alias the handle and match the
owning API value for value, that a reused `Borrowed` is fully reset, that the
size-only API agrees with the full one, that token values survive later calls,
and that the per-call allocation counts are what this document claims.
