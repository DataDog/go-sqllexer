//! Byte-for-byte port of the Go lexer.
//!
//! The structure deliberately mirrors `sqllexer.go` scanner for scanner: this is
//! the component every other part is validated against, so divergence in shape
//! would make differential failures hard to localize. The Go `hasQuotes` flag is
//! omitted because it is never read outside the Go package's own tests.

use crate::chars::*;
use crate::keywords::{keyword_trie, trie_index, Trie};
use crate::token::{Token, TokenType};

/// The dialects the lexer changes behavior for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Dbms {
    #[default]
    None,
    SqlServer,
    Postgres,
    MySql,
    Oracle,
    Snowflake,
}

impl Dbms {
    /// Resolves the tracer-specific aliases the Go package accepts.
    pub fn from_name(name: &str) -> Dbms {
        match name {
            "mssql" | "sql-server" | "sqlserver" => Dbms::SqlServer,
            "postgresql" | "postgres" => Dbms::Postgres,
            "mysql" => Dbms::MySql,
            "oracle" => Dbms::Oracle,
            "snowflake" => Dbms::Snowflake,
            _ => Dbms::None,
        }
    }
}

pub struct Lexer<'a> {
    src: &'a [u8],
    cursor: usize,
    start: usize,
    dbms: Dbms,
    trie: &'static Trie,
    has_digits: bool,
    is_table_indicator: bool,
    is_simple_identifier: bool,
}

impl<'a> Lexer<'a> {
    pub fn new(src: &'a [u8], dbms: Dbms) -> Self {
        Lexer {
            src,
            cursor: 0,
            start: 0,
            dbms,
            trie: keyword_trie(),
            has_digits: false,
            is_table_indicator: false,
            is_simple_identifier: false,
        }
    }

    pub fn scan(&mut self) -> Token<'a> {
        let ch = self.peek();
        if is_space(ch) {
            return self.scan_whitespace();
        }
        if is_letter(ch) {
            return self.scan_identifier(ch);
        }
        if is_double_quote(ch) {
            // MySQL without ANSI_QUOTES treats double quotes as string literals.
            if self.dbms == Dbms::MySql {
                return self.scan_string_with_delimiter(b'"' as u32);
            }
            return self.scan_double_quoted_identifier(b'"' as u32);
        }
        if is_single_quote(ch) {
            return self.scan_string_with_delimiter(b'\'' as u32);
        }
        if is_single_line_comment(ch, self.look_ahead(1)) {
            return self.scan_single_line_comment(ch);
        }
        if is_multi_line_comment(ch, self.look_ahead(1)) {
            return self.scan_multi_line_comment();
        }
        if is_leading_sign(ch) {
            let next_ch = self.look_ahead(1);
            if is_digit(next_ch) || next_ch == b'.' as u32 {
                return self.scan_number_with_leading_sign();
            }
            return self.scan_operator(ch);
        }
        if is_digit(ch) {
            return self.scan_number(ch);
        }
        if is_wildcard(ch) {
            return self.scan_wildcard();
        }
        if ch == b'$' as u32 {
            if is_digit(self.look_ahead(1)) {
                return self.scan_positional_parameter();
            }
            if self.dbms == Dbms::SqlServer && is_letter(self.look_ahead(1)) {
                return self.scan_identifier(ch);
            }
            return self.scan_dollar_quoted_string();
        }
        if ch == b':' as u32 {
            if self.dbms == Dbms::Oracle && is_alpha_numeric(self.look_ahead(1)) {
                return self.scan_bind_parameter();
            }
            return self.scan_operator(ch);
        }
        if ch == b'`' as u32 {
            if self.dbms == Dbms::MySql {
                return self.scan_double_quoted_identifier(b'`' as u32);
            }
            return self.scan_unknown(); // backticks are only valid in MySQL
        }
        if ch == b'#' as u32 {
            if self.dbms == Dbms::SqlServer {
                return self.scan_identifier(ch);
            }
            if self.dbms == Dbms::MySql {
                return self.scan_single_line_comment(ch);
            }
            return self.scan_operator(ch);
        }
        if ch == b'@' as u32 {
            if self.look_ahead(1) == b'@' as u32 {
                if is_alpha_numeric(self.look_ahead(2)) {
                    return self.scan_system_variable();
                }
                self.start = self.cursor;
                self.next_by(2); // consume @@
                return self.emit(TokenType::JsonOp);
            }
            if is_alpha_numeric(self.look_ahead(1)) {
                if self.dbms == Dbms::Snowflake {
                    return self.scan_identifier(ch);
                }
                return self.scan_bind_parameter();
            }
            if self.look_ahead(1) == b'?' as u32 || self.look_ahead(1) == b'>' as u32 {
                self.start = self.cursor;
                self.next_by(2); // consume @? or @>
                return self.emit(TokenType::JsonOp);
            }
            // falls through to the operator case, as in Go
            return self.scan_operator(ch);
        }
        if is_operator(ch) {
            return self.scan_operator(ch);
        }
        if is_punctuation(ch) {
            if ch == b'[' as u32 && self.dbms == Dbms::SqlServer {
                return self.scan_double_quoted_identifier(b'[' as u32);
            }
            return self.scan_punctuation();
        }
        if is_eof(ch) {
            return self.emit(TokenType::Eof);
        }
        self.scan_unknown()
    }

    #[inline]
    fn look_ahead(&self, n: usize) -> u32 {
        let pos = self.cursor + n;
        let Some(&b) = self.src.get(pos) else {
            return EOF;
        };
        if b < 0x80 {
            return b as u32;
        }
        decode_rune(self.src, pos).0
    }

    #[inline]
    fn peek(&self) -> u32 {
        self.look_ahead(0)
    }

    /// Advances by `n` bytes and returns the rune now under the cursor. Matches the
    /// Go behavior of refusing to move past the end and reporting EOF instead.
    #[inline]
    fn next_by(&mut self, n: usize) -> u32 {
        if self.cursor + n > self.src.len() {
            return EOF;
        }
        self.cursor += n;
        let Some(&b) = self.src.get(self.cursor) else {
            return EOF;
        };
        if b < 0x80 {
            return b as u32;
        }
        decode_rune(self.src, self.cursor).0
    }

    #[inline]
    fn next(&mut self) -> u32 {
        self.next_by(1)
    }

    /// Compares runes against raw bytes at the cursor, truncating each rune to its
    /// low byte exactly like Go's `byte(ch)` conversion.
    fn match_at(&self, pattern: &[u32]) -> bool {
        if self.cursor + pattern.len() > self.src.len() {
            return false;
        }
        pattern
            .iter()
            .enumerate()
            .all(|(i, &ch)| self.src[self.cursor + i] == ch as u8)
    }

    fn scan_number_with_leading_sign(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let ch = self.next(); // consume the leading sign
        self.scan_decimal_number(ch)
    }

    fn scan_number(&mut self, ch: u32) -> Token<'a> {
        self.start = self.cursor;
        self.scan_numeric(ch)
    }

    fn scan_numeric(&mut self, ch: u32) -> Token<'a> {
        self.start = self.cursor;
        if ch == b'0' as u32 {
            let next_ch = self.look_ahead(1);
            if next_ch == b'x' as u32 || next_ch == b'X' as u32 {
                return self.scan_hex_number();
            } else if (b'0' as u32..=b'7' as u32).contains(&next_ch) {
                return self.scan_octal_number();
            }
        }
        let ch = self.next(); // consume the first digit
        self.scan_decimal_number(ch)
    }

    fn scan_decimal_number(&mut self, mut ch: u32) -> Token<'a> {
        while is_digit(ch) || ch == b'.' as u32 || is_exponent(ch) {
            if is_exponent(ch) {
                ch = self.next();
                if is_leading_sign(ch) {
                    ch = self.next();
                }
            } else {
                ch = self.next();
            }
        }
        self.emit(TokenType::Number)
    }

    fn scan_hex_number(&mut self) -> Token<'a> {
        let mut ch = self.next_by(2); // consume 0x or 0X
        while is_digit(ch)
            || (b'a' as u32..=b'f' as u32).contains(&ch)
            || (b'A' as u32..=b'F' as u32).contains(&ch)
        {
            ch = self.next();
        }
        self.emit(TokenType::Number)
    }

    fn scan_octal_number(&mut self) -> Token<'a> {
        let mut ch = self.next_by(2); // consume the leading zero and first digit
        while (b'0' as u32..=b'7' as u32).contains(&ch) {
            ch = self.next();
        }
        self.emit(TokenType::Number)
    }

    fn scan_string_with_delimiter(&mut self, delimiter: u32) -> Token<'a> {
        self.start = self.cursor;
        let mut escaped = false;
        let mut escaped_quote = false;

        // T-SQL and Oracle escape a quote by doubling it, not with a backslash.
        let backslash_escapes = self.dbms != Dbms::SqlServer && self.dbms != Dbms::Oracle;

        let mut ch = self.next(); // consume the opening quote
        while !is_eof(ch) {
            if escaped {
                escaped = false;
                escaped_quote = ch == delimiter;
                ch = self.next();
                continue;
            }
            if backslash_escapes && ch == b'\\' as u32 {
                escaped = true;
                ch = self.next();
                continue;
            }
            if ch == delimiter {
                self.next(); // consume the closing quote
                return self.emit(TokenType::String);
            }
            ch = self.next();
        }
        // A trailing escaped quote (e.g. ESCAPE '\') still counts as a string.
        if escaped_quote {
            return self.emit(TokenType::String);
        }
        self.emit(TokenType::IncompleteString)
    }

    fn scan_identifier(&mut self, mut ch: u32) -> Token<'a> {
        self.start = self.cursor;
        let mut node = self.trie.root();
        let mut pos = self.cursor;

        // Non-ASCII first character can never start a keyword.
        if ch > 127 {
            while is_identifier(ch) {
                self.has_digits = self.has_digits || is_digit(ch);
                ch = self.next_by(rune_len(ch));
            }
            if self.start == self.cursor {
                return self.scan_unknown();
            }
            return self.emit(TokenType::Ident);
        }

        while is_ascii_letter(ch) || ch == b'_' as u32 {
            let upper_ch = if (b'a' as u32..=b'z' as u32).contains(&ch) {
                ch - 32
            } else {
                ch
            };
            let Some(slot) = trie_index(upper_ch) else {
                node = self.trie.root();
                ch = self.next();
                break;
            };
            match self.trie.child(node, slot) {
                Some(next) => {
                    node = next;
                    pos = self.cursor;
                    ch = self.next();
                }
                None => {
                    node = self.trie.root();
                    ch = self.next();
                    break;
                }
            }
        }

        let matched = self.trie.node(node);
        if matched.is_end
            && (is_punctuation(ch)
                || is_space(ch)
                || is_multi_line_comment(ch, self.look_ahead(1))
                || is_eof(ch))
        {
            self.cursor = pos + 1; // include the last matched character
            self.is_table_indicator = matched.is_table_indicator;
            let token_type = matched.token_type;
            return self.emit(token_type);
        }

        while is_identifier(ch) {
            self.has_digits = self.has_digits || is_digit(ch);
            ch = self.next_by(rune_len(ch));
        }

        if self.start == self.cursor {
            return self.scan_unknown();
        }
        if ch == b'(' as u32 {
            return self.emit(TokenType::Function);
        }
        self.emit(TokenType::Ident)
    }

    fn scan_double_quoted_identifier(&mut self, delimiter: u32) -> Token<'a> {
        let closing_delimiter = if delimiter == b'[' as u32 {
            b']' as u32
        } else {
            delimiter
        };

        self.start = self.cursor;
        self.is_simple_identifier = true;
        let mut first_rune = true;
        let mut ch = self.next(); // consume the opening quote
        let special_case = [closing_delimiter, b'.' as u32, delimiter];
        loop {
            // A closing quote followed by `."` continues the identifier, so that
            // postgres "foo"."bar" and sqlserver [foo].[bar] stay one token.
            if ch == closing_delimiter {
                if self.match_at(&special_case) {
                    self.is_simple_identifier = false;
                    ch = self.next_by(3);
                    continue;
                }
                if first_rune {
                    self.is_simple_identifier = false;
                }
                break;
            }
            if is_eof(ch) {
                self.is_simple_identifier = false;
                return self.emit(TokenType::Error);
            }
            self.has_digits = self.has_digits || is_digit(ch);
            if self.is_simple_identifier {
                if first_rune {
                    if !is_letter(ch) {
                        self.is_simple_identifier = false;
                    }
                    first_rune = false;
                } else if !is_alpha_numeric(ch) {
                    self.is_simple_identifier = false;
                }
            }
            // Advance by the decoded size so truncated UTF-8 is handled the same way.
            let (_, size) = decode_rune(self.src, self.cursor);
            ch = self.next_by(size);
        }
        self.next(); // consume the closing quote (always ASCII)
        self.emit(TokenType::QuotedIdent)
    }

    fn scan_whitespace(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next();
        while is_space(ch) {
            ch = self.next();
        }
        self.emit(TokenType::Space)
    }

    fn scan_operator(&mut self, last_ch: u32) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next(); // consume the first character
        let mut last_ch = last_ch;

        match last_ch as u8 {
            b'-' => {
                if ch == b'>' as u32 {
                    ch = self.next();
                    if ch == b'>' as u32 {
                        self.next();
                        return self.emit(TokenType::JsonOp); // ->>
                    }
                    return self.emit(TokenType::JsonOp); // ->
                }
            }
            b'#' => {
                if ch == b'>' as u32 {
                    ch = self.next();
                    if ch == b'>' as u32 {
                        self.next();
                        return self.emit(TokenType::JsonOp); // #>>
                    }
                    return self.emit(TokenType::JsonOp); // #>
                } else if ch == b'-' as u32 {
                    self.next();
                    return self.emit(TokenType::JsonOp); // #-
                }
            }
            b'?' => {
                if ch == b'|' as u32 {
                    self.next();
                    return self.emit(TokenType::JsonOp); // ?|
                } else if ch == b'&' as u32 {
                    self.next();
                    return self.emit(TokenType::JsonOp); // ?&
                }
            }
            b'<' if ch == b'@' as u32 => {
                self.next();
                return self.emit(TokenType::JsonOp); // <@
            }
            _ => {}
        }

        // "=?" and "=@" must not be absorbed into a single operator.
        while is_operator(ch)
            && !(last_ch == b'=' as u32 && (ch == b'?' as u32 || ch == b'@' as u32))
        {
            last_ch = ch;
            ch = self.next();
        }

        self.emit(TokenType::Operator)
    }

    fn scan_wildcard(&mut self) -> Token<'a> {
        self.start = self.cursor;
        self.next();
        self.emit(TokenType::Wildcard)
    }

    fn scan_single_line_comment(&mut self, ch: u32) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = if ch == b'#' as u32 {
            self.next() // consume the opening #
        } else {
            self.next_by(2) // consume the opening dashes
        };
        while ch != b'\n' as u32 && !is_eof(ch) {
            ch = self.next();
        }
        self.emit(TokenType::Comment)
    }

    fn scan_multi_line_comment(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next_by(2); // consume the opening /*
        loop {
            if ch == b'*' as u32 && self.look_ahead(1) == b'/' as u32 {
                self.next_by(2);
                break;
            }
            if is_eof(ch) {
                // Truncated comment.
                return self.emit(TokenType::Error);
            }
            ch = self.next();
        }
        self.emit(TokenType::MultilineComment)
    }

    fn scan_punctuation(&mut self) -> Token<'a> {
        self.start = self.cursor;
        self.next();
        self.emit(TokenType::Punctuation)
    }

    fn scan_dollar_quoted_string(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next(); // consume the dollar sign
        let tag_start = self.cursor;

        while self.cursor < self.src.len() && ch != b'$' as u32 {
            ch = self.next();
        }
        self.next(); // consume the closing dollar sign of the tag
        let tag = &self.src[tag_start - 1..self.cursor]; // includes both dollar signs
        let tag_runes: Vec<u32> = Runes::new(tag).collect();
        let tag_len = tag_runes.len();

        while self.cursor < self.src.len() {
            if self.match_at(&tag_runes) {
                self.next_by(tag_len); // consume the closing tag
                if tag == b"$func$" {
                    return self.emit(TokenType::DollarQuotedFunction);
                }
                return self.emit(TokenType::DollarQuotedString);
            }
            self.next();
        }
        self.emit(TokenType::Error)
    }

    fn scan_positional_parameter(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next_by(2); // consume the dollar sign and first digit
        while is_digit(ch) {
            ch = self.next();
        }
        self.emit(TokenType::PositionalParameter)
    }

    fn scan_bind_parameter(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next_by(2); // consume the colon or at sign and first char
        while is_alpha_numeric(ch) {
            ch = self.next();
        }
        self.emit(TokenType::BindParameter)
    }

    fn scan_system_variable(&mut self) -> Token<'a> {
        self.start = self.cursor;
        let mut ch = self.next_by(2); // consume @@
        if !is_alpha_numeric(ch) {
            return self.emit(TokenType::Error);
        }
        while is_alpha_numeric(ch) {
            ch = self.next();
        }
        self.emit(TokenType::SystemVariable)
    }

    fn scan_unknown(&mut self) -> Token<'a> {
        // Advance a whole rune so multi-byte characters are not split into
        // separate single-byte tokens.
        self.start = self.cursor;
        let (_, size) = decode_rune(self.src, self.cursor);
        self.cursor += size;
        self.emit(TokenType::Unknown)
    }

    fn emit(&mut self, token_type: TokenType) -> Token<'a> {
        let mut token = Token::new(token_type, &self.src[self.start..self.cursor]);
        token.is_table_indicator = self.is_table_indicator;
        token.has_digits = self.has_digits;
        token.is_simple_identifier = self.is_simple_identifier;

        self.start = self.cursor;
        self.is_table_indicator = false;
        self.has_digits = false;
        self.is_simple_identifier = false;

        token
    }
}
