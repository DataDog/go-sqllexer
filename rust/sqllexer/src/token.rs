use std::borrow::Cow;

/// Token kinds. The discriminants match the Go `TokenType` iota values because the
/// differential harness compares token streams by numeric type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum TokenType {
    Error = 0,
    Eof,
    Space,
    String,
    IncompleteString,
    Number,
    Ident,
    QuotedIdent,
    Operator,
    Wildcard,
    Comment,
    MultilineComment,
    Punctuation,
    DollarQuotedFunction,
    DollarQuotedString,
    PositionalParameter,
    BindParameter,
    Function,
    SystemVariable,
    Unknown,
    Command,
    Keyword,
    JsonOp,
    Boolean,
    Null,
    ProcIndicator,
    CteIndicator,
    AliasIndicator,
}

/// A scanned token. `value` borrows from the input until an obfuscation or
/// normalization step has to rewrite it, which keeps the common path allocation
/// free.
#[derive(Debug, Clone)]
pub struct Token<'a> {
    pub token_type: TokenType,
    pub value: Cow<'a, [u8]>,
    pub(crate) is_table_indicator: bool,
    pub(crate) has_digits: bool,
    pub(crate) is_simple_identifier: bool,
}

impl<'a> Token<'a> {
    pub(crate) fn new(token_type: TokenType, value: &'a [u8]) -> Self {
        Token {
            token_type,
            value: Cow::Borrowed(value),
            is_table_indicator: false,
            has_digits: false,
            is_simple_identifier: false,
        }
    }

    /// A value token is anything that is not whitespace, a comment or EOF; only
    /// these update the "last value token" context the obfuscator and normalizer
    /// make decisions from.
    #[inline]
    pub(crate) fn is_value_token(&self) -> bool {
        !matches!(
            self.token_type,
            TokenType::Eof | TokenType::Space | TokenType::Comment | TokenType::MultilineComment
        )
    }

    pub(crate) fn last_value(&self) -> LastValueToken<'a> {
        LastValueToken {
            token_type: self.token_type,
            value: self.value.clone(),
            is_table_indicator: self.is_table_indicator,
        }
    }
}

/// The subset of a token that later tokens are interpreted against. The Go
/// equivalent also carries `isSimpleIdentifier`, but nothing ever reads it from
/// the previous token, so it is not tracked here.
#[derive(Debug, Clone)]
pub(crate) struct LastValueToken<'a> {
    pub(crate) token_type: TokenType,
    pub(crate) value: Cow<'a, [u8]>,
    pub(crate) is_table_indicator: bool,
}
