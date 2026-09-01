//! Case-insensitive keyword trie.
//!
//! Laid out as a flat arena of nodes indexed by `u16` rather than as a tree of
//! boxed nodes: the walk is one array index per character and the whole table fits
//! in a few cache lines, which matters because every identifier in every statement
//! walks it.

use std::sync::OnceLock;

use crate::token::TokenType;

const NONE: u16 = 0; // index 0 is the root, so it can never be a child

pub(crate) struct Node {
    children: [u16; 27], // A-Z, then underscore
    pub(crate) is_end: bool,
    pub(crate) token_type: TokenType,
    pub(crate) is_table_indicator: bool,
}

impl Node {
    fn empty() -> Self {
        Node {
            children: [NONE; 27],
            is_end: false,
            token_type: TokenType::Error,
            is_table_indicator: false,
        }
    }
}

pub(crate) struct Trie {
    nodes: Vec<Node>,
}

impl Trie {
    #[inline]
    pub(crate) fn root(&self) -> u16 {
        0
    }

    #[inline]
    pub(crate) fn node(&self, idx: u16) -> &Node {
        &self.nodes[idx as usize]
    }

    /// Follows one character, returning the child index if the path continues.
    #[inline]
    pub(crate) fn child(&self, idx: u16, slot: usize) -> Option<u16> {
        match self.nodes[idx as usize].children[slot] {
            NONE => None,
            next => Some(next),
        }
    }

    fn insert(&mut self, word: &str, token_type: TokenType, is_table_indicator: bool) {
        let mut idx = 0usize;
        for ch in word.bytes() {
            let Some(slot) = trie_index(ch as u32) else {
                continue;
            };
            let next = self.nodes[idx].children[slot];
            idx = if next == NONE {
                self.nodes.push(Node::empty());
                let new = (self.nodes.len() - 1) as u16;
                self.nodes[idx].children[slot] = new;
                new as usize
            } else {
                next as usize
            };
        }
        let node = &mut self.nodes[idx];
        node.is_end = true;
        node.token_type = token_type;
        node.is_table_indicator = is_table_indicator;
    }
}

/// Slot for an uppercase character: 0-25 for A-Z, 26 for underscore.
#[inline]
pub(crate) fn trie_index(ch: u32) -> Option<usize> {
    if (b'A' as u32..=b'Z' as u32).contains(&ch) {
        Some((ch - b'A' as u32) as usize)
    } else if ch == b'_' as u32 {
        Some(26)
    } else {
        None
    }
}

pub(crate) fn keyword_trie() -> &'static Trie {
    static TRIE: OnceLock<Trie> = OnceLock::new();
    TRIE.get_or_init(build)
}

fn build() -> Trie {
    let mut trie = Trie {
        nodes: vec![Node::empty()],
    };
    // Insertion order matters: later groups overwrite the token type and table
    // indicator flag of words that appear more than once (JOIN, FROM, EXISTS, ...).
    for (words, token_type, is_table_indicator) in [
        (COMMANDS.as_slice(), TokenType::Command, false),
        (KEYWORDS.as_slice(), TokenType::Keyword, false),
        (
            TABLE_INDICATOR_COMMANDS.as_slice(),
            TokenType::Command,
            true,
        ),
        (
            TABLE_INDICATOR_KEYWORDS.as_slice(),
            TokenType::Keyword,
            true,
        ),
        (BOOLEAN_VALUES.as_slice(), TokenType::Boolean, false),
        (NULL_VALUES.as_slice(), TokenType::Null, false),
        (PROCEDURE_NAMES.as_slice(), TokenType::ProcIndicator, false),
        (CTES.as_slice(), TokenType::CteIndicator, false),
        (ALIAS.as_slice(), TokenType::AliasIndicator, false),
    ] {
        for word in words {
            trie.insert(word, token_type, is_table_indicator);
        }
    }
    trie
}

static COMMANDS: [&str; 21] = [
    "SELECT",
    "INSERT",
    "UPDATE",
    "DELETE",
    "CREATE",
    "ALTER",
    "DROP",
    "JOIN",
    "GRANT",
    "REVOKE",
    "COMMIT",
    "BEGIN",
    "TRUNCATE",
    "MERGE",
    "EXECUTE",
    "EXEC",
    "EXPLAIN",
    "STRAIGHT_JOIN",
    "USE",
    "CLONE",
    "VACUUM",
];

static TABLE_INDICATOR_COMMANDS: [&str; 4] = ["JOIN", "UPDATE", "STRAIGHT_JOIN", "CLONE"];

static TABLE_INDICATOR_KEYWORDS: [&str; 5] = ["FROM", "INTO", "TABLE", "EXISTS", "ONLY"];

static KEYWORDS: [&str; 78] = [
    "ADD",
    "ALL",
    "AND",
    "ANY",
    "ASC",
    "BETWEEN",
    "BY",
    "CASE",
    "CHECK",
    "COLUMN",
    "CONSTRAINT",
    "DATABASE",
    "DECLARE",
    "DEFAULT",
    "DESC",
    "DISTINCT",
    "ELSE",
    "END",
    "ESCAPE",
    "EXISTS",
    "FOREIGN",
    "FROM",
    "GROUP",
    "HAVING",
    "IN",
    "INDEX",
    "INNER",
    "INTO",
    "IS",
    "KEY",
    "LATERAL",
    "LEFT",
    "LIKE",
    "LIMIT",
    "NOT",
    "ON",
    "OR",
    "ORDER",
    "OUT",
    "OUTER",
    "PRIMARY",
    "PROCEDURE",
    "REPLACE",
    "RETURNS",
    "RIGHT",
    "ROLLBACK",
    "ROWNUM",
    "SET",
    "SOME",
    "TABLE",
    "TOP",
    "UNION",
    "UNIQUE",
    "VALUES",
    "VIEW",
    "WHERE",
    "CUBE",
    "ROLLUP",
    "LITERAL",
    "WINDOW",
    "ANALYZE",
    "ILIKE",
    "USING",
    "ASSERTION",
    "DOMAIN",
    "CLUSTER",
    "COPY",
    "PLPGSQL",
    "TRIGGER",
    "TEMPORARY",
    "UNLOGGED",
    "RECURSIVE",
    "RETURNING",
    "OFFSET",
    "OF",
    "SKIP",
    "IF",
    "ONLY",
];

static BOOLEAN_VALUES: [&str; 2] = ["TRUE", "FALSE"];
static NULL_VALUES: [&str; 1] = ["NULL"];
static PROCEDURE_NAMES: [&str; 2] = ["PROCEDURE", "PROC"];
static CTES: [&str; 1] = ["WITH"];
static ALIAS: [&str; 1] = ["AS"];
