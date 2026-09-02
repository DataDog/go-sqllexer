//! A Rust implementation of [go-sqllexer](https://github.com/DataDog/go-sqllexer).
//!
//! The crate is a behavioral port, not a reinterpretation: it reproduces the Go
//! package's output byte for byte, including its handling of malformed input, so
//! the two can be compared directly by the differential harness in `harness/`.
//! Where the Go code has observable quirks (invalid UTF-8 advancing the cursor by
//! three bytes, `strings.TrimSpace` trimming Unicode spaces, metadata sizes
//! counting only first occurrences) the quirk is reproduced and commented rather
//! than corrected.
//!
//! Input and output are byte slices rather than `str` because the Go API accepts
//! arbitrary bytes and its behavior on invalid UTF-8 is part of the contract.
//!
//! ```
//! use sqllexer::{Dbms, Normalizer, NormalizerConfig, Obfuscator, ObfuscatorConfig};
//!
//! let obfuscator = Obfuscator::new(ObfuscatorConfig::default());
//! let normalizer = Normalizer::new(NormalizerConfig {
//!     collect_tables: true,
//!     collect_commands: true,
//!     ..Default::default()
//! });
//!
//! let mut sql = Vec::new();
//! let mut metadata = Default::default();
//! normalizer.obfuscate_and_normalize_into(
//!     b"SELECT * FROM users WHERE id = 42",
//!     &obfuscator,
//!     Dbms::None,
//!     &mut sql,
//!     &mut metadata,
//! );
//! assert_eq!(sql, b"SELECT * FROM users WHERE id = ?");
//! assert_eq!(metadata.tables.values(), [b"users".to_vec()]);
//! ```

mod chars;
mod keywords;
mod lexer;
mod normalizer;
mod obfuscator;
mod token;
mod unicode_tables;

pub use lexer::{Dbms, Lexer};
pub use normalizer::{MetadataList, Normalizer, NormalizerConfig, StatementMetadata};
pub use obfuscator::{Obfuscator, ObfuscatorConfig, NUMBER_PLACEHOLDER, STRING_PLACEHOLDER};
pub use token::{Token, TokenType};

/// A reusable obfuscate-and-normalize pipeline: one per worker.
///
/// Configuration, buffers and metadata storage are set up once instead of per
/// statement, and `process` reuses the same buffers across calls, which is where
/// most of the throughput difference against a construct-per-call pattern comes
/// from. Not `Sync`: `process` takes `&mut self`.
pub struct Processor {
    obfuscator: Obfuscator,
    normalizer: Normalizer,
    sql: Vec<u8>,
    metadata: StatementMetadata,
}

impl Processor {
    pub fn new(obfuscator: ObfuscatorConfig, normalizer: NormalizerConfig) -> Self {
        Processor {
            obfuscator: Obfuscator::new(obfuscator),
            normalizer: Normalizer::new(normalizer),
            sql: Vec::new(),
            metadata: StatementMetadata::default(),
        }
    }

    /// Obfuscates and normalizes one statement, returning borrowed views of the
    /// reusable buffers. Both are invalidated by the next call.
    pub fn process(&mut self, input: &[u8], dbms: Dbms) -> (&[u8], &StatementMetadata) {
        self.normalizer.obfuscate_and_normalize_into(
            input,
            &self.obfuscator,
            dbms,
            &mut self.sql,
            &mut self.metadata,
        );
        (&self.sql, &self.metadata)
    }
}
