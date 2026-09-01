# Measured baseline

Two implementations, one corpus, one host, sequential runs. Reproduce with
[`run.sh`](run.sh); the JSON files under `ci-arm/` and `ci-x86/` are the raw output,
and [`summarize.py`](summarize.py) turns a directory of runs into the tables below.

- **go** — the current implementation, one obfuscator/normalizer pair per worker.
- **rust** — the Rust rewrite, same reuse, no binding in the path.

Only the ratio between the two travels between hosts. Absolute ops/s does not: the
runners differ in core count, clock and memory bandwidth, which is why Go and Rust
are always measured back to back on the same machine and why worker counts follow
the host's core count instead of being fixed.

Latency percentiles come from a fixed-size log-linear histogram (~0.1% bucket
error) rather than stored samples, in both drivers. That matters for more than
memory footprint: keeping every sample made the harness's own RSS a function of how
many operations an implementation completed, which turned the faster engine into
the one that appeared to use more memory.

## Results

See [CROSS-PLATFORM.md](CROSS-PLATFORM.md) for the ARM and x86 matrices, the
run-to-run spread, and which acceptance gates each one supports.
