//! C ABI for the Rust sqllexer, consumed from Go through cgo.
//!
//! The object model is a reusable handle: a caller creates one `Processor` per
//! configuration and reuses it for every statement, so configuration parsing and
//! buffer allocation stay out of the per-call path. That is the model a tracer
//! would use, and it is what the cgo benchmarks measure.
//!
//! Ownership rules, which the Go side depends on:
//!
//! - A handle is created by [`sqllexer_processor_new`] and must be released with
//!   [`sqllexer_processor_free`]. Nothing else allocates across the boundary.
//! - Every slice in a [`Result`] points into buffers owned by the handle and stays
//!   valid until the next call on that handle or until it is freed. Callers copy
//!   what they need before calling again.
//! - A handle is `Send` but not `Sync`: it holds the buffers it writes into, so
//!   concurrent callers need one handle each.
//!
//! No panic may unwind into the host process, so every entry point is wrapped in
//! `catch_unwind` and reports failure as a status code. This mirrors the Go
//! package, whose `Normalize` recovers panics into an error rather than crashing.

use std::panic::{catch_unwind, AssertUnwindSafe};
use std::ptr;

use sqllexer::{
    Dbms, Lexer, Normalizer, NormalizerConfig, Obfuscator, ObfuscatorConfig, StatementMetadata,
    TokenType,
};

/// Status codes returned by every entry point.
pub const SQLLEXER_OK: i32 = 0;
/// A required pointer argument was null.
pub const SQLLEXER_ERR_NULL: i32 = -1;
/// A panic was caught at the boundary; the handle is left usable but the result
/// is undefined and must be discarded.
pub const SQLLEXER_ERR_PANIC: i32 = -2;

// Obfuscator option bits, in the order the options appear in obfuscator.go.
const OBF_DOLLAR_QUOTED_FUNC: u32 = 1 << 0;
const OBF_REPLACE_DIGITS: u32 = 1 << 1;
const OBF_REPLACE_POSITIONAL_PARAMETER: u32 = 1 << 2;
const OBF_REPLACE_BOOLEAN: u32 = 1 << 3;
const OBF_REPLACE_NULL: u32 = 1 << 4;
const OBF_KEEP_JSON_PATH: u32 = 1 << 5;
const OBF_REPLACE_BIND_PARAMETER: u32 = 1 << 6;

// Normalizer option bits, in the order the options appear in normalizer.go.
const NORM_COLLECT_TABLES: u32 = 1 << 0;
const NORM_COLLECT_COMMANDS: u32 = 1 << 1;
const NORM_COLLECT_COMMENTS: u32 = 1 << 2;
const NORM_COLLECT_PROCEDURE: u32 = 1 << 3;
const NORM_KEEP_SQL_ALIAS: u32 = 1 << 4;
const NORM_UPPERCASE_KEYWORDS: u32 = 1 << 5;
const NORM_REMOVE_SPACE_BETWEEN_PARENTHESES: u32 = 1 << 6;
const NORM_KEEP_TRAILING_SEMICOLON: u32 = 1 << 7;
const NORM_KEEP_IDENTIFIER_QUOTATION: u32 = 1 << 8;

/// A borrowed byte slice. The bytes are not NUL-terminated and may be invalid
/// UTF-8, because the library accepts arbitrary input.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Slice {
    pub ptr: *const u8,
    pub len: usize,
}

impl Slice {
    const EMPTY: Slice = Slice {
        ptr: ptr::null(),
        len: 0,
    };

    fn of(bytes: &[u8]) -> Slice {
        Slice {
            ptr: bytes.as_ptr(),
            len: bytes.len(),
        }
    }
}

/// A borrowed list of slices.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct SliceList {
    pub ptr: *const Slice,
    pub len: usize,
}

impl SliceList {
    const EMPTY: SliceList = SliceList {
        ptr: ptr::null(),
        len: 0,
    };

    fn of(slices: &[Slice]) -> SliceList {
        SliceList {
            ptr: slices.as_ptr(),
            len: slices.len(),
        }
    }
}

/// The result of one call, borrowing the handle's buffers.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Result {
    pub sql: Slice,
    pub metadata_size: usize,
    pub tables: SliceList,
    pub comments: SliceList,
    pub commands: SliceList,
    pub procedures: SliceList,
}

impl Result {
    const EMPTY: Result = Result {
        sql: Slice::EMPTY,
        metadata_size: 0,
        tables: SliceList::EMPTY,
        comments: SliceList::EMPTY,
        commands: SliceList::EMPTY,
        procedures: SliceList::EMPTY,
    };
}

/// The result of one call, packed so a caller that has to own its output can
/// take it in one copy instead of one per value.
///
/// `values` is every metadata value concatenated in list order — tables,
/// comments, commands, procedures — and `lens` has one entry per value in that
/// same order, so a caller reconstructs the lists by walking `lens` with a
/// running offset. Splitting the SQL out of `values` keeps the normalized
/// output where it already is (the handle's own buffer) instead of copying it
/// twice.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct PackedResult {
    pub sql: Slice,
    pub values: Slice,
    pub lens: *const usize,
    /// Number of values belonging to tables, comments, commands, procedures.
    pub counts: [usize; 4],
    pub metadata_size: usize,
}

impl PackedResult {
    const EMPTY: PackedResult = PackedResult {
        sql: Slice::EMPTY,
        values: Slice::EMPTY,
        lens: ptr::null(),
        counts: [0; 4],
        metadata_size: 0,
    };
}

/// The result of one call for callers that need the normalized SQL and the
/// metadata size, but not the metadata values themselves.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct SizeResult {
    pub sql: Slice,
    pub metadata_size: usize,
}

impl SizeResult {
    const EMPTY: SizeResult = SizeResult {
        sql: Slice::EMPTY,
        metadata_size: 0,
    };
}

/// One token, borrowing the input the caller passed in.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Token {
    /// The numeric `TokenType`, identical to the Go constant of the same name.
    pub token_type: u32,
    pub value: Slice,
}

/// A borrowed list of tokens.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct TokenList {
    pub ptr: *const Token,
    pub len: usize,
}

impl TokenList {
    const EMPTY: TokenList = TokenList {
        ptr: ptr::null(),
        len: 0,
    };
}

/// A reusable handle covering the whole package surface.
pub struct Processor {
    obfuscator: Obfuscator,
    normalizer: Normalizer,
    sql: Vec<u8>,
    metadata: StatementMetadata,
    /// Slice descriptors handed back to the caller, kept alive by the handle and
    /// rebuilt in place on every call so no allocation is needed per statement.
    views: Vec<Slice>,
    /// Metadata values concatenated for [`PackedResult`], and their lengths.
    /// Both are handle-owned and reused, so packing costs a memcpy of the
    /// metadata bytes and no allocation once the handle is warm.
    pack: Vec<u8>,
    lens: Vec<usize>,
    tokens: Vec<Token>,
    /// Backing store for token values that the lexer had to materialize (the
    /// unescaping paths); the rest borrow the caller's input directly.
    token_values: Vec<u8>,
}

fn obfuscator_config(flags: u32) -> ObfuscatorConfig {
    ObfuscatorConfig {
        dollar_quoted_func: flags & OBF_DOLLAR_QUOTED_FUNC != 0,
        replace_digits: flags & OBF_REPLACE_DIGITS != 0,
        replace_positional_parameter: flags & OBF_REPLACE_POSITIONAL_PARAMETER != 0,
        replace_boolean: flags & OBF_REPLACE_BOOLEAN != 0,
        replace_null: flags & OBF_REPLACE_NULL != 0,
        keep_json_path: flags & OBF_KEEP_JSON_PATH != 0,
        replace_bind_parameter: flags & OBF_REPLACE_BIND_PARAMETER != 0,
    }
}

fn normalizer_config(flags: u32) -> NormalizerConfig {
    NormalizerConfig {
        collect_tables: flags & NORM_COLLECT_TABLES != 0,
        collect_commands: flags & NORM_COLLECT_COMMANDS != 0,
        collect_comments: flags & NORM_COLLECT_COMMENTS != 0,
        collect_procedure: flags & NORM_COLLECT_PROCEDURE != 0,
        keep_sql_alias: flags & NORM_KEEP_SQL_ALIAS != 0,
        uppercase_keywords: flags & NORM_UPPERCASE_KEYWORDS != 0,
        remove_space_between_parentheses: flags & NORM_REMOVE_SPACE_BETWEEN_PARENTHESES != 0,
        keep_trailing_semicolon: flags & NORM_KEEP_TRAILING_SEMICOLON != 0,
        keep_identifier_quotation: flags & NORM_KEEP_IDENTIFIER_QUOTATION != 0,
    }
}

fn dbms_from_code(code: u32) -> Dbms {
    match code {
        1 => Dbms::SqlServer,
        2 => Dbms::Postgres,
        3 => Dbms::MySql,
        4 => Dbms::Oracle,
        5 => Dbms::Snowflake,
        _ => Dbms::None,
    }
}

/// Creates a handle for one configuration. Never returns null unless the
/// allocation itself panicked.
///
/// # Safety
/// The returned pointer must be released with [`sqllexer_processor_free`].
#[no_mangle]
pub extern "C" fn sqllexer_processor_new(
    obfuscator_flags: u32,
    normalizer_flags: u32,
) -> *mut Processor {
    catch_unwind(|| {
        Box::into_raw(Box::new(Processor {
            obfuscator: Obfuscator::new(obfuscator_config(obfuscator_flags)),
            normalizer: Normalizer::new(normalizer_config(normalizer_flags)),
            sql: Vec::with_capacity(1024),
            metadata: StatementMetadata::default(),
            views: Vec::new(),
            pack: Vec::new(),
            lens: Vec::new(),
            tokens: Vec::new(),
            token_values: Vec::new(),
        }))
    })
    .unwrap_or(ptr::null_mut())
}

/// Releases a handle. Null is accepted and ignored.
///
/// # Safety
/// `processor` must have come from [`sqllexer_processor_new`] and must not be
/// used afterwards.
#[no_mangle]
pub unsafe extern "C" fn sqllexer_processor_free(processor: *mut Processor) {
    if processor.is_null() {
        return;
    }
    let processor = Box::from_raw(processor);
    let _ = catch_unwind(AssertUnwindSafe(move || drop(processor)));
}

/// Obfuscates and normalizes one statement.
///
/// On success `out` is filled with slices borrowing the handle's buffers, valid
/// until the next call on this handle.
///
/// # Safety
/// `processor` must be a live handle, `out` must be a valid `Result`, and
/// `sql`/`sql_len` must describe a readable range (a null `sql` is allowed only
/// when `sql_len` is zero).
#[no_mangle]
pub unsafe extern "C" fn sqllexer_process(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut Result,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = Result::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.normalizer.obfuscate_and_normalize_into(
            input,
            &processor.obfuscator,
            dbms_from_code(dbms),
            &mut processor.sql,
            &mut processor.metadata,
        );
        build_result(processor)
    }));
    finish(result, out)
}

/// Publishes the handle's output buffers as borrowed slices. Descriptors for all
/// four metadata lists live in one buffer so a call needs at most one growth, and
/// none once the handle is warm.
fn build_result(processor: &mut Processor) -> Result {
    let metadata = &processor.metadata;
    let lists = [
        metadata.tables.values(),
        metadata.comments.values(),
        metadata.commands.values(),
        metadata.procedures.values(),
    ];
    processor.views.clear();
    for list in lists {
        processor.views.extend(list.iter().map(|v| Slice::of(v)));
    }

    let mut offset = 0;
    let mut next = |len: usize| {
        let list = SliceList::of(&processor.views[offset..offset + len]);
        offset += len;
        list
    };
    Result {
        sql: Slice::of(&processor.sql),
        metadata_size: metadata.size,
        tables: next(lists[0].len()),
        comments: next(lists[1].len()),
        commands: next(lists[2].len()),
        procedures: next(lists[3].len()),
    }
}

/// Obfuscates and normalizes one statement, reporting the result packed.
///
/// Same computation as [`sqllexer_process`]; only the shape of `out` differs.
/// It exists for callers that copy the result into their own memory, which is
/// one copy of `sql` plus one of `values` instead of one copy per value.
///
/// # Safety
/// As [`sqllexer_process`], except that `out` is a [`PackedResult`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_process_packed(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut PackedResult,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = PackedResult::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.normalizer.obfuscate_and_normalize_into(
            input,
            &processor.obfuscator,
            dbms_from_code(dbms),
            &mut processor.sql,
            &mut processor.metadata,
        );
        build_packed(processor)
    }));
    finish(result, out)
}

/// Normalizes one statement, reporting the result packed.
///
/// # Safety
/// As [`sqllexer_process_packed`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_normalize_packed(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut PackedResult,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = PackedResult::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.normalizer.normalize_into(
            input,
            dbms_from_code(dbms),
            &mut processor.sql,
            &mut processor.metadata,
        );
        build_packed(processor)
    }));
    finish(result, out)
}

/// Obfuscates and normalizes one statement, reporting only the normalized SQL
/// and the metadata size.
///
/// The metadata is still collected — `size` is defined by it — but nothing is
/// published for the values, so a caller that only reports the size never
/// materializes the lists.
///
/// # Safety
/// As [`sqllexer_process`], except that `out` is a [`SizeResult`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_process_size(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut SizeResult,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = SizeResult::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.normalizer.obfuscate_and_normalize_into(
            input,
            &processor.obfuscator,
            dbms_from_code(dbms),
            &mut processor.sql,
            &mut processor.metadata,
        );
        SizeResult {
            sql: Slice::of(&processor.sql),
            metadata_size: processor.metadata.size,
        }
    }));
    finish(result, out)
}

/// Concatenates the metadata values into the handle's pack buffer and describes
/// them with one length per value. The buffers are reused across calls, so this
/// is a memcpy of the metadata bytes and no allocation once the handle is warm.
fn build_packed(processor: &mut Processor) -> PackedResult {
    let Processor {
        sql,
        metadata,
        pack,
        lens,
        ..
    } = processor;
    let lists = [
        metadata.tables.values(),
        metadata.comments.values(),
        metadata.commands.values(),
        metadata.procedures.values(),
    ];
    pack.clear();
    lens.clear();
    let mut counts = [0usize; 4];
    for (i, list) in lists.iter().enumerate() {
        counts[i] = list.len();
        for value in *list {
            pack.extend_from_slice(value);
            lens.push(value.len());
        }
    }
    PackedResult {
        sql: Slice::of(sql),
        values: Slice::of(pack),
        lens: lens.as_ptr(),
        counts,
        metadata_size: metadata.size,
    }
}

/// Obfuscates one statement, without normalizing it.
///
/// # Safety
/// As [`sqllexer_process`], except that `out` is a single [`Slice`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_obfuscate(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut Slice,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = Slice::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        // obfuscate_into appends, so the handle's buffer has to be reset first.
        processor.sql.clear();
        processor
            .obfuscator
            .obfuscate_into(input, dbms_from_code(dbms), &mut processor.sql);
        Slice::of(&processor.sql)
    }));
    finish(result, out)
}

/// Normalizes one statement, without obfuscating it first.
///
/// # Safety
/// As [`sqllexer_process`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_normalize(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut Result,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = Result::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };
    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.normalizer.normalize_into(
            input,
            dbms_from_code(dbms),
            &mut processor.sql,
            &mut processor.metadata,
        );
        build_result(processor)
    }));
    finish(result, out)
}

/// Scans one statement into tokens, the equivalent of driving `Lexer.Scan` to EOF.
///
/// The returned values borrow the handle's buffers *and* the caller's input, so
/// `sql` must stay alive and unmodified for as long as the result is read.
///
/// # Safety
/// As [`sqllexer_process`], except that `out` is a [`TokenList`].
#[no_mangle]
pub unsafe extern "C" fn sqllexer_tokenize(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
    dbms: u32,
    out: *mut TokenList,
) -> i32 {
    if out.is_null() {
        return SQLLEXER_ERR_NULL;
    }
    *out = TokenList::EMPTY;
    let Some((processor, input)) = args(processor, sql, sql_len) else {
        return SQLLEXER_ERR_NULL;
    };

    let result = catch_unwind(AssertUnwindSafe(|| {
        processor.tokens.clear();
        processor.token_values.clear();
        // Owned values are collected into one buffer first: pushing to it while
        // slices point into it would dangle on reallocation, so the descriptors
        // are patched up in a second pass once the buffer has stopped moving.
        let mut owned_ranges = Vec::new();
        let mut lexer = Lexer::new(input, dbms_from_code(dbms));
        loop {
            let token = lexer.scan();
            if token.token_type == TokenType::Eof {
                break;
            }
            let value = match token.value {
                std::borrow::Cow::Borrowed(bytes) => Slice::of(bytes),
                std::borrow::Cow::Owned(bytes) => {
                    let start = processor.token_values.len();
                    processor.token_values.extend_from_slice(&bytes);
                    owned_ranges.push((processor.tokens.len(), start, bytes.len()));
                    Slice::EMPTY
                }
            };
            processor.tokens.push(Token {
                token_type: token.token_type as u32,
                value,
            });
        }
        for (index, start, len) in owned_ranges {
            processor.tokens[index].value = Slice::of(&processor.token_values[start..start + len]);
        }
        TokenList {
            ptr: processor.tokens.as_ptr(),
            len: processor.tokens.len(),
        }
    }));
    finish(result, out)
}

/// Shared argument validation for the entry points that take a statement.
unsafe fn args<'a>(
    processor: *mut Processor,
    sql: *const u8,
    sql_len: usize,
) -> Option<(&'a mut Processor, &'a [u8])> {
    if processor.is_null() || (sql.is_null() && sql_len != 0) {
        return None;
    }
    let input = if sql_len == 0 {
        &[][..]
    } else {
        std::slice::from_raw_parts(sql, sql_len)
    };
    Some((&mut *processor, input))
}

unsafe fn finish<T>(result: std::thread::Result<T>, out: *mut T) -> i32 {
    match result {
        Ok(value) => {
            *out = value;
            SQLLEXER_OK
        }
        Err(_) => SQLLEXER_ERR_PANIC,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn slice_bytes(slice: Slice) -> Vec<u8> {
        if slice.ptr.is_null() {
            return Vec::new();
        }
        unsafe { std::slice::from_raw_parts(slice.ptr, slice.len) }.to_vec()
    }

    fn list_bytes(list: SliceList) -> Vec<Vec<u8>> {
        if list.ptr.is_null() {
            return Vec::new();
        }
        unsafe { std::slice::from_raw_parts(list.ptr, list.len) }
            .iter()
            .map(|s| slice_bytes(*s))
            .collect()
    }

    #[test]
    fn round_trips_a_statement_through_the_boundary() {
        let processor = sqllexer_processor_new(
            OBF_REPLACE_DIGITS | OBF_REPLACE_BOOLEAN | OBF_REPLACE_NULL,
            NORM_COLLECT_TABLES | NORM_COLLECT_COMMANDS | NORM_COLLECT_COMMENTS,
        );
        assert!(!processor.is_null());
        let sql = b"/* c */ SELECT * FROM users WHERE id = 42";
        let mut result = Result::EMPTY;
        let status =
            unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 2, &mut result) };
        assert_eq!(status, SQLLEXER_OK);
        assert_eq!(slice_bytes(result.sql), b"SELECT * FROM users WHERE id = ?");
        assert_eq!(list_bytes(result.tables), [b"users".to_vec()]);
        assert_eq!(list_bytes(result.commands), [b"SELECT".to_vec()]);
        assert_eq!(list_bytes(result.comments), [b"/* c */".to_vec()]);
        assert!(list_bytes(result.procedures).is_empty());
        assert_eq!(result.metadata_size, 18);
        unsafe { sqllexer_processor_free(processor) };
    }

    #[test]
    fn reuses_a_handle_without_leaking_state() {
        let processor = sqllexer_processor_new(OBF_REPLACE_DIGITS, NORM_COLLECT_TABLES);
        let mut result = Result::EMPTY;
        for (sql, expected) in [
            (&b"SELECT * FROM a"[..], &b"a"[..]),
            (&b"SELECT * FROM bb"[..], &b"bb"[..]),
            (&b"SELECT 1"[..], &b""[..]),
        ] {
            let status =
                unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 0, &mut result) };
            assert_eq!(status, SQLLEXER_OK);
            let tables = list_bytes(result.tables);
            if expected.is_empty() {
                assert!(tables.is_empty());
            } else {
                assert_eq!(tables, [expected.to_vec()]);
            }
        }
        unsafe { sqllexer_processor_free(processor) };
    }

    #[test]
    fn exposes_obfuscate_and_normalize_separately() {
        let processor = sqllexer_processor_new(OBF_REPLACE_DIGITS, NORM_COLLECT_TABLES);
        let sql = b"SELECT  *  FROM users1 WHERE id = 42";

        let mut obfuscated = Slice::EMPTY;
        assert_eq!(
            unsafe { sqllexer_obfuscate(processor, sql.as_ptr(), sql.len(), 0, &mut obfuscated) },
            SQLLEXER_OK
        );
        assert_eq!(
            slice_bytes(obfuscated),
            b"SELECT  *  FROM users? WHERE id = ?"
        );
        // Again on the same handle: the buffer must be reset, not appended to.
        assert_eq!(
            unsafe { sqllexer_obfuscate(processor, sql.as_ptr(), sql.len(), 0, &mut obfuscated) },
            SQLLEXER_OK
        );
        assert_eq!(
            slice_bytes(obfuscated),
            b"SELECT  *  FROM users? WHERE id = ?"
        );

        let mut normalized = Result::EMPTY;
        assert_eq!(
            unsafe { sqllexer_normalize(processor, sql.as_ptr(), sql.len(), 0, &mut normalized) },
            SQLLEXER_OK
        );
        assert_eq!(
            slice_bytes(normalized.sql),
            b"SELECT * FROM users1 WHERE id = 42"
        );
        assert_eq!(list_bytes(normalized.tables), [b"users1".to_vec()]);
        unsafe { sqllexer_processor_free(processor) };
    }

    #[test]
    fn tokenizes_including_values_the_lexer_had_to_materialize() {
        let processor = sqllexer_processor_new(0, 0);
        // The escaped quote forces an owned token value, which must survive being
        // appended to the same buffer as its neighbours.
        let sql = b"SELECT 'a''b', 'c' FROM t";
        let mut tokens = TokenList::EMPTY;
        assert_eq!(
            unsafe { sqllexer_tokenize(processor, sql.as_ptr(), sql.len(), 0, &mut tokens) },
            SQLLEXER_OK
        );
        let scanned = unsafe { std::slice::from_raw_parts(tokens.ptr, tokens.len) };
        let values: Vec<Vec<u8>> = scanned.iter().map(|t| slice_bytes(t.value)).collect();
        let joined = values.concat();
        assert_eq!(joined, sql, "token values must reconstruct the input");
        assert_eq!(scanned[0].token_type, TokenType::Command as u32);

        let mut empty = TokenList::EMPTY;
        assert_eq!(
            unsafe { sqllexer_tokenize(processor, ptr::null(), 0, 0, &mut empty) },
            SQLLEXER_OK
        );
        assert_eq!(empty.len, 0);
        unsafe { sqllexer_processor_free(processor) };
    }

    #[test]
    fn rejects_null_arguments_instead_of_dereferencing_them() {
        let mut result = Result::EMPTY;
        assert_eq!(
            unsafe { sqllexer_process(ptr::null_mut(), b"x".as_ptr(), 1, 0, &mut result) },
            SQLLEXER_ERR_NULL
        );
        let processor = sqllexer_processor_new(0, 0);
        assert_eq!(
            unsafe { sqllexer_process(processor, ptr::null(), 1, 0, &mut result) },
            SQLLEXER_ERR_NULL
        );
        assert_eq!(
            unsafe { sqllexer_process(processor, ptr::null(), 0, 0, &mut result) },
            SQLLEXER_OK
        );
        assert_eq!(
            unsafe { sqllexer_process(processor, b"x".as_ptr(), 1, 0, ptr::null_mut()) },
            SQLLEXER_ERR_NULL
        );
        unsafe { sqllexer_processor_free(processor) };
        unsafe { sqllexer_processor_free(ptr::null_mut()) };
    }
}
