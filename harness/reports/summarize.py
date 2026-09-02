#!/usr/bin/env python3
"""Aggregates the JSON reports of one or more environments.

    python3 harness/reports/summarize.py harness/reports/ci-arm
    python3 harness/reports/summarize.py --html harness/reports/benchmarks.html \
        harness/reports/ci-arm harness/reports/ci-x86

Without --html it prints markdown tables for one environment. With --html it
writes a single self-contained page putting Go and Rust side by side for every
environment, corpus and worker count, with the run-to-run spread — which is the
number that says whether a gate can be enforced on that hardware at all.
"""

import argparse
import html
import json
import statistics
import sys
from pathlib import Path

ENGINES = ["go", "rust"]
CORPORA = ["workloads", "pathological"]
CLASSES = ["W1-short", "W2-medium", "W3-large", "W4-pathological"]


def load(root: Path) -> dict[tuple[str, int, str], list[dict]]:
    runs: dict[tuple[str, int, str], list[dict]] = {}
    for run_dir in sorted(root.glob("run*")):
        for path in run_dir.glob("*.json"):
            corpus, workers, engine = path.stem.split("-", 2)
            runs.setdefault((corpus, int(workers[1:]), engine), []).append(
                json.loads(path.read_text())
            )
    return runs


def environment(root: Path) -> str:
    for run_dir in sorted(root.glob("run*")):
        env = run_dir / "environment.txt"
        if env.exists():
            return env.read_text().strip()
    return ""


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


def mean(reports, *path):
    values = []
    for report in reports:
        node = report
        for key in path:
            node = node[key]
        values.append(node)
    return statistics.mean(values)


def print_markdown(root: Path):
    runs = load(root)
    # Worker counts come from the reports: they follow the host's core count.
    workers_seen = sorted({key[1] for key in runs})
    print(f"# {root.name}  ({len(list(root.glob('run*')))} runs)\n")
    for corpus in CORPORA:
        for workers in workers_seen:
            rows = []
            base = None
            for engine in ENGINES:
                reports = runs.get((corpus, workers, engine))
                if not reports:
                    continue
                ops = [r["ops_per_second"] for r in reports]
                avg = statistics.mean(ops)
                if engine == "go":
                    base = avg
                rows.append(
                    "| %s | %.0f | ±%.1f%% | %s | %.0f | %.2f | %.1f |"
                    % (
                        engine,
                        avg,
                        spread(ops),
                        "1.00x" if base is None else "%.2fx" % (avg / base),
                        mean(reports, "memory", "bytes_per_op"),
                        mean(reports, "memory", "allocs_per_op"),
                        mean(reports, "memory", "rss_peak_mb"),
                    )
                )
            if not rows:
                continue
            print(f"## {corpus}, {workers} worker(s)\n")
            print("| engine | ops/s | spread | vs go | B/op | allocs/op | RSS MB |")
            print("| --- | --- | --- | --- | --- | --- | --- |")
            print("\n".join(rows))
            print()

            for name in CLASSES:
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


STYLE = """
body { font: 14px/1.5 -apple-system, Segoe UI, Roboto, sans-serif; margin: 2rem auto;
       max-width: 68rem; color: #1b1b1f; padding: 0 1rem; }
h1 { margin-bottom: .2rem; }
h2 { margin-top: 2.4rem; border-bottom: 1px solid #ddd; padding-bottom: .3rem; }
h3 { margin-top: 1.6rem; font-size: 1rem; color: #444; }
p.lede { color: #555; margin-top: 0; }
table { border-collapse: collapse; width: 100%; margin: .6rem 0 1.4rem; }
th, td { border: 1px solid #dcdce0; padding: .35rem .6rem; text-align: right; }
th { background: #f5f5f7; font-weight: 600; }
th:first-child, td:first-child { text-align: left; }
td.go { background: #fbfbfc; }
td.rust { background: #f4f9f4; }
td.ratio { font-weight: 600; }
.win { color: #1a7f37; }
.loss { color: #b3261e; }
.spread { color: #777; font-weight: 400; font-size: .85em; }
pre { background: #f5f5f7; padding: .8rem; border-radius: 4px; overflow-x: auto;
      font-size: .85em; }
footer { color: #777; font-size: .85em; margin-top: 3rem; }
"""


def ratio_cell(value: float, higher_is_better: bool) -> str:
    good = value >= 1.0 if higher_is_better else value <= 1.0
    return '<td class="ratio %s">%.2f×</td>' % ("win" if good else "loss", value)


def html_environment(root: Path) -> str:
    text = environment(root)
    return f"<pre>{html.escape(text)}</pre>" if text else ""


def html_environment_section(root: Path) -> str:
    count = len(list(root.glob("run*")))
    return (
        f"<h2>{html.escape(root.name)}</h2>"
        f"<p class='lede'>{count} full repeats of the matrix; every cell is the mean "
        "across them, and Go and Rust ran back to back on this host.</p>"
        + html_environment(root)
    )


def html_throughput(runs, corpus: str, workers: int) -> str:
    go = runs.get((corpus, workers, "go"))
    rust = runs.get((corpus, workers, "rust"))
    if not go or not rust:
        return ""

    def row(label, path, fmt, higher_is_better):
        g, r = mean(go, *path), mean(rust, *path)
        ratio = (g / r) if not higher_is_better else (r / g)
        # For "lower is better" metrics the ratio reads as "Go over Rust", so a
        # value above 1 always means Rust is ahead in both directions.
        return (
            f"<tr><td>{label}</td>"
            f'<td class="go">{fmt % g}</td>'
            f'<td class="rust">{fmt % r}</td>' + ratio_cell(ratio, True) + "</tr>"
        )

    go_ops = [r["ops_per_second"] for r in go]
    rust_ops = [r["ops_per_second"] for r in rust]
    out = [
        f"<h3>{corpus}.jsonl — {workers} worker(s)</h3>",
        (
            "<table><thead><tr><th>metric</th><th>go</th><th>rust</th>"
            "<th>rust advantage</th></tr></thead><tbody>"
        ),
        "<tr><td>throughput (ops/s)</td>"
        f'<td class="go">{statistics.mean(go_ops):,.0f}'
        f'<span class="spread"> ±{spread(go_ops):.1f}%</span></td>'
        f'<td class="rust">{statistics.mean(rust_ops):,.0f}'
        f'<span class="spread"> ±{spread(rust_ops):.1f}%</span></td>'
        + ratio_cell(statistics.mean(rust_ops) / statistics.mean(go_ops), True)
        + "</tr>",
        row("bytes allocated / statement", ("memory", "bytes_per_op"), "%.0f", False),
        row("allocations / statement", ("memory", "allocs_per_op"), "%.2f", False),
        row("peak RSS (MB)", ("memory", "rss_peak_mb"), "%.1f", False),
        "</tbody></table>",
    ]
    if corpus == "pathological":
        out.append(
            "<p class='lede'>Allocation <em>count</em> inverts on this corpus and is "
            "not a like-for-like metric: Rust grows reusable buffers in many small "
            "steps where Go takes a few huge ones, which is why it loses on count "
            "while allocating ~290× fewer bytes. Total bytes is the honest measure "
            "here.</p>"
        )

    go_classes, rust_classes = classes(go), classes(rust)
    rows = []
    for name in CLASSES:
        g, r = go_classes.get(name), rust_classes.get(name)
        if not g or not r:
            continue
        cells = [f"<td>{name}</td>"]
        for percentile in ("p50_ns", "p99_ns"):
            gv = statistics.mean([c[percentile] for c in g]) / 1000
            rv = statistics.mean([c[percentile] for c in r]) / 1000
            cells.append(f'<td class="go">{gv:.3f}µs</td>')
            cells.append(f'<td class="rust">{rv:.3f}µs</td>')
            cells.append(ratio_cell(gv / rv, True))
        rows.append("<tr>" + "".join(cells) + "</tr>")
    if rows:
        out += [
            (
                "<table><thead><tr><th>class</th>"
                "<th>go p50</th><th>rust p50</th><th>p50 advantage</th>"
                "<th>go p99</th><th>rust p99</th><th>p99 advantage</th>"
                "</tr></thead><tbody>"
            ),
            "".join(rows),
            "</tbody></table>",
        ]
    return "".join(out)


def write_html(roots, out_path: Path):
    parts = [
        "<!doctype html><html lang='en'><head><meta charset='utf-8'>",
        "<title>go-sqllexer: Rust vs Go benchmarks</title>",
        f"<style>{STYLE}</style></head><body>",
        "<h1>go-sqllexer: Rust vs Go</h1>",
        (
            "<p class='lede'>Sustained-load comparison of the two parallel "
            "implementations. Each environment ran Go and Rust back to back on one "
            "host; only the ratio travels between hosts, never the absolute "
            "ops/s.</p>"
        ),
    ]
    for root in roots:
        runs = load(root)
        if not runs:
            continue
        parts.append(html_environment_section(root))
        workers_seen = sorted({key[1] for key in runs})
        for corpus in CORPORA:
            for workers in workers_seen:
                parts.append(html_throughput(runs, corpus, workers))
    parts.append(
        "<footer>Generated by harness/reports/summarize.py from the JSON reports "
        "in this directory. Regenerate with: python3 harness/reports/summarize.py "
        "--html harness/reports/benchmarks.html harness/reports/ci-arm "
        "harness/reports/ci-x86</footer></body></html>"
    )
    out_path.write_text("".join(parts))
    print(f"wrote {out_path}")


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("roots", nargs="+", type=Path, help="report directories")
    parser.add_argument("--html", type=Path, help="write a side-by-side HTML report")
    args = parser.parse_args(argv)

    if args.html:
        write_html(args.roots, args.html)
    else:
        for root in args.roots:
            print_markdown(root)


if __name__ == "__main__":
    main(sys.argv[1:])
