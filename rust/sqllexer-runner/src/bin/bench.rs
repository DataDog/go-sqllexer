//! Load driver for the Rust core.
//!
//! It is the counterpart of `harness/cmd/throughput`: same corpora, same worker
//! model, same workload classes, same histogram, same report fields, so the two
//! implementations are comparable line by line.
//!
//!     cargo build --release
//!     ./target/release/bench --corpus ../harness/corpus/workloads.jsonl \
//!       --workers 8 --duration 60 --warmup 10 --json /tmp/rust.json
//!
//! Allocation counting comes from a global allocator wrapping the system
//! allocator — never a faster one, which would flatter the comparison — and is
//! measured in a second pass with latency recording switched off, so the
//! recording path is not part of the counts.

use std::alloc::{GlobalAlloc, Layout, System};
use std::cell::Cell;
use std::fs::File;
use std::io::{self, BufWriter, Write};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};

use serde::Serialize;
use sqllexer::{NormalizerConfig, ObfuscatorConfig, Processor};
use sqllexer_runner::corpus::{self, Entry};
use sqllexer_runner::histogram::Histogram;

// Allocation accounting. Threads count into a thread-local cell and flush into
// the globals when a pass ends: a shared atomic per allocation would be measuring
// the counter rather than the library.
thread_local! {
    static LOCAL: Cell<(u64, u64)> = const { Cell::new((0, 0)) };
}

static ALLOC_BYTES: AtomicU64 = AtomicU64::new(0);
static ALLOC_COUNT: AtomicU64 = AtomicU64::new(0);

fn record(bytes: usize) {
    // try_with: an allocation can outlive its thread's TLS during teardown.
    let _ = LOCAL.try_with(|cell| {
        let (b, n) = cell.get();
        cell.set((b + bytes as u64, n + 1));
    });
}

fn flush_local() {
    LOCAL.with(|cell| {
        let (b, n) = cell.take();
        ALLOC_BYTES.fetch_add(b, Ordering::Relaxed);
        ALLOC_COUNT.fetch_add(n, Ordering::Relaxed);
    });
}

struct Counting;

// A growth is counted as one allocation of the new size, which is how Go's
// Mallocs/TotalAlloc account for the same operation.
unsafe impl GlobalAlloc for Counting {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        record(layout.size());
        System.alloc(layout)
    }

    unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
        record(layout.size());
        System.alloc_zeroed(layout)
    }

    unsafe fn realloc(&self, ptr: *mut u8, layout: Layout, new_size: usize) -> *mut u8 {
        record(new_size);
        System.realloc(ptr, layout, new_size)
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        System.dealloc(ptr, layout)
    }
}

#[global_allocator]
static ALLOCATOR: Counting = Counting;

/// Workload classes, identical to the Go driver's.
const CLASSES: [(&str, usize); 4] = [
    ("short", 256),
    ("medium", 2048),
    ("large", 16384),
    ("pathological", usize::MAX),
];

fn classify(size: usize) -> usize {
    CLASSES
        .iter()
        .position(|(_, max)| size <= *max)
        .unwrap_or(CLASSES.len() - 1)
}

#[derive(Default)]
struct ClassStats {
    histogram: Histogram,
    bytes: u64,
    ops: u64,
}

impl ClassStats {
    fn merge(&mut self, other: &ClassStats) {
        self.histogram.merge(&other.histogram);
        self.bytes += other.bytes;
        self.ops += other.ops;
    }
}

#[derive(Serialize)]
struct ClassReport {
    class: &'static str,
    ops: u64,
    ops_per_second: f64,
    mean_ns: f64,
    p50_ns: u64,
    p90_ns: u64,
    p99_ns: u64,
    p999_ns: u64,
    max_ns: u64,
}

#[derive(Serialize)]
struct MemoryReport {
    bytes_allocated_total: u64,
    bytes_per_op: f64,
    allocs_total: u64,
    allocs_per_op: f64,
    alloc_rate_mb_per_second: f64,
    // Kept so the report is field-compatible with the Go driver's. There is no
    // garbage collector on this side; the zeros are the point.
    rss_peak_mb: f64,
    rss_final_mb: f64,
}

#[derive(Serialize)]
struct Report {
    implementation: String,
    corpus: String,
    workers: usize,
    duration: String,
    total_ops: u64,
    ops_per_second: f64,
    bytes_per_second: f64,
    classes: Vec<ClassReport>,
    memory: MemoryReport,
}

struct Args {
    corpus: String,
    workers: usize,
    duration: u64,
    warmup: u64,
    json: Option<String>,
}

/// The allocation pass only needs enough operations for a stable per-op average,
/// not the full measured duration.
const ALLOC_PASS: Duration = Duration::from_secs(5);

fn parse_args() -> Args {
    let mut args = Args {
        corpus: "harness/corpus/workloads.jsonl".to_string(),
        workers: thread::available_parallelism()
            .map(|n| n.get())
            .unwrap_or(1),
        duration: 30,
        warmup: 3,
        json: None,
    };
    let mut argv = std::env::args().skip(1);
    while let Some(flag) = argv.next() {
        let mut value = || {
            argv.next()
                .unwrap_or_else(|| panic!("{flag} needs a value"))
        };
        match flag.as_str() {
            "--corpus" => args.corpus = value(),
            "--workers" => args.workers = value().parse().expect("--workers"),
            "--duration" => args.duration = value().parse().expect("--duration in seconds"),
            "--warmup" => args.warmup = value().parse().expect("--warmup in seconds"),
            "--json" => args.json = Some(value()),
            other => panic!("unknown flag {other}"),
        }
    }
    args
}

fn processor() -> Processor {
    Processor::new(
        ObfuscatorConfig {
            replace_digits: true,
            replace_boolean: true,
            replace_null: true,
            ..Default::default()
        },
        NormalizerConfig {
            collect_tables: true,
            collect_commands: true,
            collect_comments: true,
            ..Default::default()
        },
    )
}

/// Drives the corpus round-robin across workers until the deadline. With
/// `record` off nothing but the operation itself runs in the loop, which is the
/// pass the allocation numbers come from.
fn run_load(
    entries: &[Entry],
    workers: usize,
    duration: Duration,
    record: bool,
) -> (u64, Vec<ClassStats>) {
    let deadline = Instant::now() + duration;
    let ops = AtomicU64::new(0);

    let merged = thread::scope(|scope| {
        let handles: Vec<_> = (0..workers)
            .map(|worker| {
                let ops = &ops;
                scope.spawn(move || {
                    let mut proc = processor();
                    let mut stats: Vec<ClassStats> =
                        (0..CLASSES.len()).map(|_| ClassStats::default()).collect();
                    let mut local_ops = 0u64;

                    // Workers start at different offsets so they do not march
                    // through the corpus in lockstep.
                    let mut i = worker * entries.len() / workers;
                    // The allocations made setting the worker up (the handle and
                    // its histograms) are not part of the pass.
                    LOCAL.with(|cell| cell.set((0, 0)));
                    loop {
                        if Instant::now() > deadline {
                            break;
                        }
                        // Same 64-iteration clock check as the Go driver.
                        for _ in 0..64 {
                            let entry = &entries[i % entries.len()];
                            i += 1;

                            if record {
                                let started = Instant::now();
                                let (sql, metadata) = proc.process(&entry.sql, entry.dbms);
                                let produced = sql.len() + metadata.size;
                                let elapsed = started.elapsed();
                                std::hint::black_box(produced);
                                let class = &mut stats[classify(entry.sql.len())];
                                class.histogram.add(elapsed.as_nanos() as u64);
                                class.bytes += entry.sql.len() as u64;
                                class.ops += 1;
                                local_ops += 1;
                            } else {
                                let (sql, metadata) = proc.process(&entry.sql, entry.dbms);
                                std::hint::black_box(sql.len() + metadata.size);
                                local_ops += 1;
                            }
                        }
                    }
                    ops.fetch_add(local_ops, Ordering::Relaxed);
                    flush_local();
                    stats
                })
            })
            .collect();

        let mut merged: Vec<ClassStats> =
            (0..CLASSES.len()).map(|_| ClassStats::default()).collect();
        for handle in handles {
            let stats = handle.join().expect("worker panicked");
            for (dst, src) in merged.iter_mut().zip(stats.iter()) {
                dst.merge(src);
            }
        }
        merged
    });

    (ops.load(Ordering::Relaxed), merged)
}

/// Resident set size in bytes, from /proc/self/statm. Absent elsewhere, in which
/// case the RSS fields are reported as zero rather than failing the run.
fn read_rss() -> Option<u64> {
    let raw = std::fs::read_to_string("/proc/self/statm").ok()?;
    let pages: u64 = raw.split_whitespace().nth(1)?.parse().ok()?;
    Some(pages * page_size())
}

extern "C" {
    fn sysconf(name: i32) -> i64;
}

/// The kernel page size. Not always 4KB — aarch64 kernels are commonly built
/// with 16KB or 64KB pages, which would make a hardcoded constant misreport RSS
/// by 4x or 16x on the ARM runner.
fn page_size() -> u64 {
    // _SC_PAGESIZE is 30 on Linux, on both glibc and musl.
    let value = unsafe { sysconf(30) };
    if value <= 0 {
        4096
    } else {
        value as u64
    }
}

struct RssSampler {
    stop: Arc<AtomicBool>,
    handle: thread::JoinHandle<(u64, u64)>,
}

impl RssSampler {
    fn start() -> RssSampler {
        let stop = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            let (mut peak, mut last) = (0u64, 0u64);
            loop {
                if let Some(rss) = read_rss() {
                    last = rss;
                    peak = peak.max(rss);
                }
                if flag.load(Ordering::Relaxed) {
                    return (peak, last);
                }
                thread::sleep(Duration::from_millis(100));
            }
        });
        RssSampler { stop, handle }
    }

    fn stop(self) -> (u64, u64) {
        self.stop.store(true, Ordering::Relaxed);
        self.handle.join().unwrap_or((0, 0))
    }
}

fn main() -> io::Result<()> {
    let args = parse_args();
    let entries = corpus::read(&args.corpus)?;
    if entries.is_empty() {
        panic!("corpus {} is empty", args.corpus);
    }

    // Warmup: let the allocator reach steady state before anything is recorded.
    run_load(
        &entries,
        args.workers,
        Duration::from_secs(args.warmup),
        false,
    );

    let sampler = RssSampler::start();
    let start = Instant::now();
    let (total_ops, stats) = run_load(
        &entries,
        args.workers,
        Duration::from_secs(args.duration),
        true,
    );
    let elapsed = start.elapsed();
    let (rss_peak, rss_final) = sampler.stop();

    // Second pass, recording off: the allocation numbers must not include the
    // histogram and clock work the measured pass does per operation.
    ALLOC_BYTES.store(0, Ordering::Relaxed);
    ALLOC_COUNT.store(0, Ordering::Relaxed);
    let alloc_start = Instant::now();
    let (alloc_ops, _) = run_load(&entries, args.workers, ALLOC_PASS, false);
    let alloc_elapsed = alloc_start.elapsed();
    let alloc_bytes = ALLOC_BYTES.load(Ordering::Relaxed);
    let alloc_count = ALLOC_COUNT.load(Ordering::Relaxed);

    let seconds = elapsed.as_secs_f64();
    let mut classes = Vec::new();
    let mut total_bytes = 0u64;
    for (i, (name, _)) in CLASSES.iter().enumerate() {
        let s = &stats[i];
        if s.ops == 0 {
            continue;
        }
        total_bytes += s.bytes;
        classes.push(ClassReport {
            class: name,
            ops: s.ops,
            ops_per_second: s.ops as f64 / seconds,
            mean_ns: s.histogram.mean(),
            p50_ns: s.histogram.quantile(0.50),
            p90_ns: s.histogram.quantile(0.90),
            p99_ns: s.histogram.quantile(0.99),
            p999_ns: s.histogram.quantile(0.999),
            max_ns: s.histogram.max(),
        });
    }

    let report = Report {
        implementation: "rust".to_string(),
        corpus: args.corpus.clone(),
        workers: args.workers,
        duration: format!("{}s", args.duration),
        total_ops,
        ops_per_second: total_ops as f64 / seconds,
        bytes_per_second: total_bytes as f64 / seconds,
        classes,
        memory: MemoryReport {
            bytes_allocated_total: alloc_bytes,
            bytes_per_op: div(alloc_bytes as f64, alloc_ops as f64),
            allocs_total: alloc_count,
            allocs_per_op: div(alloc_count as f64, alloc_ops as f64),
            alloc_rate_mb_per_second: alloc_bytes as f64
                / (1 << 20) as f64
                / alloc_elapsed.as_secs_f64(),
            rss_peak_mb: rss_peak as f64 / (1 << 20) as f64,
            rss_final_mb: rss_final as f64 / (1 << 20) as f64,
        },
    };

    print_report(&report);
    if let Some(path) = &args.json {
        let file = File::create(path)?;
        serde_json::to_writer_pretty(BufWriter::new(file), &report)?;
        println!("\njson report: {path}");
    }
    Ok(())
}

fn div(a: f64, b: f64) -> f64 {
    if b == 0.0 {
        0.0
    } else {
        a / b
    }
}

fn print_report(r: &Report) {
    let out = io::stdout();
    let mut out = out.lock();
    let _ = writeln!(out, "implementation  {}", r.implementation);
    let _ = writeln!(out, "corpus          {}", r.corpus);
    let _ = writeln!(out, "workers         {}", r.workers);
    let _ = writeln!(
        out,
        "throughput      {:.0} ops/s  ({:.1} MB/s of SQL)",
        r.ops_per_second,
        r.bytes_per_second / (1 << 20) as f64
    );
    let _ = writeln!(out, "total ops       {}\n", r.total_ops);
    let _ = writeln!(
        out,
        "{:<18} {:>10} {:>12} {:>10} {:>10} {:>10} {:>10}",
        "class", "ops", "ops/s", "p50", "p90", "p99", "p99.9"
    );
    for c in &r.classes {
        let _ = writeln!(
            out,
            "{:<18} {:>10} {:>12.0} {:>10} {:>10} {:>10} {:>10}",
            c.class,
            c.ops,
            c.ops_per_second,
            micros(c.p50_ns),
            micros(c.p90_ns),
            micros(c.p99_ns),
            micros(c.p999_ns)
        );
    }
    let m = &r.memory;
    let _ = writeln!(
        out,
        "\nallocation      {:.0} B/op, {:.1} allocs/op, {:.1} MB/s allocated",
        m.bytes_per_op, m.allocs_per_op, m.alloc_rate_mb_per_second
    );
    let _ = writeln!(
        out,
        "memory          no GC, RSS peak {:.1} MB, RSS final {:.1} MB",
        m.rss_peak_mb, m.rss_final_mb
    );
}

fn micros(ns: u64) -> String {
    format!("{:.3}µs", ns as f64 / 1000.0)
}
