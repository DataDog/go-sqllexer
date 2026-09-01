//! Literal obfuscation, port of `obfuscator.go`.

use std::borrow::Cow;

use crate::chars::{equal_fold_ascii, is_digit, trim_space_range, write_rune, Runes};
use crate::lexer::{Dbms, Lexer};
use crate::token::{LastValueToken, Token, TokenType};

pub const STRING_PLACEHOLDER: &[u8] = b"?";
pub const NUMBER_PLACEHOLDER: &[u8] = b"?";

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ObfuscatorConfig {
    pub dollar_quoted_func: bool,
    pub replace_digits: bool,
    pub replace_positional_parameter: bool,
    pub replace_boolean: bool,
    pub replace_null: bool,
    /// When set, values reached through a JSON operator keep their literal form.
    pub keep_json_path: bool,
    pub replace_bind_parameter: bool,
}

pub struct Obfuscator {
    pub(crate) config: ObfuscatorConfig,
}

impl Obfuscator {
    pub fn new(config: ObfuscatorConfig) -> Self {
        Obfuscator { config }
    }

    /// Replaces every literal in `input` with a placeholder.
    pub fn obfuscate(&self, input: &[u8], dbms: Dbms) -> Vec<u8> {
        let mut out = Vec::with_capacity(input.len());
        self.obfuscate_into(input, dbms, &mut out);
        out
    }

    /// Same as [`Obfuscator::obfuscate`] but appends into a caller-owned buffer so
    /// callers processing many statements can reuse one allocation.
    pub fn obfuscate_into(&self, input: &[u8], dbms: Dbms, out: &mut Vec<u8>) {
        let mut lexer = Lexer::new(input, dbms);
        let mut last_value_token: Option<LastValueToken> = None;
        let mut ec = ExtractContext::default();

        let start = out.len();
        loop {
            let mut token = lexer.scan();
            if token.token_type == TokenType::Eof {
                break;
            }
            self.obfuscate_token_value(&mut token, last_value_token.as_ref(), dbms);
            ec.maybe_replace_extract_field(&mut token);

            out.extend_from_slice(&token.value);
            if token.is_value_token() {
                last_value_token = Some(token.last_value());
            }
            ec.update(&token);
        }

        // strings.TrimSpace on the assembled output, in place.
        let (lo, hi) = trim_space_range(&out[start..]);
        out.copy_within(start + lo..start + hi, start);
        out.truncate(start + hi - lo);
    }

    pub(crate) fn obfuscate_token_value<'a>(
        &self,
        token: &mut Token<'a>,
        last_value_token: Option<&LastValueToken<'a>>,
        dbms: Dbms,
    ) {
        let after_json_op = || {
            last_value_token
                .map(|t| t.token_type == TokenType::JsonOp)
                .unwrap_or(false)
        };
        match token.token_type {
            TokenType::Number => {
                if self.config.keep_json_path && after_json_op() {
                    return;
                }
                token.value = Cow::Borrowed(NUMBER_PLACEHOLDER);
            }
            TokenType::DollarQuotedFunction => {
                if self.config.dollar_quoted_func {
                    // Obfuscate the body of the function, keeping the $func$ tags.
                    let body = &token.value[6..token.value.len() - 6];
                    let mut rewritten = Vec::with_capacity(body.len() + 12);
                    rewritten.extend_from_slice(b"$func$");
                    self.obfuscate_into(body, dbms, &mut rewritten);
                    rewritten.extend_from_slice(b"$func$");
                    token.value = Cow::Owned(rewritten);
                    return;
                }
                token.value = Cow::Borrowed(STRING_PLACEHOLDER);
            }
            TokenType::String | TokenType::IncompleteString | TokenType::DollarQuotedString => {
                if self.config.keep_json_path && after_json_op() {
                    return;
                }
                token.value = Cow::Borrowed(STRING_PLACEHOLDER);
            }
            TokenType::PositionalParameter => {
                if self.config.replace_positional_parameter {
                    token.value = Cow::Borrowed(STRING_PLACEHOLDER);
                }
            }
            TokenType::BindParameter => {
                if self.config.replace_bind_parameter {
                    token.value = Cow::Borrowed(STRING_PLACEHOLDER);
                }
            }
            TokenType::Boolean => {
                if self.config.replace_boolean {
                    token.value = Cow::Borrowed(STRING_PLACEHOLDER);
                }
            }
            TokenType::Null => {
                if self.config.replace_null {
                    token.value = Cow::Borrowed(STRING_PLACEHOLDER);
                }
            }
            TokenType::Ident | TokenType::QuotedIdent
                if self.config.replace_digits && token.has_digits =>
            {
                let mut replaced = Vec::with_capacity(token.value.len());
                replace_digits(&token.value, NUMBER_PLACEHOLDER, &mut replaced);
                token.value = Cow::Owned(replaced);
            }
            _ => {}
        }
    }
}

/// Collapses each run of digits into a single placeholder.
pub(crate) fn replace_digits(value: &[u8], placeholder: &[u8], out: &mut Vec<u8>) {
    out.reserve(value.len());
    let mut last_was_digit = false;
    for r in Runes::new(value) {
        if is_digit(r) {
            if !last_was_digit {
                out.extend_from_slice(placeholder);
                last_was_digit = true;
            }
        } else {
            write_rune(out, r);
            last_was_digit = false;
        }
    }
}

/// Tracks the two most recent value tokens so `EXTRACT(field FROM ...)` arguments
/// can be recognized and obfuscated, matching the pg_stat_statements form.
#[derive(Default)]
pub(crate) struct ExtractContext<'a> {
    prev: Option<(TokenType, Cow<'a, [u8]>)>,
    prev2: Option<(TokenType, Cow<'a, [u8]>)>,
}

static EXTRACT_FIELD_KEYWORDS: [&[u8]; 22] = [
    b"CENTURY",
    b"DAY",
    b"DECADE",
    b"DOW",
    b"DOY",
    b"EPOCH",
    b"HOUR",
    b"ISODOW",
    b"ISOYEAR",
    b"JULIAN",
    b"MICROSECONDS",
    b"MILLENNIUM",
    b"MILLISECONDS",
    b"MINUTE",
    b"MONTH",
    b"QUARTER",
    b"SECOND",
    b"TIMEZONE",
    b"TIMEZONE_HOUR",
    b"TIMEZONE_MINUTE",
    b"WEEK",
    b"YEAR",
];

fn is_extract_field_keyword(value: &[u8]) -> bool {
    EXTRACT_FIELD_KEYWORDS
        .iter()
        .any(|kw| equal_fold_ascii(value, kw))
}

impl<'a> ExtractContext<'a> {
    pub(crate) fn maybe_replace_extract_field(&self, token: &mut Token<'a>) {
        if token.token_type != TokenType::Ident && token.token_type != TokenType::Keyword {
            return;
        }
        match &self.prev {
            Some((TokenType::Punctuation, v)) if v.as_ref() == b"(" => {}
            _ => return,
        }
        match &self.prev2 {
            Some((TokenType::Function, v)) if equal_fold_ascii(v, b"EXTRACT") => {}
            _ => return,
        }
        if !is_extract_field_keyword(&token.value) {
            return;
        }
        token.value = Cow::Borrowed(STRING_PLACEHOLDER);
    }

    pub(crate) fn update(&mut self, token: &Token<'a>) {
        if matches!(
            token.token_type,
            TokenType::Space | TokenType::Comment | TokenType::MultilineComment
        ) {
            return;
        }
        self.prev2 = self.prev.take();
        self.prev = Some((token.token_type, token.value.clone()));
    }
}
