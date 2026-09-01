#!/usr/bin/env bash
# Reproduces every run recorded in this directory's README. Each stage writes its
# raw output to logs/<stage>.log; nothing is filtered, so a log that looks noisy
# is meant to look that way.
#
#   ./harness/sanitizers/run.sh            # every stage, in order
#   ./harness/sanitizers/run.sh asan miri  # only the named stages
#
# Stages differ enormously in cost: `asan` and `miri` are minutes, `valgrind` is
# tens of minutes, `load` is LOAD_DURATION long (10m by default).
set -uo pipefail

cd "$(dirname "$0")/../.."
root=$PWD
logs=harness/sanitizers/logs
mkdir -p "$logs"
# Binaries and corpora are per-invocation: two invocations running at once must
# not delete each other's inputs.
work=harness/sanitizers/.work-$$
mkdir -p "$work"
trap 'rm -rf "$work"' EXIT

nightly=${NIGHTLY:-nightly}
target=${TARGET:-x86_64-unknown-linux-gnu}
load_duration=${LOAD_DURATION:-10m}
load_workers=${LOAD_WORKERS:-4}

# Runs a command with its whole output teed into a log, and records the exit
# status in the log rather than aborting: a stage that fails on purpose (the
# sanitizer positive controls) still has to leave evidence behind.
run() {
  local log=$1
  shift
  {
    echo "\$ $*"
    "$@" 2>&1
    echo "[exit $?]"
  } | tee -a "$logs/$log"
}

stage_versions() {
  : >"$logs/versions.log"
  run versions.log go version
  run versions.log rustc --version
  run versions.log cargo --version
  run versions.log rustc "+$nightly" --version
  run versions.log cargo "+$nightly" miri --version
  run versions.log valgrind --version
  run versions.log llvm-symbolizer --version
  run versions.log uname -a
}

# The static archive the cgo binding links against. Go's build cache keys on
# source content and not on the archive, so every Go build below passes -a.
stage_build() {
  : >"$logs/build.log"
  run build.log bash -c "cd rust && cargo build --release"
  run build.log go build -a -tags rustffi -o "$work/ffirunner" ./harness/cmd/ffirunner
  run build.log go build -o "$work/gorunner" ./harness/cmd/gorunner
  run build.log go build -a -tags rustffi -o "$work/throughput" ./harness/cmd/throughput
}

ensure_build() {
  [ -x "$work/ffirunner" ] && [ -x "$work/gorunner" ] && [ -x "$work/throughput" ] && return
  stage_build
}

# ASan+LSan over the FFI crate's own tests. -Zbuild-std rebuilds core/alloc with
# the sanitizer, without which every allocation made inside std is invisible.
asan_test() {
  local log=$1
  shift
  RUSTFLAGS="-Zsanitizer=address" \
    RUSTDOCFLAGS="-Zsanitizer=address" \
    ASAN_OPTIONS="detect_leaks=1:abort_on_error=0:detect_stack_use_after_return=1" \
    run "$log" bash -c "cd rust && cargo +$nightly test -Zbuild-std --target $target -p sqllexer-ffi --features panic-probe $*"
}

stage_asan() {
  : >"$logs/asan.log"
  asan_test asan.log
}

# The positive controls: deliberate defects that prove the sanitizer would have
# caught a real one. Each aborts the test process, so they run one at a time and
# a zero exit status here is the failure.
stage_asan_controls() {
  : >"$logs/asan-controls.log"
  for control in reading_a_result_after_free_is_a_use_after_free leaking_a_handle_is_reported; do
    asan_test asan-controls.log "--test stress -- --ignored --exact $control"
  done
}

# Miri interprets Rust; it cannot execute the Go side, so this covers the ABI
# functions only, called from Rust.
stage_miri() {
  : >"$logs/miri.log"
  run miri.log bash -c "cd rust && cargo +$nightly miri test -p sqllexer"
  run miri.log bash -c "cd rust && cargo +$nightly miri test -p sqllexer-ffi --features panic-probe"
}

stage_miri_controls() {
  : >"$logs/miri-controls.log"
  for control in feeding_a_result_back_as_input_aliases_handle_memory reading_a_result_after_free_is_a_use_after_free; do
    run miri-controls.log bash -c \
      "cd rust && cargo +$nightly miri test -p sqllexer-ffi --features panic-probe --test stress -- --ignored --exact $control"
  done
}

# memcheck over the real cgo binary, with the pure-Go binary as the baseline: the
# Go runtime alone produces errors under valgrind, so only the difference between
# the two is attributable to the FFI path.
stage_valgrind() {
  ensure_build
  : >"$logs/valgrind.log"
  local corpus=$work/corpus.jsonl
  # The pathological corpus is 11MB of deliberately awful statements and memcheck
  # costs 30-100x, so it is sampled rather than replayed whole: the shapes are
  # what matter here, not the count.
  cat harness/corpus/testdata.jsonl harness/corpus/workloads.jsonl \
    harness/corpus/fuzzseeds.jsonl >"$corpus"
  awk "NR % ${PATHOLOGICAL_STRIDE:-4} == 0" harness/corpus/pathological.jsonl >>"$corpus"
  awk "NR % ${MATRIX_STRIDE:-10} == 0" harness/corpus/matrix.jsonl >>"$corpus"
  run valgrind.log wc -l "$corpus"
  for runner in gorunner ffirunner; do
    # The default error limit stays on deliberately. The Go runtime produces
    # thousands of distinct uninitialised-value contexts, and with --error-limit=no
    # valgrind's context list grows until the run no longer finishes; the leak
    # check, which is the part that matters here, is unaffected by the limit.
    # GOMAXPROCS=1 GOGC=off is not cosmetic: valgrind serializes threads, and a Go
    # binary that GCs concurrently under it goes from seconds to hours on this
    # corpus. Both runners get the same treatment, so the comparison holds; the
    # cost is that GC-time behavior is out of scope for this stage.
    run valgrind.log bash -c "GOMAXPROCS=1 GOGC=off valgrind --tool=memcheck --leak-check=full --show-leak-kinds=all \
      --num-callers=20 \
      --log-file=$logs/valgrind-$runner.txt \
      $work/$runner <$corpus >$work/$runner.out"
    run valgrind.log bash -c "grep -E '^==[0-9]+== (ERROR SUMMARY|LEAK SUMMARY|   definitely|   indirectly|   possibly|   still reachable|   suppressed)' $logs/valgrind-$runner.txt"
  done
  run valgrind.log bash -c "cmp $work/gorunner.out $work/ffirunner.out && echo 'runners agreed on every response'"
  # The comparison the criterion actually needs: which error kinds and which
  # stacks exist under the FFI binary that do not exist under the Go one.
  for runner in gorunner ffirunner; do
    grep -E '^==[0-9]+== [A-Z]' "$logs/valgrind-$runner.txt" |
      sed -E 's/^==[0-9]+== //' | sort | uniq -c | sort -rn >"$logs/valgrind-$runner-kinds.txt"
    grep -E '^==[0-9]+==    (at|by) ' "$logs/valgrind-$runner.txt" |
      sed -E 's/^==[0-9]+==    (at|by) 0x[0-9A-F]+: //' | sort -u >"$logs/valgrind-$runner-frames.txt"
  done
  run valgrind.log bash -c "comm -13 $logs/valgrind-gorunner-frames.txt $logs/valgrind-ffirunner-frames.txt | tee $logs/valgrind-ffi-only-frames.txt | wc -l"
}

# A leak in a per-call path is invisible in a short run and fatal in production.
# RSS is sampled from /proc rather than taken from the harness's own report so
# the measurement does not depend on the thing being measured.
stage_load() {
  ensure_build
  : >"$logs/load.log"
  local rss=$logs/load-rss.tsv
  echo -e "seconds\trss_kb" >"$rss"
  "$work/throughput" -corpus harness/corpus/workloads.jsonl -impl rust \
    -workers "$load_workers" -duration "$load_duration" -warmup 10s \
    -json "$logs/load-throughput.json" >>"$logs/load.log" 2>&1 &
  local pid=$!
  local start
  start=$(date +%s)
  while kill -0 "$pid" 2>/dev/null; do
    printf '%s\t%s\n' "$(($(date +%s) - start))" "$(awk '/VmRSS/{print $2}' "/proc/$pid/status" 2>/dev/null)" >>"$rss"
    sleep 5
  done
  wait "$pid"
  echo "[exit $?]" >>"$logs/load.log"
  {
    echo "RSS samples (kB): $(tail -n +2 "$rss" | wc -l)"
    awk 'NR>1 && $2!="" {if(min==""||$2<min)min=$2; if($2>max)max=$2; last=$2; if(NR==2)first=$2}
         END{printf "first=%s last=%s min=%s max=%s drift=%s kB\n", first, last, min, max, last-first}' "$rss"
  } | tee -a "$logs/load.log"
}

# The Go-side misuse suite, including the concurrency stress that is opt-in in
# the normal test run, plus the panic probe, which needs both build tags and a
# Rust archive built with the panic-probe feature.
stage_misuse() {
  : >"$logs/misuse.log"
  run misuse.log env SQLLEXER_FFI_MISUSE_STRESS=1 go test -a -count=1 -v -tags rustffi ./harness/rustffi/...
  run misuse.log env SQLLEXER_FFI_MISUSE_STRESS=1 go test -a -count=1 -race -tags rustffi ./harness/rustffi/...
  run misuse.log bash -c "cd rust && cargo build --release -p sqllexer-ffi --features panic-probe"
  run misuse.log go test -a -count=1 -v -tags "rustffi rustffi_panicprobe" -run Panic ./harness/rustffi/...
  # Leave the archive in its shipped shape: the panic probe must not be linkable
  # into anything but this test.
  run misuse.log bash -c "cd rust && cargo build --release -p sqllexer-ffi"
}

stages=("$@")
if [ ${#stages[@]} -eq 0 ]; then
  stages=(versions build asan asan-controls miri miri-controls valgrind load misuse)
fi
for stage in "${stages[@]}"; do
  echo "=== $stage"
  "stage_${stage//-/_}"
done
echo "logs in $root/$logs"
