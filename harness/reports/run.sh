#!/usr/bin/env bash
# Produces every JSON report in this directory. Runs are strictly sequential: two
# load drivers sharing the host would measure each other rather than the library.
set -euo pipefail

cd "$(dirname "$0")/../.."
out=harness/reports
duration=${DURATION:-20s}
warmup=${WARMUP:-5s}

(cd rust && cargo build --release)
# -a: Go's build cache keys on source content, not on the static archive named in
# CGO LDFLAGS, so a rebuilt libsqllexer_ffi.a would otherwise be a cache hit and the
# binary would silently keep the previous Rust code.
go build -a -tags rustffi -o "$out/.throughput" ./harness/cmd/throughput
trap 'rm -f "$out/.throughput"' EXIT

for workers in 1 8; do
  for corpus in workloads pathological; do
    for impl in go rust; do
      "$out/.throughput" -corpus "harness/corpus/$corpus.jsonl" -workers "$workers" \
        -duration "$duration" -warmup "$warmup" -impl "$impl" \
        -json "$out/$corpus-w$workers-$impl.json"
    done
    (cd rust && ./target/release/bench \
      --corpus "../harness/corpus/$corpus.jsonl" --workers "$workers" \
      --duration "${duration%s}" --warmup "${warmup%s}" \
      --json "../$out/$corpus-w$workers-rust-native.json")
  done
done
