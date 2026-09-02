//! Behavioral parity checks against the Go implementation.
//!
//! Every expectation here was produced by running the same input through
//! `harness/cmd/gorunner` (the Go package itself), so a failure means the Rust port
//! diverged, not that a hand-written expectation is wrong. This file is a fast
//! smoke layer over the interesting corners; the exhaustive evidence comes from
//! `harness/cmd/differ` over the generated corpora.

use sqllexer::{
    Dbms, Lexer, Normalizer, NormalizerConfig, Obfuscator, ObfuscatorConfig, StatementMetadata,
    TokenType,
};

fn tokens(sql: &str, dbms: Dbms) -> Vec<(u8, String)> {
    let mut lexer = Lexer::new(sql.as_bytes(), dbms);
    let mut out = Vec::new();
    loop {
        let token = lexer.scan();
        if token.token_type == TokenType::Eof {
            return out;
        }
        out.push((
            token.token_type as u8,
            String::from_utf8_lossy(&token.value).into_owned(),
        ));
    }
}

fn assert_tokens(sql: &str, dbms: Dbms, expected: &[(TokenType, &str)]) {
    let expected: Vec<(u8, String)> = expected
        .iter()
        .map(|(t, v)| (*t as u8, (*v).to_string()))
        .collect();
    assert_eq!(tokens(sql, dbms), expected, "tokenizing {sql:?}");
}

/// Defaults used by the library's own test suite, mirrored by the harness.
fn default_obfuscator() -> Obfuscator {
    Obfuscator::new(ObfuscatorConfig {
        replace_digits: true,
        replace_boolean: true,
        replace_null: true,
        ..Default::default()
    })
}

fn default_normalizer_config() -> NormalizerConfig {
    NormalizerConfig {
        collect_tables: true,
        collect_commands: true,
        collect_comments: true,
        ..Default::default()
    }
}

struct Result {
    sql: String,
    size: usize,
    tables: Vec<String>,
    comments: Vec<String>,
    commands: Vec<String>,
    procedures: Vec<String>,
}

fn run(
    sql: &str,
    dbms: Dbms,
    obfuscator: &Obfuscator,
    normalizer_config: NormalizerConfig,
) -> Result {
    let normalizer = Normalizer::new(normalizer_config);
    let mut out = Vec::new();
    let mut metadata = StatementMetadata::default();
    normalizer.obfuscate_and_normalize_into(
        sql.as_bytes(),
        obfuscator,
        dbms,
        &mut out,
        &mut metadata,
    );
    let strings = |list: &sqllexer::MetadataList| {
        list.values()
            .iter()
            .map(|v| String::from_utf8_lossy(v).into_owned())
            .collect()
    };
    Result {
        sql: String::from_utf8_lossy(&out).into_owned(),
        size: metadata.size,
        tables: strings(&metadata.tables),
        comments: strings(&metadata.comments),
        commands: strings(&metadata.commands),
        procedures: strings(&metadata.procedures),
    }
}

fn normalize(sql: &str, dbms: Dbms) -> Result {
    run(
        sql,
        dbms,
        &default_obfuscator(),
        default_normalizer_config(),
    )
}

#[test]
fn token_type_discriminants_match_go() {
    // The wire protocol transmits token types as integers, so the iota ordering
    // in sqllexer.go is part of the contract.
    assert_eq!(TokenType::Error as u8, 0);
    assert_eq!(TokenType::Eof as u8, 1);
    assert_eq!(TokenType::Space as u8, 2);
    assert_eq!(TokenType::Number as u8, 5);
    assert_eq!(TokenType::Ident as u8, 6);
    assert_eq!(TokenType::Command as u8, 20);
    assert_eq!(TokenType::Keyword as u8, 21);
    assert_eq!(TokenType::JsonOp as u8, 22);
    assert_eq!(TokenType::AliasIndicator as u8, 27);
}

#[test]
fn scans_a_plain_statement() {
    use TokenType::*;
    assert_tokens(
        "SELECT * FROM users WHERE id = 42",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (Wildcard, "*"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "users"),
            (Space, " "),
            (Keyword, "WHERE"),
            (Space, " "),
            (Ident, "id"),
            (Space, " "),
            (Operator, "="),
            (Space, " "),
            (Number, "42"),
        ],
    );
}

#[test]
fn splits_identifiers_on_non_letter_runes() {
    // An emoji is not a Go `unicode.IsLetter`, so it terminates the identifier and
    // is emitted as UNKNOWN rather than folded into it.
    use TokenType::*;
    assert_tokens(
        "SELECT tëst_😀 FROM tbl",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (Ident, "tëst_"),
            (Unknown, "😀"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "tbl"),
        ],
    );
}

#[test]
fn mysql_treats_double_quotes_as_strings_and_backticks_as_identifiers() {
    use TokenType::*;
    assert_tokens(
        "SELECT \"str\", `ident` FROM t",
        Dbms::MySql,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (String, "\"str\""),
            (Punctuation, ","),
            (Space, " "),
            (QuotedIdent, "`ident`"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "t"),
        ],
    );
}

#[test]
fn sqlserver_accepts_bracket_hash_and_dollar_identifiers() {
    use TokenType::*;
    assert_tokens(
        "SELECT [id], #temp, $ident FROM t",
        Dbms::SqlServer,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (QuotedIdent, "[id]"),
            (Punctuation, ","),
            (Space, " "),
            (Ident, "#temp"),
            (Punctuation, ","),
            (Space, " "),
            (Ident, "$ident"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "t"),
        ],
    );
}

#[test]
fn oracle_binds_and_snowflake_stages_are_dbms_specific() {
    use TokenType::*;
    assert_tokens(
        "SELECT :bind FROM t",
        Dbms::Oracle,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (BindParameter, ":bind"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "t"),
        ],
    );
    // The same `@stage` is a bind parameter elsewhere and an identifier here.
    assert_tokens(
        "SELECT @stage FROM t",
        Dbms::Snowflake,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (Ident, "@stage"),
            (Space, " "),
            (Keyword, "FROM"),
            (Space, " "),
            (Ident, "t"),
        ],
    );
    assert_tokens(
        "SELECT @bind",
        Dbms::None,
        &[(Command, "SELECT"), (Space, " "), (BindParameter, "@bind")],
    );
}

#[test]
fn scans_numeric_literal_shapes() {
    // `0o17` is not octal to this lexer: it scans `0` and then an identifier.
    use TokenType::*;
    assert_tokens(
        "SELECT 1.5e-3, 0xFF, -42, 0o17",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (Number, "1.5e-3"),
            (Punctuation, ","),
            (Space, " "),
            (Number, "0xFF"),
            (Punctuation, ","),
            (Space, " "),
            (Number, "-42"),
            (Punctuation, ","),
            (Space, " "),
            (Number, "0"),
            (Ident, "o17"),
        ],
    );
}

#[test]
fn distinguishes_dollar_quoted_functions_strings_and_parameters() {
    use TokenType::*;
    assert_tokens(
        "SELECT $func$ SELECT 1 $func$, $tag$x$tag$, $1",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (DollarQuotedFunction, "$func$ SELECT 1 $func$"),
            (Punctuation, ","),
            (Space, " "),
            (DollarQuotedString, "$tag$x$tag$"),
            (Punctuation, ","),
            (Space, " "),
            (PositionalParameter, "$1"),
        ],
    );
}

#[test]
fn unterminated_string_is_incomplete_not_an_error() {
    use TokenType::*;
    assert_tokens(
        "SELECT 'abc",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (IncompleteString, "'abc"),
        ],
    );
}

#[test]
fn scans_comments_and_json_operators() {
    use TokenType::*;
    assert_tokens(
        "SELECT 1 -- line\n/* block */",
        Dbms::None,
        &[
            (Command, "SELECT"),
            (Space, " "),
            (Number, "1"),
            (Space, " "),
            (Comment, "-- line"),
            (Space, "\n"),
            (MultilineComment, "/* block */"),
        ],
    );
    assert_tokens(
        "a->'b' @> c ?| d @@ e",
        Dbms::None,
        &[
            (Ident, "a"),
            (JsonOp, "->"),
            (String, "'b'"),
            (Space, " "),
            (JsonOp, "@>"),
            (Space, " "),
            (Ident, "c"),
            (Space, " "),
            (JsonOp, "?|"),
            (Space, " "),
            (Ident, "d"),
            (Space, " "),
            (JsonOp, "@@"),
            (Space, " "),
            (Ident, "e"),
        ],
    );
}

#[test]
fn invalid_utf8_is_preserved_byte_for_byte() {
    // Go decodes an invalid byte as U+FFFD but advances by one byte, so each stray
    // byte becomes its own UNKNOWN token carrying the original byte.
    let input = b"SELECT \xff\xfe";
    let mut lexer = Lexer::new(input, Dbms::None);
    let mut values: Vec<Vec<u8>> = Vec::new();
    loop {
        let token = lexer.scan();
        if token.token_type == TokenType::Eof {
            break;
        }
        values.push(token.value.to_vec());
    }
    assert_eq!(
        values,
        vec![b"SELECT".to_vec(), b" ".to_vec(), vec![0xff], vec![0xfe]]
    );
}

#[test]
fn obfuscates_and_normalizes_with_metadata() {
    let result = normalize(
        "SELECT * FROM users WHERE id = 42 AND name = 'x'",
        Dbms::None,
    );
    assert_eq!(result.sql, "SELECT * FROM users WHERE id = ? AND name = ?");
    assert_eq!(result.tables, ["users"]);
    assert_eq!(result.commands, ["SELECT"]);
    assert_eq!(result.size, 11);
}

#[test]
fn metadata_is_deduplicated_and_sized_by_first_occurrence() {
    let result = normalize(
        "/* c */ SELECT * FROM a JOIN b ON a.id=b.id; SELECT * FROM a",
        Dbms::None,
    );
    assert_eq!(
        result.sql,
        "SELECT * FROM a JOIN b ON a.id = b.id; SELECT * FROM a"
    );
    assert_eq!(result.tables, ["a", "b"]);
    assert_eq!(result.commands, ["SELECT", "JOIN"]);
    assert_eq!(result.comments, ["/* c */"]);
    assert!(result.procedures.is_empty());
    // "a" + "b" + "SELECT" + "JOIN" + "/* c */", each counted once.
    assert_eq!(result.size, 19);
}

#[test]
fn collapses_repeated_placeholders_in_a_list() {
    let result = normalize("SELECT * FROM t WHERE id IN (1, 2, 3, 4)", Dbms::None);
    assert_eq!(result.sql, "SELECT * FROM t WHERE id IN ( ? )");
}

#[test]
fn honors_normalizer_options() {
    let obfuscator = default_obfuscator();
    let aliased = run(
        "SELECT a AS b FROM t AS x",
        Dbms::None,
        &obfuscator,
        NormalizerConfig {
            keep_sql_alias: true,
            ..Default::default()
        },
    );
    assert_eq!(aliased.sql, "SELECT a AS b FROM t AS x");

    let stripped = run(
        "SELECT a AS b FROM t AS x",
        Dbms::None,
        &obfuscator,
        NormalizerConfig::default(),
    );
    assert_eq!(stripped.sql, "SELECT a FROM t");

    let uppercased = run(
        "select * from t",
        Dbms::None,
        &obfuscator,
        NormalizerConfig {
            uppercase_keywords: true,
            ..Default::default()
        },
    );
    assert_eq!(uppercased.sql, "SELECT * FROM t");

    let semicolon = run(
        "SELECT 1;",
        Dbms::None,
        &obfuscator,
        NormalizerConfig {
            keep_trailing_semicolon: true,
            ..Default::default()
        },
    );
    assert_eq!(semicolon.sql, "SELECT ?;");
}

#[test]
fn honors_obfuscator_options() {
    let digits = run(
        "SELECT col1, tbl2.col3 FROM tbl2",
        Dbms::None,
        &default_obfuscator(),
        default_normalizer_config(),
    );
    assert_eq!(digits.sql, "SELECT col?, tbl?.col? FROM tbl?");
    assert_eq!(digits.tables, ["tbl?"]);

    let json_path = run(
        "SELECT data->'k'->>'k2' FROM t",
        Dbms::None,
        &Obfuscator::new(ObfuscatorConfig {
            keep_json_path: true,
            replace_digits: true,
            ..Default::default()
        }),
        default_normalizer_config(),
    );
    assert_eq!(json_path.sql, "SELECT data -> 'k' ->> 'k2' FROM t");

    let dollar_func = run(
        "SELECT $func$ SELECT 1 $func$",
        Dbms::None,
        &Obfuscator::new(ObfuscatorConfig {
            dollar_quoted_func: true,
            ..Default::default()
        }),
        default_normalizer_config(),
    );
    assert_eq!(dollar_func.sql, "SELECT $func$SELECT ?$func$");
}

#[test]
fn handles_degenerate_input() {
    for input in ["", "   ", "-- only a comment", "/*", "'", "((((", "\u{a0}"] {
        // A panic on input the Go library accepts is a parity failure by itself.
        let _ = normalize(input, Dbms::None);
    }
    assert_eq!(normalize("", Dbms::None).sql, "");
    assert_eq!(normalize("   ", Dbms::None).sql, "");
}
