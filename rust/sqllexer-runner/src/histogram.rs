//! Fixed-size latency histogram, mirroring `harness/internal/latency` so the Rust
//! and Go load drivers report percentiles computed the same way — and so neither
//! driver's own memory grows with the number of operations it completed.

const SUB_BUCKET_BITS: u32 = 10;
const SUB_BUCKET_COUNT: u64 = 1 << SUB_BUCKET_BITS;
const BUCKET_COUNT: usize = (SUB_BUCKET_COUNT as usize) * 54;

pub struct Histogram {
    counts: Vec<u64>,
    total: u64,
    max: u64,
}

impl Default for Histogram {
    fn default() -> Self {
        Histogram {
            counts: vec![0; BUCKET_COUNT],
            total: 0,
            max: 0,
        }
    }
}

fn index(ns: u64) -> usize {
    if ns < SUB_BUCKET_COUNT {
        return ns as usize;
    }
    let magnitude = 63 - ns.leading_zeros();
    let shift = magnitude - SUB_BUCKET_BITS;
    ((shift as usize + 1) * SUB_BUCKET_COUNT as usize) + ((ns >> shift) - SUB_BUCKET_COUNT) as usize
}

fn value(i: usize) -> u64 {
    if (i as u64) < SUB_BUCKET_COUNT {
        return i as u64;
    }
    let shift = i / SUB_BUCKET_COUNT as usize - 1;
    // The bucket's upper bound, so percentiles never read low.
    ((i as u64 % SUB_BUCKET_COUNT) + SUB_BUCKET_COUNT + 1) << shift
}

impl Histogram {
    pub fn add(&mut self, ns: u64) {
        let i = index(ns).min(BUCKET_COUNT - 1);
        self.counts[i] += 1;
        self.total += 1;
        self.max = self.max.max(ns);
    }

    pub fn merge(&mut self, other: &Histogram) {
        for (dst, src) in self.counts.iter_mut().zip(&other.counts) {
            *dst += src;
        }
        self.total += other.total;
        self.max = self.max.max(other.max);
    }

    pub fn count(&self) -> u64 {
        self.total
    }

    pub fn max(&self) -> u64 {
        self.max
    }

    pub fn mean(&self) -> f64 {
        if self.total == 0 {
            return 0.0;
        }
        let sum: f64 = self
            .counts
            .iter()
            .enumerate()
            .filter(|(_, c)| **c != 0)
            .map(|(i, c)| value(i) as f64 * *c as f64)
            .sum();
        sum / self.total as f64
    }

    pub fn quantile(&self, q: f64) -> u64 {
        if self.total == 0 {
            return 0;
        }
        let target = ((self.total - 1) as f64 * q) as u64;
        let mut seen = 0u64;
        for (i, c) in self.counts.iter().enumerate() {
            seen += c;
            if seen > target {
                return value(i).min(self.max);
            }
        }
        self.max
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn quantiles_are_within_bucket_error() {
        let mut h = Histogram::default();
        let mut samples: Vec<u64> = (1..=100_000).map(|i| i * 7 % 50_000).collect();
        for s in &samples {
            h.add(*s);
        }
        samples.sort_unstable();
        for q in [0.5, 0.9, 0.99, 0.999] {
            let want = samples[((samples.len() - 1) as f64 * q) as usize];
            let got = h.quantile(q);
            assert!(
                got >= want && (got - want) as f64 <= want as f64 / 1024.0 + 1.0,
                "q{q}: got {got}, want {want}"
            );
        }
        assert_eq!(h.count(), samples.len() as u64);
        assert_eq!(h.max(), *samples.last().unwrap());
    }

    #[test]
    fn merge_is_additive() {
        let (mut a, mut b, mut both) = (
            Histogram::default(),
            Histogram::default(),
            Histogram::default(),
        );
        for i in 1..=1000 {
            a.add(i);
            both.add(i);
        }
        for i in 5000..=6000 {
            b.add(i);
            both.add(i);
        }
        a.merge(&b);
        assert_eq!(a.count(), both.count());
        assert_eq!(a.max(), both.max());
        for q in [0.1, 0.5, 0.99] {
            assert_eq!(a.quantile(q), both.quantile(q));
        }
    }

    #[test]
    fn empty_histogram_reports_zeros() {
        let h = Histogram::default();
        assert_eq!(h.quantile(0.5), 0);
        assert_eq!(h.mean(), 0.0);
        assert_eq!(h.count(), 0);
    }
}
