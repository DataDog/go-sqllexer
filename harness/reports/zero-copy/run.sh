#!/usr/bin/env bash
# Before/after measurements for the binding optimization described in
# ../ZERO-COPY.md. Same driver, same corpora, same host as ../run.sh; the only
# difference is that the phase (before|after) is part of the file name so the two
# sets can be diffed.
#
#   harness/reports/zero-copy/run.sh before          # on the unmodified binding
#   harness/reports/zero-copy/run.sh after           # after the change
#   IMPLS="rust rust-borrowed" harness/reports/zero-copy/run.sh after
#
# Runs are strictly sequential: two load drivers sharing the host would measure
# each other rather than the library.
set -euo pipefail

phase=${1:?usage: run.sh <before|after>}
cd "$(dirname "$0")/../../.."
out=harness/reports/zero-copy
duration=${DURATION:-20s}
warmup=${WARMUP:-5s}
impls=${IMPLS:-"go rust"}

(cd rust && cargo build --release)
# -a: Go's build cache keys on source content, not on the static archive named in
# CGO LDFLAGS, so a rebuilt libsqllexer_ffi.a would otherwise be a cache hit and the
# binary would silently keep the previous Rust code.
go build -a -tags rustffi -o "$out/.throughput" ./harness/cmd/throughput
trap 'rm -f "$out/.throughput"' EXIT

for workers in 1 8; do
  for corpus in workloads pathological; do
    for impl in $impls; do
      "$out/.throughput" -corpus "harness/corpus/$corpus.jsonl" -workers "$workers" \
        -duration "$duration" -warmup "$warmup" -impl "$impl" \
        -json "$out/$phase-$corpus-w$workers-$impl.json"
    done
  done
done
