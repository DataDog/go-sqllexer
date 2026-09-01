//! Rust side of the differential harness.
//!
//! Reads harness protocol requests as newline-delimited JSON on stdin and writes
//! one response per line, interchangeably with `harness/cmd/gorunner`. Handles are
//! cached per configuration so the measured path is the one production would use
//! (a reusable obfuscator/normalizer pair rather than one built per statement).
//!
//!     cargo run --release -p sqllexer-runner < corpus/testdata.jsonl > rust.out.jsonl

mod text;

use std::collections::HashMap;
use std::io::{self, BufWriter, Read, Write};

use serde::{Deserialize, Serialize};
use sqllexer::{
    Dbms, Lexer, Normalizer, NormalizerConfig, Obfuscator, ObfuscatorConfig, StatementMetadata,
    TokenType,
};

use crate::text::Text;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Deserialize, Default)]
#[serde(default)]
struct WireObfuscatorConfig {
    dollar_quoted_func: bool,
    replace_digits: bool,
    replace_positional_parameter: bool,
    replace_boolean: bool,
    replace_null: bool,
    keep_json_path: bool,
    replace_bind_parameter: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Deserialize, Default)]
#[serde(default)]
struct WireNormalizerConfig {
    collect_tables: bool,
    collect_commands: bool,
    collect_comments: bool,
    collect_procedure: bool,
    keep_sql_alias: bool,
    uppercase_keywords: bool,
    remove_space_between_parentheses: bool,
    keep_trailing_semicolon: bool,
    keep_identifier_quotation: bool,
}

impl From<WireObfuscatorConfig> for ObfuscatorConfig {
    fn from(c: WireObfuscatorConfig) -> Self {
        ObfuscatorConfig {
            dollar_quoted_func: c.dollar_quoted_func,
            replace_digits: c.replace_digits,
            replace_positional_parameter: c.replace_positional_parameter,
            replace_boolean: c.replace_boolean,
            replace_null: c.replace_null,
            keep_json_path: c.keep_json_path,
            replace_bind_parameter: c.replace_bind_parameter,
        }
    }
}

impl From<WireNormalizerConfig> for NormalizerConfig {
    fn from(c: WireNormalizerConfig) -> Self {
        NormalizerConfig {
            collect_tables: c.collect_tables,
            collect_commands: c.collect_commands,
            collect_comments: c.collect_comments,
            collect_procedure: c.collect_procedure,
            keep_sql_alias: c.keep_sql_alias,
            uppercase_keywords: c.uppercase_keywords,
            remove_space_between_parentheses: c.remove_space_between_parentheses,
            keep_trailing_semicolon: c.keep_trailing_semicolon,
            keep_identifier_quotation: c.keep_identifier_quotation,
        }
    }
}

/// Defaults applied when a request omits a configuration, matching
/// `protocol.DefaultObfuscatorConfig` and `protocol.DefaultNormalizerConfig`.
fn default_obfuscator() -> WireObfuscatorConfig {
    WireObfuscatorConfig {
        replace_digits: true,
        replace_boolean: true,
        replace_null: true,
        ..Default::default()
    }
}

fn default_normalizer() -> WireNormalizerConfig {
    WireNormalizerConfig {
        collect_tables: true,
        collect_commands: true,
        collect_comments: true,
        ..Default::default()
    }
}

#[derive(Debug, Deserialize)]
struct Request {
    #[serde(default)]
    id: String,
    #[serde(default)]
    sql: Text,
    #[serde(default)]
    dbms: String,
    mode: String,
    #[serde(default)]
    obfuscator: Option<WireObfuscatorConfig>,
    #[serde(default)]
    normalizer: Option<WireNormalizerConfig>,
}

#[derive(Debug, Serialize)]
struct WireMetadata {
    size: usize,
    tables: Vec<Text>,
    comments: Vec<Text>,
    commands: Vec<Text>,
    procedures: Vec<Text>,
}

#[derive(Debug, Serialize)]
struct WireToken {
    #[serde(rename = "type")]
    token_type: u8,
    value: Text,
}

#[derive(Debug, Serialize, Default)]
struct Response {
    id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    output: Option<Text>,
    #[serde(skip_serializing_if = "Option::is_none")]
    metadata: Option<WireMetadata>,
    #[serde(skip_serializing_if = "Option::is_none")]
    tokens: Option<Vec<WireToken>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

/// Reusable per-configuration state, mirroring how a tracer would hold handles.
#[derive(Default)]
struct Handles {
    obfuscators: HashMap<WireObfuscatorConfig, Obfuscator>,
    normalizers: HashMap<WireNormalizerConfig, Normalizer>,
}

impl Handles {
    /// Borrows both handles at once; they live in separate maps so no aliasing
    /// workaround is needed.
    fn pair(
        &mut self,
        obfuscator: WireObfuscatorConfig,
        normalizer: WireNormalizerConfig,
    ) -> (&Obfuscator, &Normalizer) {
        (
            self.obfuscators
                .entry(obfuscator)
                .or_insert_with(|| Obfuscator::new(obfuscator.into())),
            self.normalizers
                .entry(normalizer)
                .or_insert_with(|| Normalizer::new(normalizer.into())),
        )
    }

    fn obfuscator(&mut self, cfg: WireObfuscatorConfig) -> &Obfuscator {
        self.obfuscators
            .entry(cfg)
            .or_insert_with(|| Obfuscator::new(cfg.into()))
    }

    fn normalizer(&mut self, cfg: WireNormalizerConfig) -> &Normalizer {
        self.normalizers
            .entry(cfg)
            .or_insert_with(|| Normalizer::new(cfg.into()))
    }
}

fn main() {
    let mut input = Vec::new();
    if let Err(err) = io::stdin().read_to_end(&mut input) {
        eprintln!("read failed: {err}");
        std::process::exit(1);
    }
    let stdout = io::stdout();
    let mut out = BufWriter::with_capacity(1 << 20, stdout.lock());
    let mut handles = Handles::default();
    let mut sql = Vec::new();
    let mut metadata = StatementMetadata::default();

    for line in input.split(|&b| b == b'\n') {
        if line.is_empty() {
            continue;
        }
        let req: Request = match serde_json::from_slice(line) {
            Ok(req) => req,
            Err(err) => {
                eprintln!("malformed request: {err}");
                std::process::exit(1);
            }
        };
        let resp = handle(req, &mut handles, &mut sql, &mut metadata);
        if let Err(err) = serde_json::to_writer(&mut out, &resp).and_then(|()| {
            out.write_all(b"\n").map_err(serde_json::Error::io)?;
            Ok(())
        }) {
            eprintln!("write failed: {err}");
            std::process::exit(1);
        }
    }
    if let Err(err) = out.flush() {
        eprintln!("flush failed: {err}");
        std::process::exit(1);
    }
}

fn handle(
    req: Request,
    handles: &mut Handles,
    sql: &mut Vec<u8>,
    metadata: &mut StatementMetadata,
) -> Response {
    let dbms = Dbms::from_name(&req.dbms);
    let mut resp = Response {
        id: req.id,
        ..Default::default()
    };

    match req.mode.as_str() {
        "tokenize" => {
            let mut lexer = Lexer::new(req.sql.as_bytes(), dbms);
            let mut tokens = Vec::new();
            loop {
                let token = lexer.scan();
                if token.token_type == TokenType::Eof {
                    break;
                }
                tokens.push(WireToken {
                    token_type: token.token_type as u8,
                    value: Text(token.value.into_owned()),
                });
            }
            // An empty token list is reported as absent, like the Go runner's nil slice.
            resp.tokens = (!tokens.is_empty()).then_some(tokens);
        }
        "obfuscate" => {
            let cfg = req.obfuscator.unwrap_or_else(default_obfuscator);
            let output = handles.obfuscator(cfg).obfuscate(req.sql.as_bytes(), dbms);
            resp.output = Some(Text(output));
        }
        "normalize" => {
            let cfg = req.normalizer.unwrap_or_else(default_normalizer);
            handles
                .normalizer(cfg)
                .normalize_into(req.sql.as_bytes(), dbms, sql, metadata);
            resp.output = Some(Text(sql.clone()));
            resp.metadata = Some(wire_metadata(metadata));
        }
        "obfuscate_and_normalize" => {
            let (obfuscator, normalizer) = handles.pair(
                req.obfuscator.unwrap_or_else(default_obfuscator),
                req.normalizer.unwrap_or_else(default_normalizer),
            );
            normalizer.obfuscate_and_normalize_into(
                req.sql.as_bytes(),
                obfuscator,
                dbms,
                sql,
                metadata,
            );
            resp.output = Some(Text(sql.clone()));
            resp.metadata = Some(wire_metadata(metadata));
        }
        other => {
            resp.error = Some(format!("unknown mode \"{other}\""));
        }
    }
    resp
}

fn wire_metadata(metadata: &StatementMetadata) -> WireMetadata {
    let texts = |list: &sqllexer::MetadataList| -> Vec<Text> {
        list.values().iter().map(|v| Text(v.clone())).collect()
    };
    WireMetadata {
        size: metadata.size,
        tables: texts(&metadata.tables),
        comments: texts(&metadata.comments),
        commands: texts(&metadata.commands),
        procedures: texts(&metadata.procedures),
    }
}
