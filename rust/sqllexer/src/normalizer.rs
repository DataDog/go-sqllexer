//! Normalization and metadata collection, port of `normalizer.go`.

use std::borrow::Cow;

use crate::chars::{trim_space_range, write_rune, Runes};
use crate::lexer::{Dbms, Lexer};
use crate::obfuscator::{ExtractContext, Obfuscator, NUMBER_PLACEHOLDER, STRING_PLACEHOLDER};
use crate::token::{LastValueToken, Token, TokenType};

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct NormalizerConfig {
    pub collect_tables: bool,
    pub collect_commands: bool,
    pub collect_comments: bool,
    pub collect_procedure: bool,
    pub keep_sql_alias: bool,
    pub uppercase_keywords: bool,
    pub remove_space_between_parentheses: bool,
    pub keep_trailing_semicolon: bool,
    pub keep_identifier_quotation: bool,
}

/// Deduplicated metadata values in first-seen order, matching the Go output.
///
/// Dedup is a linear scan guarded by a stored hash rather than a hash map: these
/// lists almost always hold a handful of entries, and avoiding the map keeps the
/// common statement allocation-light.
#[derive(Debug, Default, Clone)]
pub struct MetadataList {
    values: Vec<Vec<u8>>,
    hashes: Vec<u64>,
}

impl MetadataList {
    pub fn values(&self) -> &[Vec<u8>] {
        &self.values
    }

    pub fn is_empty(&self) -> bool {
        self.values.is_empty()
    }

    fn clear(&mut self) {
        self.values.clear();
        self.hashes.clear();
    }

    /// Inserts `value` if absent, returning the number of bytes it contributes to
    /// the metadata size (0 when it was already present).
    fn insert(&mut self, value: &[u8]) -> usize {
        let hash = fnv1a(value);
        for (i, h) in self.hashes.iter().enumerate() {
            if *h == hash && self.values[i] == value {
                return 0;
            }
        }
        self.hashes.push(hash);
        self.values.push(value.to_vec());
        value.len()
    }
}

fn fnv1a(bytes: &[u8]) -> u64 {
    let mut hash: u64 = 0xcbf2_9ce4_8422_2325;
    for &b in bytes {
        hash ^= b as u64;
        hash = hash.wrapping_mul(0x100_0000_01b3);
    }
    hash
}

#[derive(Debug, Default, Clone)]
pub struct StatementMetadata {
    pub size: usize,
    pub tables: MetadataList,
    pub comments: MetadataList,
    pub commands: MetadataList,
    pub procedures: MetadataList,
}

impl StatementMetadata {
    pub fn clear(&mut self) {
        self.size = 0;
        self.tables.clear();
        self.comments.clear();
        self.commands.clear();
        self.procedures.clear();
    }
}

pub struct Normalizer {
    config: NormalizerConfig,
}

#[derive(Default)]
struct HeadState {
    read_first_non_space_non_comment: bool,
    in_leading_parentheses_expression: bool,
    found_leading_expression_in_parentheses: bool,
    standalone_expression_in_parentheses: bool,
    expression_in_parentheses: Vec<u8>,
    has_command_in_leading_parentheses: bool,
    parentheses_depth: i32,
}

#[derive(Default)]
struct ColonContext {
    token_type_before_colon: Option<TokenType>,
    has_colon: bool,
}

impl Normalizer {
    pub fn new(config: NormalizerConfig) -> Self {
        Normalizer { config }
    }

    /// Normalizes `input`, appending the result to `out` and filling `metadata`.
    /// Both are caller-owned so a long-lived handle can reuse their allocations.
    pub fn normalize_into(
        &self,
        input: &[u8],
        dbms: Dbms,
        out: &mut Vec<u8>,
        metadata: &mut StatementMetadata,
    ) {
        self.run(input, dbms, None, out, metadata)
    }

    pub fn normalize(&self, input: &[u8], dbms: Dbms) -> (Vec<u8>, StatementMetadata) {
        let mut out = Vec::with_capacity(input.len());
        let mut metadata = StatementMetadata::default();
        self.normalize_into(input, dbms, &mut out, &mut metadata);
        (out, metadata)
    }

    /// Obfuscates and normalizes in a single pass, the combination the tracers use.
    pub fn obfuscate_and_normalize_into(
        &self,
        input: &[u8],
        obfuscator: &Obfuscator,
        dbms: Dbms,
        out: &mut Vec<u8>,
        metadata: &mut StatementMetadata,
    ) {
        self.run(input, dbms, Some(obfuscator), out, metadata)
    }

    fn run(
        &self,
        input: &[u8],
        dbms: Dbms,
        obfuscator: Option<&Obfuscator>,
        out: &mut Vec<u8>,
        metadata: &mut StatementMetadata,
    ) {
        out.clear();
        out.reserve(input.len());
        metadata.clear();

        let mut lexer = Lexer::new(input, dbms);
        let mut groupable = false;
        let mut head = HeadState::default();
        let mut colon_ctx = ColonContext::default();
        let mut ctes: Vec<Vec<u8>> = Vec::new();
        let mut in_table_list = false;
        let mut last_value_token: Option<LastValueToken> = None;
        let mut ec = ExtractContext::default();

        loop {
            let mut token = lexer.scan();
            if let Some(obfuscator) = obfuscator {
                obfuscator.obfuscate_token_value(&mut token, last_value_token.as_ref(), dbms);
                ec.maybe_replace_extract_field(&mut token);
                ec.update(&token);
            }
            if self.should_collect_metadata() {
                self.collect_metadata(
                    &mut token,
                    last_value_token.as_ref(),
                    metadata,
                    &mut ctes,
                    &mut in_table_list,
                );
            }
            self.normalize_sql(
                &mut token,
                last_value_token.as_ref(),
                out,
                &mut groupable,
                &mut head,
                &mut colon_ctx,
            );
            if token.token_type == TokenType::Eof {
                break;
            }
            if token.is_value_token() {
                last_value_token = Some(token.last_value());
            }
        }

        self.trim_normalized_sql(out);
    }

    fn should_collect_metadata(&self) -> bool {
        self.config.collect_tables
            || self.config.collect_commands
            || self.config.collect_comments
            || self.config.collect_procedure
    }

    fn collect_metadata<'a>(
        &self,
        token: &mut Token<'a>,
        last_value_token: Option<&LastValueToken<'a>>,
        metadata: &mut StatementMetadata,
        ctes: &mut Vec<Vec<u8>>,
        in_table_list: &mut bool,
    ) {
        if self.config.collect_comments
            && matches!(
                token.token_type,
                TokenType::Comment | TokenType::MultilineComment
            )
        {
            metadata.size += metadata.comments.insert(&token.value);
            return;
        }
        if matches!(token.token_type, TokenType::Command | TokenType::Keyword) {
            *in_table_list = false;
            if self.config.collect_commands && token.token_type == TokenType::Command {
                let command = token.value.to_ascii_uppercase();
                metadata.size += metadata.commands.insert(&command);
            }
            return;
        }
        if token.token_type == TokenType::Punctuation
            && (token.value.as_ref() == b"(" || token.value.as_ref() == b")")
        {
            *in_table_list = false;
            return;
        }
        if !matches!(
            token.token_type,
            TokenType::Ident | TokenType::QuotedIdent | TokenType::Function
        ) {
            return;
        }

        let mut token_val: Cow<'a, [u8]> = token.value.clone();
        if token.token_type == TokenType::QuotedIdent {
            let trimmed = trim_quotes(&token.value);
            let strip = self.should_strip_identifier_quotes(token, last_value_token);
            token_val = Cow::Owned(trimmed);
            if strip {
                token.value = token_val.clone();
                token.token_type = TokenType::Ident;
            }
        }

        let Some(last) = last_value_token else {
            return;
        };
        if last.token_type == TokenType::CteIndicator {
            if !ctes.iter().any(|c| c.as_slice() == token_val.as_ref()) {
                ctes.push(token_val.to_vec());
            }
        } else if self.config.collect_tables && last.is_table_indicator {
            *in_table_list = true;
            if !ctes.iter().any(|c| c.as_slice() == token_val.as_ref()) {
                metadata.size += metadata.tables.insert(&token_val);
            }
        } else if self.config.collect_tables
            && *in_table_list
            && last.token_type == TokenType::Punctuation
            && last.value.as_ref() == b","
        {
            if !ctes.iter().any(|c| c.as_slice() == token_val.as_ref()) {
                metadata.size += metadata.tables.insert(&token_val);
            }
        } else if self.config.collect_procedure && last.token_type == TokenType::ProcIndicator {
            metadata.size += metadata.procedures.insert(&token_val);
        }
    }

    #[allow(clippy::too_many_arguments)]
    fn normalize_sql<'a>(
        &self,
        token: &mut Token<'a>,
        last_value_token: Option<&LastValueToken<'a>>,
        out: &mut Vec<u8>,
        groupable: &mut bool,
        head: &mut HeadState,
        colon_ctx: &mut ColonContext,
    ) {
        if matches!(
            token.token_type,
            TokenType::Space | TokenType::Comment | TokenType::MultilineComment
        ) {
            return;
        }

        if token.token_type == TokenType::QuotedIdent
            && !self.config.keep_identifier_quotation
            && self.should_strip_identifier_quotes(token, last_value_token)
        {
            token.value = Cow::Owned(trim_quotes(&token.value));
        }

        // A statement that opens with a parenthesized expression is buffered
        // separately: it is only emitted if it turns out to be a real expression
        // rather than a parameter declaration.
        if !head.read_first_non_space_non_comment {
            head.read_first_non_space_non_comment = true;
            if token.token_type == TokenType::Punctuation && token.value.as_ref() == b"(" {
                head.in_leading_parentheses_expression = true;
                head.standalone_expression_in_parentheses = true;
                head.parentheses_depth = 1;
                head.expression_in_parentheses
                    .extend_from_slice(&token.value);
                return;
            }
        }

        if token.token_type == TokenType::Eof {
            if head.standalone_expression_in_parentheses {
                out.extend_from_slice(&head.expression_in_parentheses);
            }
            return;
        } else if head.found_leading_expression_in_parentheses {
            if head.has_command_in_leading_parentheses {
                out.extend_from_slice(&head.expression_in_parentheses);
            }
            head.standalone_expression_in_parentheses = false;
            head.found_leading_expression_in_parentheses = false;
        }

        if token.token_type == TokenType::DollarQuotedFunction
            && token.value.as_ref() != STRING_PLACEHOLDER
        {
            // Not obfuscated, so normalize the function body recursively.
            // The Go implementation recurses without lexer options, so the body is
            // always normalized in the default dialect.
            let body = &token.value[6..token.value.len() - 6];
            let (normalized, _) = self.normalize(body, Dbms::None);
            let mut rewritten = Vec::with_capacity(normalized.len() + 12);
            rewritten.extend_from_slice(b"$func$");
            rewritten.extend_from_slice(&normalized);
            rewritten.extend_from_slice(b"$func$");
            token.value = Cow::Owned(rewritten);
        }

        if !self.config.keep_sql_alias {
            if token.token_type == TokenType::AliasIndicator {
                return;
            }
            if let Some(last) = last_value_token {
                if last.token_type == TokenType::AliasIndicator {
                    if matches!(token.token_type, TokenType::Ident | TokenType::QuotedIdent) {
                        return;
                    }
                    // Not an alias after all (e.g. WITH x AS (...)), so the
                    // suppressed AS has to be written back out.
                    self.append_space(token, last_value_token, out, colon_ctx);
                    self.write_token(last.token_type, &last.value, out);
                }
            }
        }

        if self.is_obfuscated_value_groupable(token, last_value_token, groupable, out) {
            return;
        }

        if head.in_leading_parentheses_expression {
            let buffer = &mut head.expression_in_parentheses;
            self.append_space(token, last_value_token, buffer, colon_ctx);
            self.write_token(token.token_type, &token.value, buffer);
            if token.token_type == TokenType::Command {
                head.has_command_in_leading_parentheses = true;
            }
            if token.token_type == TokenType::Punctuation {
                if token.value.as_ref() == b"(" {
                    head.parentheses_depth += 1;
                } else if token.value.as_ref() == b")" {
                    head.parentheses_depth -= 1;
                    if head.parentheses_depth == 0 {
                        head.in_leading_parentheses_expression = false;
                        head.found_leading_expression_in_parentheses = true;
                    }
                }
            }
        } else {
            self.append_space(token, last_value_token, out, colon_ctx);
            self.write_token(token.token_type, &token.value, out);
        }

        if token.value.as_ref() == b":" {
            colon_ctx.has_colon = true;
            colon_ctx.token_type_before_colon = last_value_token.map(|t| t.token_type);
        } else if colon_ctx.has_colon {
            colon_ctx.has_colon = false;
            colon_ctx.token_type_before_colon = None;
        }
    }

    fn should_strip_identifier_quotes(
        &self,
        token: &Token,
        last_value_token: Option<&LastValueToken>,
    ) -> bool {
        if self.config.keep_identifier_quotation {
            return false;
        }
        let alias_context = last_value_token
            .map(|t| t.token_type == TokenType::AliasIndicator)
            .unwrap_or(false);
        if alias_context && !token.is_simple_identifier {
            return false;
        }
        // Unquoting a bracketed identifier containing whitespace would split it
        // into several tokens, changing the statement.
        !token
            .value
            .iter()
            .any(|b| matches!(b, b' ' | b'\t' | b'\n' | b'\r'))
    }

    fn write_token(&self, token_type: TokenType, value: &[u8], out: &mut Vec<u8>) {
        if self.config.uppercase_keywords
            && matches!(token_type, TokenType::Command | TokenType::Keyword)
        {
            out.extend(value.iter().map(|b| b.to_ascii_uppercase()));
        } else {
            out.extend_from_slice(value);
        }
    }

    /// Collapses `(?, ?, ?)` style runs into a single placeholder.
    fn is_obfuscated_value_groupable(
        &self,
        token: &Token,
        last_value_token: Option<&LastValueToken>,
        groupable: &mut bool,
        out: &mut Vec<u8>,
    ) -> bool {
        let is_placeholder = |v: &[u8]| v == NUMBER_PLACEHOLDER || v == STRING_PLACEHOLDER;

        if is_placeholder(&token.value) {
            let Some(last) = last_value_token else {
                return false;
            };
            if last.value.as_ref() == b"(" || last.value.as_ref() == b"[" {
                *groupable = true;
            } else if last.value.as_ref() == b"," && *groupable {
                return true;
            }
        }

        if let Some(last) = last_value_token {
            if is_placeholder(&last.value) && token.value.as_ref() == b"," && *groupable {
                return true;
            }
        }

        if *groupable && (token.value.as_ref() == b")" || token.value.as_ref() == b"]") {
            *groupable = false;
            return false;
        }

        if *groupable && !is_placeholder(&token.value) {
            if let Some(last) = last_value_token {
                if last.value.as_ref() == b"," {
                    // Leaving a groupable run, so the comma skipped earlier has to
                    // be written before the current token: (?, ARRAY[?, ?]) becomes
                    // (?, ARRAY[?]).
                    out.extend_from_slice(&last.value);
                }
            }
        }

        false
    }

    fn append_space(
        &self,
        token: &Token,
        last_value_token: Option<&LastValueToken>,
        out: &mut Vec<u8>,
        colon_ctx: &ColonContext,
    ) {
        if self.config.remove_space_between_parentheses {
            if let Some(last) = last_value_token {
                if last.token_type == TokenType::Function
                    || last.value.as_ref() == b"("
                    || last.value.as_ref() == b"["
                {
                    return;
                }
            }
            if token.value.as_ref() == b")" || token.value.as_ref() == b"]" {
                return;
            }
        }

        if let Some(last) = last_value_token {
            // The lexer splits `t.[Col]` into `t.` and `[Col]`, which must not be
            // rejoined with a space.
            if last.value.len() > 1
                && last.value[last.value.len() - 1] == b'.'
                && matches!(token.token_type, TokenType::Ident | TokenType::QuotedIdent)
            {
                return;
            }

            // Oracle bind variables (`= :param`) take no space, but MySQL labels
            // and Snowflake semi-structured access (`obj:field`) do.
            if last.value.as_ref() == b":"
                && matches!(token.token_type, TokenType::Ident | TokenType::QuotedIdent)
                && colon_ctx.has_colon
                && !matches!(
                    colon_ctx.token_type_before_colon,
                    Some(TokenType::Ident) | Some(TokenType::QuotedIdent)
                )
            {
                return;
            }
        }

        match token.value.as_ref() {
            b"," | b";" => (),
            b"=" => {
                if last_value_token
                    .map(|t| t.value.as_ref() == b":")
                    .unwrap_or(false)
                {
                    return;
                }
                out.push(b' ');
            }
            _ => out.push(b' '),
        }
    }

    fn trim_normalized_sql(&self, out: &mut Vec<u8>) {
        if !self.config.keep_trailing_semicolon && out.last() == Some(&b';') {
            out.pop();
        }
        let (lo, hi) = trim_space_range(out);
        out.copy_within(lo..hi, 0);
        out.truncate(hi - lo);
    }
}

/// Drops the quoting characters from an identifier value.
pub(crate) fn trim_quotes(value: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(value.len());
    for r in Runes::new(value) {
        if r == b'"' as u32 || r == b'[' as u32 || r == b']' as u32 || r == b'`' as u32 {
            continue;
        }
        write_rune(&mut out, r);
    }
    out
}
