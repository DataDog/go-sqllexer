#!/usr/bin/env python3
"""Aggregates the JSON reports of one environment into comparison tables.

    python3 harness/reports/summarize.py harness/reports/x86-dedicated

Prints, per corpus and worker count, the three engines side by side with the
run-to-run spread across the run*/ subdirectories, which is the number that says
whether a gate can be enforced on that hardware at all.
"""

import json
import statistics
import sys
from pathlib import Path

ENGINES = ["go", "rust", "rust-native"]
CORPORA = ["workloads", "pathological"]
WORKERS = [1, 8]


def load(root: Path):
    runs = {}
    for run_dir in sorted(root.glob("run*")):
        for path in run_dir.glob("*.json"):
            corpus, workers, engine = path.stem.split("-", 2)
            runs.setdefault((corpus, int(workers[1:]), engine), []).append(
                json.loads(path.read_text())
            )
    return runs


def spread(values):
    """Half-width of the range as a percentage of the mean."""
    if len(values) < 2 or not statistics.mean(values):
        return 0.0
    return (max(values) - min(values)) / 2 / statistics.mean(values) * 100


def classes(reports):
    out = {}
    for report in reports:
        for c in report["classes"]:
            out.setdefault(c["class"], []).append(c)
    return out


def main(root: Path):
    runs = load(root)
    print(f"# {root.name}  ({len(list(root.glob('run*')))} runs)\n")
    for corpus in CORPORA:
        for workers in WORKERS:
            rows = []
            base = None
            for engine in ENGINES:
                reports = runs.get((corpus, workers, engine))
                if not reports:
                    continue
                ops = [r["ops_per_second"] for r in reports]
                mean = statistics.mean(ops)
                if engine == "go":
                    base = mean
                mem = reports[0]["memory"]
                rows.append(
                    "| %s | %.0f | ±%.1f%% | %s | %.0f | %.2f | %.1f |"
                    % (
                        engine,
                        mean,
                        spread(ops),
                        "1.00x" if base is None else "%.2fx" % (mean / base),
                        statistics.mean([r["memory"]["bytes_per_op"] for r in reports]),
                        statistics.mean([r["memory"]["allocs_per_op"] for r in reports]),
                        statistics.mean([r["memory"]["rss_peak_mb"] for r in reports]),
                    )
                )
            if not rows:
                continue
            print(f"## {corpus}, {workers} worker(s)\n")
            print("| engine | ops/s | spread | vs go | B/op | allocs/op | RSS MB |")
            print("| --- | --- | --- | --- | --- | --- | --- |")
            print("\n".join(rows))
            print()

            for name in ["W1-short", "W2-medium", "W3-large", "W4-pathological"]:
                line = []
                for engine in ENGINES:
                    reports = runs.get((corpus, workers, engine))
                    if not reports:
                        continue
                    byclass = classes(reports).get(name)
                    if not byclass:
                        continue
                    line.append(
                        "%s p50 %.3fµs (±%.1f%%) p99 %.3fµs (±%.1f%%) p999 %.3fµs"
                        % (
                            engine,
                            statistics.mean([c["p50_ns"] for c in byclass]) / 1000,
                            spread([c["p50_ns"] for c in byclass]),
                            statistics.mean([c["p99_ns"] for c in byclass]) / 1000,
                            spread([c["p99_ns"] for c in byclass]),
                            statistics.mean([c["p999_ns"] for c in byclass]) / 1000,
                        )
                    )
                if line:
                    print(f"- {name}: " + " | ".join(line))
            print()


if __name__ == "__main__":
    main(Path(sys.argv[1]))
