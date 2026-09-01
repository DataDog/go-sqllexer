# Memory-safety audit of the Rust <-> Go boundary

Everything here is reproducible with [`run.sh`](run.sh); the unedited output of
every run is in [`logs/`](logs). This document says what each tool covered, what
it structurally could not cover, and what was found. Read the coverage limits
before reading the findings: three of the four tools cannot see the cgo crossing
at all, and the one that can (valgrind) is blind to Go's heap.

The object model under audit, as documented in `harness/rustffi/rustffi.go` and
`rust/sqllexer-ffi/src/lib.rs`:

- one handle per worker, created once and reused;
- a handle is `Send` but not `Sync` — sharing one across threads is unsupported;
- a result borrows buffers the handle owns, valid until the next call on that
  handle or its destruction; the Go binding copies every result into Go memory
  before returning, which is what makes the Go-facing API safe.

## Tools and versions

| Tool | Version | Stage |
| --- | --- | --- |
| Go | go1.25.7 linux/amd64 | all Go stages |
| Rust (stable) | 1.98.0 | release archive linked into cgo |
| Rust (nightly) | 1.100.0-nightly (0dfb098f3 2026-08-31) | ASan, Miri |
| Miri | 0.1.0 (0dfb098f3a 2026-08-31) | `miri` |
| valgrind memcheck | 3.18.1 | `valgrind` |
| llvm-symbolizer | 14.0.0 | symbolizes the ASan frames in the logs |

Host: 8 vCPU x86_64, 31 GB, Linux 5.15. `logs/versions.log` is the recorded
output. Everything below is x86_64-only; no other architecture was tested.

## What ran

### 1. AddressSanitizer + LeakSanitizer — `logs/asan.log`, `logs/asan-controls.log`

```
RUSTFLAGS=-Zsanitizer=address cargo +nightly test -Zbuild-std \
  --target x86_64-unknown-linux-gnu -p sqllexer-ffi --features panic-probe
```

`-Zbuild-std` matters: without it `core`/`alloc` are precompiled without
instrumentation and allocations made inside them are invisible.

Covers the FFI crate's own unit tests plus `rust/sqllexer-ffi/tests/stress.rs`,
which drives the ABI the way a host does:

- `drives_one_handle_through_every_mode` — one handle, all four entry points
  (`process`, `normalize`, `obfuscate`, `tokenize`), every DBMS code including an
  unknown one, over 12 inputs: empty, NUL bytes, lone continuation bytes, a
  truncated UTF-8 sequence, emoji, a 2000-element `IN` list, 500-deep nesting and
  a 26KB statement. Every result is read back in full — a bad descriptor is only a
  memory error once something dereferences it — and each output is size-checked so
  a mode that fails to reset the shared buffers shows up as growth.
- `creates_and_destroys_many_handles` — 200 create/use/destroy cycles.
- `one_handle_per_thread_is_independent` — 4 threads, one handle each, checking
  that no thread sees another's output.
- `a_panic_is_contained_and_leaves_the_handle_usable` — `sqllexer_panic_probe`
  (see below) returns `SQLLEXER_ERR_PANIC`, publishes no result, and the handle
  still works afterwards.

Result: clean. No error reports, no leaks.

**"Clean" is only worth something if the run could have failed**, so two ignored
tests exist as positive controls and are run one at a time in
`logs/asan-controls.log`:

| Control | Expected | Observed |
| --- | --- | --- |
| `reading_a_result_after_free_is_a_use_after_free` | ASan aborts | `heap-use-after-free`, `READ of size 32`, symbolized to `stress.rs:242` |
| `leaking_a_handle_is_reported` | LSan reports | `1566 byte(s) leaked in 6 allocation(s)`, rooted at `sqllexer_processor_new` |

Both exit non-zero. Leak detection was therefore on, and the sanitizer does see
violations of exactly the two lifetime rules the object model states.

**Cannot cover:** anything on the Go side. The Go runtime is not built with ASan
here, so this stage never executes the cgo crossing, the Go copy-out, or Go's
allocator. A sanitized cgo binary (`CGO_CFLAGS=-fsanitize=address` plus a
sanitized archive) was not attempted: Go's runtime and ASan's interceptors
conflict over signal handling and stack switching, and the result would not have
been trustworthy evidence. Valgrind, below, is the stage that covers the real
binary. Also: these are debug builds, so the release archive that cgo actually
links (`lto=fat`, `codegen-units=1`) is not the artifact that was sanitized.

### 2. Miri — `logs/miri.log`, `logs/miri-controls.log`

```
cargo +nightly miri test -p sqllexer
cargo +nightly miri test -p sqllexer-ffi --features panic-probe
```

Both clean: 18 tests in the pure crate, 5 unit tests plus the 4 active stress
tests in the FFI crate. The stress inputs shrink under `cfg(miri)` (`SCALE`,
`ROUNDS` in `stress.rs`) because Miri is an interpreter — the shapes are
preserved, the volume is not, and volume is not what Miri checks.

Miri is the only tool here that checks aliasing (Stacked Borrows) and pointer
provenance, so it is the only one that can see the third misuse case: handing a
result's buffer back in as the next call's input. That is UB even though nothing
is freed and nothing goes out of bounds, and neither ASan nor valgrind can
detect it. `logs/miri-controls.log` records both controls firing:

| Control | Miri says |
| --- | --- |
| `feeding_a_result_back_as_input_aliases_handle_memory` | `not granting access to tag ... because that would remove [SharedReadOnly for ...] which is strongly protected` |
| `reading_a_result_after_free_is_a_use_after_free` | `constructing invalid value of type &[u8]: encountered a dangling reference (use-after-free)` |

**Cannot cover:** the C ABI crossing. Miri interprets Rust; the `extern "C"`
functions are called *from Rust* in these tests, so what is validated is the
Rust-side implementation of the ABI functions — argument handling, lifetimes,
aliasing, provenance — and not the crossing itself, the cgo prologue, or the Go
side. Nothing in this stage exercises the binding.

### 3. valgrind memcheck on the real cgo binary — `logs/valgrind*.txt`, `logs/valgrind.log`

`harness/cmd/ffirunner` built with `-a -tags rustffi`, against
`harness/cmd/gorunner` as the baseline, over the same 3,367-statement corpus
(all of `testdata` and `workloads` and `fuzzseeds`, every 4th `pathological`
statement, every 10th `matrix` statement — memcheck costs 30-100x and the
pathological corpus alone is 11MB). Both runners produced byte-identical output
under valgrind, so the FFI path was really executed.

| | gorunner (baseline) | ffirunner (FFI) |
| --- | --- | --- |
| total heap usage | 0 allocs, 0 frees | 17,209 allocs, 17,205 frees, 4.8 MB |
| definitely lost | 0 | **0** |
| indirectly lost | 0 | **0** |
| possibly lost | 0 | 864 B in 3 blocks |
| still reachable | 0 | 29,696 B in 1 block |
| error summary | 1,038,621 errors from 1,000 contexts (limit reached) | 43 errors from 8 contexts |

The baseline line to notice is `0 allocs`: a pure-Go binary never calls `malloc`,
so memcheck sees nothing of it and reports "no leaks are possible". Every heap
block in the ffirunner column is therefore Rust's or the C runtime's — which is
what makes this stage the useful one, and also means **memcheck cannot detect a
leak on the Go side of the binding at all**.

17,209 allocations for 3,367 statements, with 4 blocks live at exit, is the
positive result: allocation is per-handle and per-buffer-growth, not per call. A
per-call leak would have shown up as tens of thousands of live blocks.

The three non-zero rows, all attributable and none a defect:

- **29,696 B still reachable** — one `realloc` under
  `sqllexer::keywords::Trie::initialize` through a `OnceLock`, i.e. the keyword
  trie built once per process and intentionally never freed.
- **864 B possibly lost in 3 blocks** — `calloc` in `_dl_allocate_tls` under
  `pthread_create` called from `_cgo_sys_thread_start`: glibc thread-local
  storage for the threads cgo starts. Standard cgo-under-valgrind noise; not
  reachable by anything the port controls.
- **43 errors in 8 contexts** — all `Conditional jump or move depends on
  uninitialised value(s)` inside `runtime.adjustframe`/`runtime.adjustctxt`
  during Go stack copying. Go runtime noise; the same class dominates the
  baseline's million errors.

`logs/valgrind-ffi-only-frames.txt` is the mechanical answer to "what is in the
FFI binary that is not in the Go one": 22 frames, all of them either the Rust
trie initialization, the cgo thread startup path, or `sqllexer_process` itself in
the still-reachable trie record. No frame in the per-call path appears in any
error or leak record.

**Cannot cover, and two compromises worth knowing:**

- Go's heap, as above.
- Both runners were run with `GOMAXPROCS=1 GOGC=off`. This is not cosmetic:
  valgrind serializes threads, and with concurrent GC on this corpus the run went
  from ~10 seconds to more than 20 minutes without finishing. The cost is that
  GC-time behavior and multi-threaded interleavings are outside this stage. The
  sustained-load stage covers the concurrent, GC-on case instead, without
  memcheck.
- The default error limit is left on. With `--error-limit=no` valgrind's context
  list grows until the run does not finish. Consequence: the baseline's error
  *count* is truncated at 1,000 contexts and the two counts are not comparable.
  The comparison that is meaningful — and the one made above — is by error kind
  and stack frame, not by count. Leak detection is unaffected by the limit.

### 4. Leak check under sustained load — `logs/load.log`, `logs/load-rss.tsv`

`harness/cmd/throughput -impl rust -workers 4 -duration 10m`, RSS sampled every
5 seconds from `/proc/<pid>/status` rather than from the harness's own report.

**927,775,744 operations** in 10 minutes (~1.55M ops/s). RSS climbs from 5.6 MB
to ~22.6 MB during the first 50 seconds and then oscillates between 22.4 and 23.4
MB for the remaining nine minutes — the GC sawtooth, with no trend. A one-byte
leak per operation would have added ~900 MB.

**Cannot cover:** a leak that is not proportional to call count (per-handle, or
per-configuration), and anything only reachable through entry points the
throughput harness does not drive (it drives `ObfuscateAndNormalize`).

### 5. Deliberate misuse — `logs/misuse.log`, `logs/misuse-guard-regression.log`

`harness/rustffi/misuse_test.go` and `harness/rustffi/panicprobe_test.go`, run
with `-a` (mandatory: Go's build cache ignores changes to the Rust archive) and
also under `-race`.

| Misuse | Behavior |
| --- | --- |
| Null input pointer (empty Go string) | Accepted, not dereferenced; returns empty results |
| Every entry point on a closed handle | Error, no call into Rust |
| `Close` twice | Idempotent |
| Result read after the handle is destroyed | Safe: results are copies, verified after `Close` + forced GC + heap churn |
| Token values after the input string is collected | Safe: values are copied, verified after the input is dropped and GC runs twice |
| One handle from two goroutines | Rejected with `ErrConcurrentUse` — see finding F1 |
| Panic inside Rust | Contained by `catch_unwind`, surfaces as `ErrPanic`, handle still usable |

Panic containment across cgo is checked with `sqllexer_panic_probe`, an entry
point behind the `panic-probe` cargo feature that panics inside the same
`catch_unwind` wrapper the real entry points use. It is feature-gated and behind
the extra `rustffi_panicprobe` build tag, so it is not in the shipped archive;
`run.sh` builds the archive with the feature, runs the probe tests, and rebuilds
it without. Both the direct case and the case where the panic happens on a
freshly grown goroutine stack return an error rather than unwinding into the Go
runtime.

## Findings

### F1 — Overlapping calls on one handle silently produced wrong output (fixed, medium)

Concurrent use of a handle is documented as unsupported, but before this branch
nothing detected it: two goroutines calling one handle each got results written
into the same handle-owned buffers, and the second call overwrote what the first
was about to copy out. The corruption is in Rust-owned memory, so neither the Go
race detector nor ASan reports it — the caller just gets another statement's
output, or a mix of two. For an obfuscator that means one caller's SQL can be
returned to another caller.

Fix: the handle carries a `busy` flag claimed for the duration of a call **and
its copy-out**, and an overlapping call returns `ErrConcurrentUse` instead of
running. This detects the misuse; it does not make sharing a handle supported.

The regression test is `TestConcurrentMisuseUnderLoad` (opt-in via
`SQLLEXER_FFI_MISUSE_STRESS=1`, because whether two goroutines actually overlap
is a scheduling accident and a flaky CI test is worse than an opt-in one).
`logs/misuse-guard-regression.log` is that test against a build where the guard
is released when the FFI call returns instead of after the copy: **27,091 of
4,677,918 completed calls returned corrupted output**. With the fix, over
33-42 million rejected and ~4 million completed calls per run, the count is 0.

Note the shape of the intermediate bug, because it is the interesting part: a
guard around the FFI call alone is not enough. The result descriptors point into
handle-owned buffers, so the claim has to outlive the call and cover the copy.

### F2 — Keyword trie is never freed (not a defect)

29,696 bytes still reachable at exit, one block, built once through a `OnceLock`
on first use. Deliberate process-lifetime state, constant regardless of load.

### F3 — cgo thread TLS reported as possibly lost (not a defect)

864 bytes in 3 blocks from `_dl_allocate_tls` under `_cgo_sys_thread_start`.
glibc TLS for cgo's threads, freed at process exit by the OS; nothing in this
port allocates or owns it.

### F4 — Concurrent `Close` remains unsupported and undetected (accepted, documented)

The `busy` guard covers overlapping *calls*. Calling `Close` while another
goroutine is inside a call is still a use-after-free with no detection, and
closing a handle is not idempotent with respect to in-flight calls. Making it
safe needs either refcounting or a mutex, both of which cost per call and neither
of which is needed by the one-handle-per-worker model. It is documented on
`Close` and left as is.

## What was not done

- No sanitized cgo binary (see the ASan section for why).
- No ThreadSanitizer. The Rust side is single-threaded per handle by
  construction, the supported model has no shared mutable state, and the Go side
  was covered with `-race` instead.
- No architecture other than x86_64. In particular the `arm-8core-linux` runner
  named in `harness/reports/README.md` was not used.
- Valgrind ran a sampled corpus, not every statement, and with GC off.
- Miri ran with the default Stacked Borrows model; Tree Borrows was not run.
- The sanitizer runs use debug builds; the release archive with `lto=fat` is not
  itself instrumented.

## Can B4 be marked met?

Yes, with wording that matches the evidence. Suggested:

> **B4 — met, x86_64 only.** The Rust FFI surface is clean under
> AddressSanitizer + LeakSanitizer (`-Zbuild-std`) across a stress suite covering
> all four entry points, invalid UTF-8 and pathological inputs, and clean under
> Miri for the pure crate and the Rust-side ABI functions. A real cgo binary run
> under valgrind memcheck over a 3,367-statement corpus reports 0 bytes
> definitely or indirectly lost, with 17,205 of 17,209 allocations freed; the
> remaining live memory is the process-lifetime keyword trie and glibc TLS for
> cgo threads. 928M operations of sustained load leave RSS flat. Sanitizer
> detection was validated with deliberate use-after-free, leak and aliasing
> controls, each of which fails the run as expected. One real defect was found
> and fixed: overlapping calls on one handle produced corrupted output and are
> now rejected with `ErrConcurrentUse`. Not covered: the cgo crossing itself
> under a sanitizer (no tool here instruments both runtimes), Go-side heap
> behavior under memcheck, non-x86_64 targets, and `Close` racing an in-flight
> call, which remains unsupported and undetected.

The claim a reviewer should take from this directory is not "clean" but "clean
under tools that were shown to catch the failures they claim to catch, with these
four gaps".
