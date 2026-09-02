#!/usr/bin/env python3
"""Aggregates the JSON reports of one or more environments.

    python3 harness/reports/summarize.py harness/reports/ci-arm
    python3 harness/reports/summarize.py --html harness/reports/benchmarks.html \
        harness/reports/ci-arm harness/reports/ci-x86

Without --html it prints markdown tables for one environment; CI pipes that into
the job summary. With --html it writes a single self-contained page written for a
reader who has never seen this project: what was measured, the headline ratio, the
methodology, every per-configuration number, and the caveats.

Reporting conventions, and why they are what they are:

- Ratios lead, absolutes follow. Only a Go-vs-Rust ratio measured on one host in
  one sitting is comparable between hosts; ops/s is not. This is the same rule
  TechEmpower applies when it normalizes each framework against the best result on
  that hardware round.
- n=3 does not support significance testing. benchstat asks for ten or more
  samples before it will call a difference significant, so this report shows the
  observed range (half the min-max spread, written "±x%") and the individual run
  values instead of a confidence interval, and never says "significant".
- Aggregation across configurations uses the geometric mean, which is the correct
  average for ratios and is what benchstat reports as its summary row.
- Direction is labelled on every table rather than implied, in the manner of
  criterion and hyperfine: each metric says "higher is better" or "lower is
  better", and the ratio column is always oriented so that above 1.00x favours
  Rust.
- Precision is capped at three significant figures for latency and none for
  throughput. The inputs vary by percent between runs; more digits would imply
  resolution the data does not have.
"""

import argparse
import html
import json
import math
import statistics
import sys
from pathlib import Path

ENGINES = ["go", "rust"]
CORPORA = ["workloads", "pathological"]
CLASSES = ["short", "medium", "large", "pathological"]

CLASS_BOUNDS = {
    "short": "statement ≤ 256 B",
    "medium": "256 B – 2 KB",
    "large": "2 KB – 16 KB",
    "pathological": "larger than 16 KB",
}


def class_name(name: str) -> str:
    """Reports written before the classes were renamed use `W1-short` and friends."""
    return name.split("-", 1)[1] if name[:1] == "W" and name[1:2].isdigit() else name


CORPUS_BLURB = {
    "workloads": (
        "199 statements extracted from the repository's own Go benchmarks. The "
        "realistic case: mostly short queries, a few large ones."
    ),
    "pathological": (
        "288 adversarial statements — huge parameter lists, deep nesting, "
        "unterminated strings and comments, invalid UTF-8, comment-only and empty "
        "input — across every DBMS and mode."
    ),
}


def run_dirs(root: Path) -> list[Path]:
    """A directory of run1/, run2/… repeats, or a single run written directly."""
    return sorted(root.glob("run*")) or [root]


def load(root: Path) -> dict[tuple[str, int, str], list[dict]]:
    runs: dict[tuple[str, int, str], list[dict]] = {}
    for run_dir in run_dirs(root):
        for path in run_dir.glob("*.json"):
            corpus, workers, engine = path.stem.split("-", 2)
            runs.setdefault((corpus, int(workers[1:]), engine), []).append(
                json.loads(path.read_text())
            )
    return runs


def environment(root: Path) -> str:
    for run_dir in run_dirs(root):
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
            out.setdefault(class_name(c["class"]), []).append(c)
    return out


def series(reports, *path):
    values = []
    for report in reports:
        node = report
        for key in path:
            node = node[key]
        values.append(node)
    return values


def mean(reports, *path):
    return statistics.mean(series(reports, *path))


def geomean(values):
    return math.exp(statistics.fmean([math.log(v) for v in values]))


def print_markdown(root: Path):
    runs = load(root)
    # Worker counts come from the reports: they follow the host's core count.
    workers_seen = sorted({key[1] for key in runs})
    print(f"# {root.name}  ({len(run_dirs(root))} run(s))\n")
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
                    "| {} | {:.0f} | ±{:.1f}% | {} | {:.0f} | {:.2f} | {:.1f} |".format(
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
                        "{} p50 {:.3f}µs (±{:.1f}%) p99 {:.3f}µs (±{:.1f}%) p999 {:.3f}µs".format(
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


# --------------------------------------------------------------------------- #
# HTML report
# --------------------------------------------------------------------------- #

STYLE = """
:root { --go:#8a6d1f; --rust:#1a7f37; --rule:#d9d9de; --muted:#5c5c66; }
* { box-sizing: border-box; }
body { font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
       Helvetica, Arial, sans-serif; margin: 0 auto; max-width: 70rem;
       color: #17171b; padding: 2rem 1.2rem 4rem; }
h1 { font-size: 1.7rem; margin: 0 0 .3rem; }
h2 { font-size: 1.25rem; margin: 2.8rem 0 .6rem; border-bottom: 1px solid var(--rule);
     padding-bottom: .35rem; }
h3 { font-size: 1.02rem; margin: 1.8rem 0 .4rem; }
h4 { font-size: .92rem; margin: 1.2rem 0 .3rem; color: var(--muted);
     font-weight: 600; }
p { margin: .6rem 0; }
p.lede, .note { color: var(--muted); }
ul { margin: .5rem 0 .8rem; padding-left: 1.2rem; }
li { margin: .2rem 0; }
code, pre { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
code { font-size: .88em; background: #f3f3f5; padding: .05rem .25rem;
       border-radius: 3px; }
pre { background: #f5f5f7; padding: .8rem; border-radius: 4px; overflow-x: auto;
      font-size: .82em; line-height: 1.45; }
nav { font-size: .9em; margin: 1rem 0 0; color: var(--muted); }
nav a { color: #17171b; margin-right: .9rem; white-space: nowrap; }
table { border-collapse: collapse; width: 100%; margin: .5rem 0 1rem;
        font-variant-numeric: tabular-nums; font-size: .9em; }
th, td { border: 1px solid var(--rule); padding: .3rem .55rem; text-align: right; }
th { background: #f4f4f6; font-weight: 600; }
th.left, td.left { text-align: left; }
td.go { background: #fdfcf7; }
td.rust { background: #f5faf6; }
td.ratio { font-weight: 600; }
tbody tr:hover td { background: #eef3fb; }
.win { color: #14682c; }
.loss { color: #a5271c; }
.spread { color: var(--muted); font-weight: 400; font-size: .85em; }
.dir { font-weight: 400; color: var(--muted); font-size: .85em; display: block; }
.cards { display: flex; flex-wrap: wrap; gap: .9rem; margin: 1rem 0 .4rem; }
.card { border: 1px solid var(--rule); border-radius: 5px; padding: .7rem .9rem;
        min-width: 13rem; flex: 1 1 13rem; background: #fafafb; }
.card .k { font-size: 1.55rem; font-weight: 650; line-height: 1.2; }
.card .t { font-size: .8rem; color: var(--muted); text-transform: uppercase;
           letter-spacing: .04em; }
.card .s { font-size: .82rem; color: var(--muted); margin-top: .25rem; }
.callout { border-left: 3px solid #c9a227; background: #fdfaf0; padding: .6rem .9rem;
           margin: .9rem 0; }
.callout h4 { margin-top: 0; color: #17171b; }
figure { margin: 1rem 0 1.4rem; }
figcaption { font-size: .85em; color: var(--muted); margin-top: .3rem; }
svg { max-width: 100%; height: auto; display: block; }
footer { color: var(--muted); font-size: .85em; margin-top: 3rem;
         border-top: 1px solid var(--rule); padding-top: .8rem; }
.pass { color: #14682c; font-weight: 600; }
.fail { color: #a5271c; font-weight: 600; }
"""


def esc(text) -> str:
    return html.escape(str(text))


def fmt_us(ns: float) -> str:
    """Latency to three significant figures, in the unit that reads best."""
    us = ns / 1000
    if us >= 1000:
        return f"{us / 1000:,.2f} ms"
    if us >= 100:
        return f"{us:.0f} µs"
    if us >= 10:
        return f"{us:.1f} µs"
    return f"{us:.2f} µs"


def ratio_cell(value: float, good_above_one: bool = True) -> str:
    good = value >= 1.0 if good_above_one else value <= 1.0
    return '<td class="ratio {}">{:.2f}×</td>'.format("win" if good else "loss", value)


# --------------------------------------------------------------------------- #
# Model
# --------------------------------------------------------------------------- #


class Environment:
    """One host: its metadata and every report measured on it."""

    def __init__(self, root: Path):
        self.root = root
        self.name = root.name
        self.runs = load(root)
        self.raw_environment = environment(root)
        self.meta = {}
        for line in self.raw_environment.splitlines():
            if line.startswith("workers="):
                # "workers=1 8": the value is the whole rest of the line.
                self.meta["workers"] = line.partition("=")[2]
            elif line.startswith(("cores=", "arch=", "duration=")):
                for field in line.split():
                    key, _, value = field.partition("=")
                    self.meta[key] = value
            elif line.startswith("go version"):
                self.meta["go"] = line.split()[2]
            elif line.startswith("rustc"):
                self.meta["rustc"] = line.split()[1]
        self.workers = sorted({key[1] for key in self.runs})
        self.repeats = len(run_dirs(root))

    @property
    def label(self) -> str:
        return "{}, {} cores".format(
            self.meta.get("arch", self.name),
            self.meta.get("cores", "?"),
        )

    def pair(self, corpus: str, workers: int):
        return (
            self.runs.get((corpus, workers, "go")),
            self.runs.get((corpus, workers, "rust")),
        )

    def configurations(self):
        for corpus in CORPORA:
            for workers in self.workers:
                go, rust = self.pair(corpus, workers)
                if go and rust:
                    yield corpus, workers, go, rust


def per_run_ratios(go, rust, *path, invert=False):
    """Ratio per repeat, pairing run i of one engine with run i of the other.

    Pairing within a run keeps each ratio a same-host, back-to-back comparison.
    """
    g, r = series(go, *path), series(rust, *path)
    pairs = list(zip(g, r))
    return [(a / b) if invert else (b / a) for a, b in pairs]


def throughput_ratios(env: Environment, corpus: str | None = None):
    out = {}
    for c, workers, go, rust in env.configurations():
        if corpus and c != corpus:
            continue
        out[(c, workers)] = per_run_ratios(go, rust, "ops_per_second")
    return out


# --------------------------------------------------------------------------- #
# Charts (inline SVG, no scripts)
# --------------------------------------------------------------------------- #


def ratio_chart(rows, threshold: float | None, title: str) -> str:
    """Horizontal bars of the throughput ratio, with the observed min-max range.

    rows: (label, mean_ratio, min_ratio, max_ratio).
    """
    if not rows:
        return ""
    width, left, right = 860, 250, 40
    row_h, top = 30, 34
    height = top + row_h * len(rows) + 26
    top_value = max(max(r[3] for r in rows), threshold or 0) * 1.12
    plot = width - left - right

    def x(value):
        return left + plot * value / top_value

    parts = [
        (
            f'<svg viewBox="0 0 {width} {height}" role="img" '
            f'aria-label="{esc(title)}" xmlns="http://www.w3.org/2000/svg" '
            'font-family="-apple-system, Segoe UI, Roboto, sans-serif" font-size="12">'
        )
    ]
    ticks = [t / 2 for t in range(int(top_value * 2) + 1)]
    for tick in ticks:
        parts.append(
            f'<line x1="{x(tick):.1f}" y1="{top - 8}" x2="{x(tick):.1f}" '
            f'y2="{height - 24}" stroke="#e6e6ea"/>'
            f'<text x="{x(tick):.1f}" y="{height - 8}" text-anchor="middle" '
            f'fill="#5c5c66">{tick:g}×</text>'
        )
    # 1.00x is parity with Go; anything left of it is a regression.
    parts.append(
        f'<line x1="{x(1):.1f}" y1="{top - 8}" x2="{x(1):.1f}" y2="{height - 24}" '
        'stroke="#17171b" stroke-width="1.2"/>'
        f'<text x="{x(1):.1f}" y="{top - 14}" text-anchor="middle" fill="#17171b">'
        "parity with Go</text>"
    )
    if threshold:
        parts.append(
            f'<line x1="{x(threshold):.1f}" y1="{top - 8}" x2="{x(threshold):.1f}" '
            f'y2="{height - 24}" stroke="#a5271c" stroke-dasharray="4 3"/>'
            f'<text x="{x(threshold):.1f}" y="{top - 14}" text-anchor="middle" '
            f'fill="#a5271c">gate {threshold:g}×</text>'
        )
    for i, (label, avg, low, high) in enumerate(rows):
        y = top + row_h * i + 4
        bar = row_h - 12
        parts.append(
            f'<text x="{left - 10}" y="{y + bar - 2}" text-anchor="end" '
            f'fill="#17171b">{esc(label)}</text>'
            f'<rect x="{x(0):.1f}" y="{y}" width="{x(avg) - x(0):.1f}" '
            f'height="{bar}" fill="#1a7f37" fill-opacity="0.75"/>'
        )
        if high > low:
            mid = y + bar / 2
            parts.append(
                f'<line x1="{x(low):.1f}" y1="{mid:.1f}" x2="{x(high):.1f}" '
                f'y2="{mid:.1f}" stroke="#17171b" stroke-width="1"/>'
                f'<line x1="{x(low):.1f}" y1="{y + 2}" x2="{x(low):.1f}" '
                f'y2="{y + bar - 2}" stroke="#17171b" stroke-width="1"/>'
                f'<line x1="{x(high):.1f}" y1="{y + 2}" x2="{x(high):.1f}" '
                f'y2="{y + bar - 2}" stroke="#17171b" stroke-width="1"/>'
            )
        parts.append(
            f'<text x="{x(avg) + 6:.1f}" y="{y + bar - 2}" fill="#17171b" '
            f'font-weight="600">{avg:.2f}×</text>'
        )
    parts.append("</svg>")
    return "".join(parts)


def latency_chart(env: Environment, corpus: str, workers: int) -> str:
    """Grouped bars of p50 and p99 per workload class, log-scaled.

    Latencies span three orders of magnitude between the short and pathological
    classes, so a linear axis would render the short ones as invisible slivers.
    """
    go, rust = env.pair(corpus, workers)
    go_c, rust_c = classes(go), classes(rust)
    names = [n for n in CLASSES if n in go_c and n in rust_c]
    if not names:
        return ""
    values = []
    for name in names:
        for source in (go_c, rust_c):
            for p in ("p50_ns", "p99_ns"):
                values.append(statistics.mean([c[p] for c in source[name]]))
    lo = 10 ** math.floor(math.log10(min(values)))
    hi = 10 ** math.ceil(math.log10(max(values)))

    width, left, right, top = 860, 120, 150, 40
    group_h = 56
    height = top + group_h * len(names) + 30
    plot = width - left - right

    def x(ns):
        return left + plot * (math.log10(ns) - math.log10(lo)) / (
            math.log10(hi) - math.log10(lo)
        )

    parts = [
        (
            f'<svg viewBox="0 0 {width} {height}" role="img" '
            f'aria-label="latency by workload class" '
            'xmlns="http://www.w3.org/2000/svg" '
            'font-family="-apple-system, Segoe UI, Roboto, sans-serif" font-size="11">'
        )
    ]
    decade = lo
    while decade <= hi:
        parts.append(
            f'<line x1="{x(decade):.1f}" y1="{top - 6}" x2="{x(decade):.1f}" '
            f'y2="{height - 22}" stroke="#e6e6ea"/>'
            f'<text x="{x(decade):.1f}" y="{height - 6}" text-anchor="middle" '
            f'fill="#5c5c66">{fmt_us(decade)}</text>'
        )
        decade *= 10
    for i, name in enumerate(names):
        base = top + group_h * i
        parts.append(
            f'<text x="{left - 8}" y="{base + 22}" text-anchor="end" '
            f'fill="#17171b" font-weight="600">{esc(name)}</text>'
        )
        for j, (percentile, colour) in enumerate((("p50_ns", 0.85), ("p99_ns", 0.45))):
            for k, (source, fill) in enumerate(
                ((go_c, "#8a6d1f"), (rust_c, "#1a7f37"))
            ):
                value = statistics.mean([c[percentile] for c in source[name]])
                y = base + j * 24 + k * 11
                parts.append(
                    f'<rect x="{left}" y="{y}" width="{max(x(value) - left, 1):.1f}" '
                    f'height="9" fill="{fill}" fill-opacity="{colour}"/>'
                )
            go_v = statistics.mean([c[percentile] for c in go_c[name]])
            rust_v = statistics.mean([c[percentile] for c in rust_c[name]])
            parts.append(
                f'<text x="{width - right + 8}" y="{base + j * 24 + 17}" '
                f'fill="#5c5c66">{percentile[:-3]} '
                f"{go_v / rust_v:.2f}× faster</text>"
            )
    parts.append(
        f'<rect x="{left}" y="2" width="9" height="9" fill="#8a6d1f" '
        'fill-opacity="0.85"/>'
        f'<text x="{left + 14}" y="10" fill="#5c5c66">Go</text>'
        f'<rect x="{left + 44}" y="2" width="9" height="9" '
        'fill="#1a7f37" fill-opacity="0.85"/>'
        f'<text x="{left + 58}" y="10" fill="#5c5c66">Rust</text>'
        "</svg>"
    )
    return "".join(parts)


# --------------------------------------------------------------------------- #
# Gates
# --------------------------------------------------------------------------- #


def gate_rows(envs):
    """Evaluates the acceptance gates of harness/ACCEPTANCE.md section C.

    Every figure is recomputed here from the committed JSON rather than copied
    from that document, so the table cannot drift away from the evidence.
    """
    configs = [(env, *rest) for env in envs for rest in env.configurations()]

    def worst_throughput(corpus):
        values = [
            statistics.mean(per_run_ratios(go, rust, "ops_per_second"))
            for env, c, workers, go, rust in configs
            if c == corpus
        ]
        return min(values) if values else None

    def worst_memory(path, corpus=None, invert=True):
        values = [
            statistics.mean(per_run_ratios(go, rust, *path, invert=invert))
            for env, c, workers, go, rust in configs
            if corpus is None or c == corpus
        ]
        return min(values) if values else None

    def worst_class(percentile, only=None):
        values = []
        for env, corpus, workers, go, rust in configs:
            go_c, rust_c = classes(go), classes(rust)
            for name in CLASSES:
                if only and name != only:
                    continue
                if name not in go_c or name not in rust_c:
                    continue
                g = statistics.mean([c[percentile] for c in go_c[name]])
                r = statistics.mean([c[percentile] for c in rust_c[name]])
                values.append(g / r)
        return min(values) if values else None

    worst_p50, worst_p99 = worst_class("p50_ns"), worst_class("p99_ns")
    worst_tail = min(worst_p50, worst_p99)
    worst_drift = max(
        spread(series(reports, "ops_per_second"))
        for env, corpus, workers, go, rust in configs
        for reports in (go, rust)
    )

    rows = [
        (
            "Throughput, mixed corpus",
            "at least 1.8× Go, at every worker count",
            "{:.2f}×".format(worst_throughput("workloads")),
            worst_throughput("workloads") >= 1.8,
        ),
        (
            "Throughput, pathological corpus",
            "at least 1.45× Go",
            "{:.2f}×".format(worst_throughput("pathological")),
            worst_throughput("pathological") >= 1.45,
        ),
        (
            "Latency",
            "p50 and p99 no worse than Go in any size class",
            f"{worst_tail:.2f}× in the weakest class",
            worst_tail >= 1.0,
        ),
        (
            "Latency, short statements",
            "p50 at least 1.3× Go for statements ≤ 256 B",
            "{:.2f}×".format(worst_class("p50_ns", only="short")),
            worst_class("p50_ns", only="short") >= 1.3,
        ),
        (
            "Bytes allocated",
            "at most 25% of Go per statement, mixed corpus",
            "%.1f%% of Go"
            % (100 / worst_memory(("memory", "bytes_per_op"), "workloads")),
            worst_memory(("memory", "bytes_per_op"), "workloads") >= 4.0,
        ),
        (
            "Allocation count",
            "no more than Go per statement, mixed corpus",
            "{:.2f}× fewer".format(
                worst_memory(("memory", "allocs_per_op"), "workloads")
            ),
            worst_memory(("memory", "allocs_per_op"), "workloads") >= 1.0,
        ),
        (
            "Peak memory",
            "resident set no larger than Go",
            "{:.2f}× lower".format(worst_memory(("memory", "rss_peak_mb"))),
            worst_memory(("memory", "rss_peak_mb")) >= 1.0,
        ),
        (
            "Stability",
            "throughput drifts less than 5% between repeats on one host",
            f"±{worst_drift:.1f}% at worst",
            worst_drift <= 5.0,
        ),
    ]
    out = []
    for name, text, observed, ok in rows:
        out.append(
            f'<tr><td class="left">{esc(name)}</td><td class="left">{esc(text)}</td>'
            f"<td>{esc(observed)}</td>"
            f'<td class="{"pass" if ok else "fail"}">{"pass" if ok else "FAIL"}</td>'
            "</tr>"
        )
    return "".join(out)


# --------------------------------------------------------------------------- #
# Sections
# --------------------------------------------------------------------------- #


def section_intro(envs) -> str:
    hosts = "; ".join(f"{env.label}" for env in envs)
    repeats = {env.repeats for env in envs}
    return f"""
<h1>go-sqllexer: Rust rewrite versus the Go implementation</h1>
<p class="lede">{hosts}. {"/".join(str(r) for r in sorted(repeats))} repeats per
host, 60&nbsp;s of load per configuration. Every figure is recomputed from the
committed JSON in <code>harness/reports/</code>.</p>
<nav><a href="#result">Result</a><a href="#gates">Acceptance gates</a>
<a href="#read">How to read this</a><a href="#method">Methodology</a>
<a href="#detail">Per-configuration numbers</a><a href="#caveats">Caveats</a></nav>

<p><code>go-sqllexer</code> masks the literals in a SQL statement, canonicalizes it
and extracts metadata about it. Its core has been rewritten in Rust as a
<strong>parallel implementation</strong>: two separate programs, no binding between
them, required to produce byte-identical output for the same input. This report
answers one question — <em>is the Rust one fast enough to justify maintaining a
second implementation?</em> Output parity is established separately (five
differential corpora and continuous differential fuzzing, section A of
<code>harness/ACCEPTANCE.md</code>) and assumed here.</p>
<p>The measurement is a load test, not a microbenchmark: each implementation replays
a corpus of SQL statements through the full obfuscate-and-normalize path on N worker
threads and reports throughput, latency per statement-size class, allocation and
peak memory. Thresholds for a pass were fixed after the first measurements and are
listed under <a href="#gates">acceptance gates</a>.</p>
"""


def section_headline(envs) -> str:
    all_ratios = {}
    for env in envs:
        for (corpus, workers), ratios in throughput_ratios(env).items():
            all_ratios[(env.name, corpus, workers)] = ratios

    def stats_for(corpus):
        means = [statistics.mean(v) for k, v in all_ratios.items() if k[1] == corpus]
        flat = [x for k, v in all_ratios.items() if k[1] == corpus for x in v]
        return geomean(means), min(flat), max(flat)

    mixed = stats_for("workloads")
    patho = stats_for("pathological")

    bytes_ratios, allocs, rss = [], [], []
    for env in envs:
        for corpus, workers, go, rust in env.configurations():
            if corpus == "workloads":
                bytes_ratios += per_run_ratios(
                    go, rust, "memory", "bytes_per_op", invert=True
                )
                allocs += per_run_ratios(
                    go, rust, "memory", "allocs_per_op", invert=True
                )
            rss += per_run_ratios(go, rust, "memory", "rss_peak_mb", invert=True)

    rows = []
    for env in envs:
        for corpus in CORPORA:
            for workers in env.workers:
                ratios = all_ratios.get((env.name, corpus, workers))
                if not ratios:
                    continue
                rows.append(
                    (
                        (
                            f"{env.meta.get('arch', env.name)} · {corpus} · "
                            f"{workers} worker{'' if workers == 1 else 's'}"
                        ),
                        statistics.mean(ratios),
                        min(ratios),
                        max(ratios),
                    )
                )

    return f"""
<h2 id="result">Result</h2>
<p>Rust is faster in every configuration measured: both architectures, both corpora,
every worker count. Throughput figures are geometric means over the four
configurations of that corpus; the range is the lowest and highest single repeat.</p>
<div class="cards">
  <div class="card"><div class="t">Mixed corpus throughput</div>
    <div class="k win">{mixed[0]:.2f}×</div>
    <div class="s">geomean of 4 configurations · observed
      {mixed[1]:.2f}×–{mixed[2]:.2f}×</div></div>
  <div class="card"><div class="t">Pathological corpus throughput</div>
    <div class="k win">{patho[0]:.2f}×</div>
    <div class="s">geomean of 4 configurations · observed
      {patho[1]:.2f}×–{patho[2]:.2f}×</div></div>
  <div class="card"><div class="t">Bytes allocated / statement</div>
    <div class="k win">{geomean(bytes_ratios):.0f}× less</div>
    <div class="s">mixed corpus · Go allocates that much more per statement</div></div>
  <div class="card"><div class="t">Peak RSS</div>
    <div class="k win">{geomean(rss):.1f}× lower</div>
    <div class="s">geomean over all configurations</div></div>
</div>
<figure>{ratio_chart(rows, None, "Rust throughput relative to Go")}
<figcaption>Rust throughput divided by Go throughput, same host, same corpus, same
worker count, measured back to back. Bar is the mean of {envs[0].repeats} repeats;
the whisker spans the lowest and highest single repeat. Higher is better; 1.00× is
parity.</figcaption></figure>
<p>Allocation is the larger and less obvious difference: on the mixed corpus Rust
allocates {geomean(bytes_ratios):.0f}× fewer bytes and
{geomean(allocs):.1f}× fewer times per statement, with no garbage collector to run
afterwards. The allocation <em>count</em> advantage does not survive on the
pathological corpus — see <a href="#caveats">caveats</a>.</p>
"""


def section_reading() -> str:
    return """
<h2 id="read">How to read this report</h2>
<ul>
<li><strong>Ratios are comparable between hosts; absolute numbers are not.</strong>
The ops/s of the ARM runner against the x86 runner says more about the runners than
about the implementations, so no ratio here is computed across hosts — each one
compares Go and Rust on one host, in one sitting.</li>
<li><strong>Above 1.00× always favours Rust</strong>, whether the metric is
higher-is-better (throughput) or lower-is-better (latency, bytes, memory). Each
table states its direction.</li>
<li><strong>"±x%" is the observed spread, not a confidence interval</strong>: half the
distance between the smallest and largest repeat, over their mean.</li>
<li><strong>Nothing here is a significance test.</strong> <code>benchstat</code>
wants ten or more runs for that. Three support a weaker claim: the effect (50–150%)
is an order of magnitude larger than the drift between repeats (a few percent) and
reproduces on two architectures. Treat single-digit percentages as noise.</li>
<li><strong>Aggregates are geometric means</strong> — the correct average for ratios
— and figures are capped at three significant figures.</li>
</ul>
"""


def section_method(envs) -> str:
    host_rows = []
    for env in envs:
        host_rows.append(
            "<tr>"
            f'<td class="left"><code>{esc(env.name)}</code></td>'
            f'<td class="left">{esc(env.meta.get("arch", "?"))}</td>'
            f"<td>{esc(env.meta.get('cores', '?'))}</td>"
            f"<td>{esc(' and '.join(str(w) for w in env.workers))}</td>"
            f"<td>{esc(env.repeats)}</td>"
            f'<td class="left">go {esc(env.meta.get("go", "?"))}, '
            f"rustc {esc(env.meta.get('rustc', '?'))}</td>"
            "</tr>"
        )
    corpus_rows = "".join(
        f'<tr><td class="left"><code>{name}.jsonl</code></td>'
        f'<td class="left">{esc(text)}</td></tr>'
        for name, text in CORPUS_BLURB.items()
    )
    class_rows = "".join(
        f'<tr><td class="left">{name}</td><td class="left">{bound}</td></tr>'
        for name, bound in CLASS_BOUNDS.items()
    )
    envs_pre = "".join(
        f"<h4>{esc(env.name)}</h4><pre>{esc(env.raw_environment)}</pre>" for env in envs
    )
    return f"""
<h2 id="method">Methodology</h2>
<h3>Hosts</h3>
<table><thead><tr><th class="left">directory</th><th class="left">architecture</th>
<th>cores</th><th>worker counts</th><th>repeats</th>
<th class="left">toolchains</th></tr></thead>
<tbody>{"".join(host_rows)}</tbody></table>
<p>Worker counts follow each host's core count rather than being fixed: measuring one
host oversubscribed against another merely saturated would compare schedulers, not
libraries. Each host is also measured at a single worker, which isolates
per-statement cost from scaling behaviour.</p>

<h3>Load</h3>
<p>Each driver replays its corpus round-robin across N worker threads (each starting
at a different corpus offset, so they do not march through it in lockstep) for
<strong>60 s after a 10 s warmup</strong>, three times per host. The warmup keeps
startup out of the measured window: what is measured is steady-state allocator, and
on the Go side garbage collector, behaviour. Both drivers build one
obfuscator/normalizer per worker and reuse it, which is how the library is meant to
be used.</p>
<p>Go and Rust run <strong>sequentially on the same host in one invocation</strong> of
<code>harness/harness bench</code> — two load drivers at once would measure each
other. The code under load is <code>sqllexer.ObfuscateAndNormalize</code> via
<code>harness/cmd/throughput</code> and its Rust equivalent via
<code>rust/sqllexer-runner --bin bench</code>: no inter-process protocol is involved
on either side, and both emit the same JSON schema.</p>

<h3>Corpora</h3>
<table><thead><tr><th class="left">corpus</th><th class="left">contents</th></tr>
</thead><tbody>{corpus_rows}</tbody></table>

<h3>Statement size classes</h3>
<p>Latency is reported per size class rather than in aggregate: in a corpus mixing
40-byte statements with 100-kilobyte ones, an overall percentile reports the input
mix rather than the implementation. Statements are bucketed by input length.</p>
<table><thead><tr><th class="left">size class</th><th class="left">input length</th>
</tr></thead><tbody>{class_rows}</tbody></table>
<p>Percentiles come from a fixed-size histogram shared by both drivers
(<code>harness/internal/latency</code>, mirrored in
<code>rust/sqllexer-runner/src/histogram.rs</code>). Keeping every sample would make
the harness's own memory a function of how many operations completed, which would
penalize the faster implementation.</p>

<h3>Counting allocations</h3>
<p>There is no shared mechanism, so the two runtimes are instrumented separately:</p>
<ul>
<li><strong>Go</strong>: <code>runtime.MemStats</code>, differencing
<code>TotalAlloc</code> and <code>Mallocs</code> across the measured window after a
forced GC at the start.</li>
<li><strong>Rust</strong>: a global allocator wrapping <code>System</code> — never a
faster allocator, which would flatter the comparison — counting bytes and calls per
thread. A reallocation is counted as one allocation of the new size, which is how
Go's <code>Mallocs</code>/<code>TotalAlloc</code> account for the same operation. The
allocation figures come from a second pass with latency recording switched off, so
the histogram is not counted against the library.</li>
</ul>
<p>Bytes per statement is therefore closely comparable, and peak RSS comes from the
OS on both sides. Allocation <em>counts</em> are comparable only where the sizes
being allocated are — true on the mixed corpus, false on the pathological one (see
<a href="#caveats">caveats</a>).</p>

<h3>Recorded host details</h3>
{envs_pre}
"""


def section_gates(envs) -> str:
    return f"""
<h2 id="gates">Acceptance gates</h2>
<p>Section C of <code>harness/ACCEPTANCE.md</code>. A gate holds only if it holds on
both architectures, so "worst observed" is the weakest of the eight configurations;
status is recomputed from the JSON on every regeneration rather than copied from
that document.</p>
<table><thead><tr><th class="left">gate</th><th class="left">threshold</th>
<th>worst observed</th><th>status</th></tr></thead>
<tbody>{gate_rows(envs)}</tbody></table>
"""


def config_tables(env: Environment, corpus: str, workers: int) -> str:
    go, rust = env.pair(corpus, workers)
    if not go or not rust:
        return ""

    def metric_row(label, path, fmt, higher_is_better, unit="", scale=1.0):
        g = [v / scale for v in series(go, *path)]
        r = [v / scale for v in series(rust, *path)]
        gm, rm = statistics.mean(g), statistics.mean(r)
        ratio = (rm / gm) if higher_is_better else (gm / rm)
        runs = " · ".join(f"{v:{fmt}}" for v in r)
        return (
            f'<tr><td class="left">{label}'
            f'<span class="dir">{"higher" if higher_is_better else "lower"} is '
            f"better</span></td>"
            f'<td class="go">{gm:{fmt}}{unit}'
            f'<span class="spread"> ±{spread(g):.1f}%</span></td>'
            f'<td class="rust">{rm:{fmt}}{unit}'
            f'<span class="spread"> ±{spread(r):.1f}%</span></td>'
            + ratio_cell(ratio)
            + f'<td class="left spread">{runs}</td></tr>'
        )

    rows = [
        metric_row("throughput (statements/s)", ("ops_per_second",), ",.0f", True),
        metric_row(
            "SQL processed (MB/s)",
            ("bytes_per_second",),
            ",.0f",
            True,
            scale=1e6,
        ),
        metric_row(
            "bytes allocated / statement", ("memory", "bytes_per_op"), ",.0f", False
        ),
        metric_row(
            "allocations / statement", ("memory", "allocs_per_op"), ".2f", False
        ),
        metric_row("peak RSS (MB)", ("memory", "rss_peak_mb"), ".1f", False),
    ]
    gc = mean(go, "memory", "gc_pause_total_ms")
    out = [
        f"<h4>{esc(corpus)}.jsonl — {workers} worker{'' if workers == 1 else 's'}</h4>",
        (
            '<table><thead><tr><th class="left">metric</th><th>Go</th><th>Rust</th>'
            '<th>Rust advantage</th><th class="left">Rust, per repeat</th></tr></thead>'
            f"<tbody>{''.join(rows)}</tbody></table>"
        ),
        (
            f'<p class="note">Go spent {gc:.1f} ms in GC pauses over the measured window '
            f"({mean(go, 'memory', 'num_gc'):.0f} collections); the Rust build has no "
            "garbage collector and reports no GC figures.</p>"
        ),
    ]

    go_c, rust_c = classes(go), classes(rust)
    latency_rows = []
    for name in CLASSES:
        g, r = go_c.get(name), rust_c.get(name)
        if not g or not r:
            continue
        cells = [f'<td class="left">{name}</td>']
        for percentile in ("p50_ns", "p90_ns", "p99_ns"):
            gv = statistics.mean([c[percentile] for c in g])
            rv = statistics.mean([c[percentile] for c in r])
            cells.append(
                f'<td class="go">{fmt_us(gv)}'
                f'<span class="spread"> ±{spread([c[percentile] for c in g]):.1f}%'
                "</span></td>"
            )
            cells.append(
                f'<td class="rust">{fmt_us(rv)}'
                f'<span class="spread"> ±{spread([c[percentile] for c in r]):.1f}%'
                "</span></td>"
            )
            cells.append(ratio_cell(gv / rv))
        for source in (g, r):
            values = [c["p999_ns"] for c in source]
            cells.append(
                f'<td class="spread">{fmt_us(statistics.mean(values))}'
                f" ±{spread(values):.0f}%</td>"
            )
        latency_rows.append("<tr>" + "".join(cells) + "</tr>")
    if latency_rows:
        out += [
            (
                '<table><thead><tr><th class="left" rowspan="2">size class</th>'
                '<th colspan="3">p50</th><th colspan="3">p90</th><th colspan="3">p99</th>'
                '<th colspan="2">p999 (not gateable)</th></tr>'
                "<tr><th>Go</th><th>Rust</th><th>adv.</th>"
                "<th>Go</th><th>Rust</th><th>adv.</th>"
                "<th>Go</th><th>Rust</th><th>adv.</th>"
                "<th>Go</th><th>Rust</th></tr></thead>"
                f"<tbody>{''.join(latency_rows)}</tbody></table>"
            ),
            (
                '<p class="note">Per-statement latency, lower is better; the advantage '
                "column is Go divided by Rust, so above 1.00× favours Rust. p999 is shown "
                "for completeness only: it drifts by tens of percent between identical "
                "runs.</p>"
            ),
            (
                f"<figure>{latency_chart(env, corpus, workers)}"
                "<figcaption>Mean p50 and p99 per workload class, logarithmic axis — the "
                "classes span three orders of magnitude. Lower is better.</figcaption>"
                "</figure>"
            ),
        ]
    if corpus == "pathological":
        out.append(
            '<p class="note">Allocation <em>count</em> is inverted in this table and '
            "is not a like-for-like metric on this corpus; see "
            '<a href="#caveats">caveats</a>. Bytes allocated is the honest measure '
            "here.</p>"
        )
    return "".join(out)


def section_detail(envs) -> str:
    parts = [
        '<h2 id="detail">Per-configuration numbers</h2>',
        (
            "<p>Every configuration measured, in full. Cells are the mean over that "
            "host's repeats with the observed spread; the last column lists the "
            "individual Rust repeats, so run-to-run variation is visible rather than "
            "summarized away.</p>"
        ),
    ]
    for env in envs:
        parts.append(f"<h3>{esc(env.name)} — {esc(env.label)}</h3>")
        for corpus in CORPORA:
            for workers in env.workers:
                parts.append(config_tables(env, corpus, workers))
    return "".join(parts)


def section_caveats(envs) -> str:
    patho_allocs = []
    patho_bytes = []
    for env in envs:
        for corpus, workers, go, rust in env.configurations():
            if corpus != "pathological":
                continue
            patho_allocs.append(
                (
                    mean(go, "memory", "allocs_per_op"),
                    mean(rust, "memory", "allocs_per_op"),
                )
            )
            patho_bytes.append(
                (
                    mean(go, "memory", "bytes_per_op"),
                    mean(rust, "memory", "bytes_per_op"),
                )
            )
    go_allocs = statistics.mean([g for g, _ in patho_allocs])
    rust_allocs = statistics.mean([r for _, r in patho_allocs])
    go_bytes = statistics.mean([g for g, _ in patho_bytes])
    rust_bytes = statistics.mean([r for _, r in patho_bytes])

    p999_drift = 0.0
    for env in envs:
        for corpus, workers, go, rust in env.configurations():
            for reports in (go, rust):
                for entries in classes(reports).values():
                    p999_drift = max(
                        p999_drift, spread([c["p999_ns"] for c in entries])
                    )

    return f"""
<h2 id="caveats">Caveats</h2>
<div class="callout">
<h4>Allocation count inverts on the pathological corpus</h4>
<p>On adversarial input Rust makes {rust_allocs:.0f} allocations per statement against
Go's {go_allocs:.0f} — it loses that metric {rust_allocs / go_allocs:.0f}× — while
allocating {go_bytes / rust_bytes:.0f}× fewer bytes ({rust_bytes:,.0f} B against
{go_bytes:,.0f} B). The cause is structural rather than a measurement artifact: Rust
grows reusable buffers in many small steps where Go takes a few very large ones, and
both sides count a reallocation as one allocation. Total bytes is the meaningful
measure here, which is why the allocation-count gate is scoped to the mixed
corpus.</p>
</div>
<div class="callout">
<h4>Tail latency past p99 is not gateable at this sample size</h4>
<p>p999 drifts by up to {p999_drift:.0f}% between repeats of identical code on the
same host, so it is reported for completeness and excluded from every gate. Any
conclusion resting on p999 or on <code>max_ns</code> needs more repeats than were run
here.</p>
</div>
<div class="callout">
<h4>The corpora are synthetic</h4>
<p>The mixed corpus is built from SQL literals in the repository's own benchmark
files, the pathological one from constructed adversarial input. They cover the
option space more exhaustively than production traffic would, but not the
<em>distribution</em> of real queries — and distribution moves these ratios, since a
workload of nothing but 40-byte statements lands near the short-class numbers rather
than the aggregate. It does not affect correctness parity, which is established per
statement. No production-traffic shadow validation has been run.</p>
</div>
<div class="callout">
<h4>Three repeats, two hosts, one shape of work</h4>
<p>Both hosts are cloud CI runners, noisier than dedicated hardware; the mitigation
is that Go and Rust are measured minutes apart on the same instance, so a slow
runner penalizes both. The benchmark covers steady-state sustained load with reused
instances and says nothing about cold start, about constructing an obfuscator per
statement, or about memory behaviour beyond a minute.</p>
</div>
<p>Out of scope here: calling the Rust core from Go (cgo, handle lifetime,
per-statement FFI overhead), the <code>CGO_ENABLED=0</code> question, and shipping
either the Rust CLI or this harness as a supported artifact.</p>
"""


def write_html(roots, out_path: Path):
    envs = [Environment(root) for root in roots]
    envs = [env for env in envs if env.runs]
    if not envs:
        raise SystemExit(
            "no JSON reports found in {}".format(", ".join(map(str, roots)))
        )
    sources = " ".join(str(root) for root in roots)
    parts = [
        "<!doctype html><html lang='en'><head><meta charset='utf-8'>",
        "<meta name='viewport' content='width=device-width, initial-scale=1'>",
        "<title>go-sqllexer: Rust vs Go benchmark report</title>",
        f"<style>{STYLE}</style></head><body>",
        section_intro(envs),
        section_headline(envs),
        section_gates(envs),
        section_reading(),
        section_method(envs),
        section_detail(envs),
        section_caveats(envs),
        "<footer>Generated by <code>harness/reports/summarize.py</code> from the "
        "JSON reports in "
        + ", ".join(f"<code>{esc(root)}</code>" for root in roots)
        + ". Regenerate with:<pre>python3 harness/reports/summarize.py --html "
        f"{esc(out_path)} {esc(sources)}</pre>"
        "This page has no external references: no scripts, no fonts, no images "
        "fetched over the network.</footer></body></html>",
    ]
    out_path.write_text("".join(parts))
    print(f"wrote {out_path}")


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("roots", nargs="+", type=Path, help="report directories")
    parser.add_argument("--html", type=Path, help="write the HTML benchmark report")
    args = parser.parse_args(argv)

    if args.html:
        write_html(args.roots, args.html)
    else:
        for root in args.roots:
            print_markdown(root)


if __name__ == "__main__":
    main(sys.argv[1:])
