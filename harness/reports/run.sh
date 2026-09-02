#!/usr/bin/env bash
# Produces every JSON report in this directory: the Go implementation and the Rust
# rewrite under the same sustained load, on the same host, back to back. Runs are
# strictly sequential — two load drivers sharing the host would measure each other
# rather than the library.
set -euo pipefail

cd "$(dirname "$0")/../.."
out=${OUT:-harness/reports}
mkdir -p "$out"
# Absolute, because the Rust driver runs from rust/ and OUT may itself be absolute.
out=$(cd "$out" && pwd)
corpus_dir=$(cd harness/corpus && pwd)
duration=${DURATION:-20s}
warmup=${WARMUP:-5s}

# Worker counts are derived from the host rather than fixed, so a 4-vCPU runner is
# not asked to run 8 workers. Comparing an oversubscribed host against a saturated
# one measures the scheduler, not the implementations.
cores=$(getconf _NPROCESSORS_ONLN)
read -r -a worker_counts <<<"${WORKERS:-1 $cores}"

(cd rust && cargo build --release)
go build -o "$out/.throughput" ./harness/cmd/throughput
trap 'rm -f "$out/.throughput"' EXIT

{
  echo "cores=$cores"
  echo "workers=${worker_counts[*]}"
  echo "duration=$duration warmup=$warmup"
  echo "arch=$(uname -m) kernel=$(uname -r)"
  go version
  (cd rust && cargo --version && rustc --version)
} >"$out/environment.txt"

for workers in "${worker_counts[@]}"; do
  for corpus in workloads pathological; do
    "$out/.throughput" -corpus "$corpus_dir/$corpus.jsonl" -workers "$workers" \
      -duration "$duration" -warmup "$warmup" \
      -json "$out/$corpus-w$workers-go.json"
    rust/target/release/bench \
      --corpus "$corpus_dir/$corpus.jsonl" --workers "$workers" \
      --duration "${duration%s}" --warmup "${warmup%s}" \
      --json "$out/$corpus-w$workers-rust.json"
  done
done
