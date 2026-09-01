//! Stress and misuse coverage for the C ABI, written to be run under
//! AddressSanitizer + LeakSanitizer (see `harness/sanitizers/run.sh`).
//!
//! The unit tests in `src/lib.rs` cover the happy path of each entry point once.
//! What a sanitizer run needs on top of that is volume and variety: many calls on
//! one warm handle across all four modes, inputs that force the buffer-growth and
//! owned-token paths, and handles created and destroyed in a loop so a per-handle
//! leak shows up as a leaked allocation rather than as flat RSS.

use sqllexer_ffi::{
    sqllexer_normalize, sqllexer_obfuscate, sqllexer_process, sqllexer_processor_free,
    sqllexer_processor_new, sqllexer_tokenize, Result as FfiResult, Slice, SliceList, TokenList,
    SQLLEXER_OK,
};

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

/// Inputs chosen for the code paths they hit rather than for their meaning:
/// growth of the output buffer, owned (unescaped) token values, invalid UTF-8,
/// unterminated constructs, deep nesting and long parameter lists.
fn inputs() -> Vec<Vec<u8>> {
    let mut inputs: Vec<Vec<u8>> = vec![
        b"".to_vec(),
        b"   ".to_vec(),
        b"SELECT 1".to_vec(),
        b"/* only a comment */".to_vec(),
        b"-- line comment".to_vec(),
        b"SELECT 'a''b', 'c' FROM t".to_vec(),
        b"SELECT \"quoted\", `back`, [bracket] FROM db.schema.tbl".to_vec(),
        b"SELECT $func$ SELECT 1 $func$".to_vec(),
        b"SELECT 'unterminated".to_vec(),
        b"/* unterminated".to_vec(),
        b"SELECT \"unterminated".to_vec(),
        b"CALL my_procedure(1, 2, 3)".to_vec(),
        b"INSERT INTO users (a, b) VALUES (1, 'x') RETURNING id;".to_vec(),
        b"SELECT @@version, :bind, $1, ? FROM dual".to_vec(),
        b"SELECT a -> 'b' ->> 'c' #> '{d}' FROM j".to_vec(),
        // Invalid UTF-8, which the library accepts and must carry through byte-exact.
        vec![0xff, 0xfe, 0x00, b'S', b'E', b'L', b'E', b'C', b'T', 0x80],
        b"SELECT '\xc3'".to_vec(),
        "sélect 😀 from tëst".as_bytes().to_vec(),
    ];

    // Pathological shapes: a long IN list, deep nesting, and one statement large
    // enough to force the handle's output buffer to grow well past its initial
    // capacity. Miri is roughly four orders of magnitude slower than native, so
    // the shapes stay but the sizes shrink to what it can finish.
    let mut in_list = b"SELECT * FROM t WHERE id IN (".to_vec();
    for i in 0..SCALE {
        if i > 0 {
            in_list.push(b',');
        }
        in_list.extend_from_slice(i.to_string().as_bytes());
    }
    in_list.extend_from_slice(b")");
    inputs.push(in_list);

    let mut nested = b"SELECT ".to_vec();
    nested.extend(std::iter::repeat(b'(').take(SCALE / 4));
    nested.extend_from_slice(b"1");
    nested.extend(std::iter::repeat(b')').take(SCALE / 4));
    inputs.push(nested);

    inputs.push(
        "SELECT a, b FROM tbl WHERE x = 'literal' AND y = 42; "
            .repeat(SCALE / 4)
            .into_bytes(),
    );

    inputs
}

const ALL_FLAGS: u32 = 0b1_1111_1111;
const DBMS_CODES: [u32; 7] = [0, 1, 2, 3, 4, 5, 99];

/// Size of the generated pathological inputs, and the number of iterations the
/// looping tests do. Native runs (including ASan) use the large value; Miri, an
/// interpreter, would take hours at that size and gains nothing from volume
/// because it checks every access rather than sampling.
const SCALE: usize = if cfg!(miri) { 20 } else { 2000 };
const ROUNDS: usize = if cfg!(miri) { 3 } else { 200 };

/// One handle, every mode, every input, every DBMS code: the shape of a worker in
/// the throughput harness, and the case where per-call state has to be reset
/// rather than accumulated.
#[test]
fn drives_one_handle_through_every_mode() {
    let processor = sqllexer_processor_new(ALL_FLAGS, ALL_FLAGS);
    assert!(!processor.is_null());

    for input in inputs() {
        for dbms in DBMS_CODES {
            let (ptr, len) = (input.as_ptr(), input.len());

            let mut result = FfiResult::EMPTY;
            assert_eq!(
                unsafe { sqllexer_process(processor, ptr, len, dbms, &mut result) },
                SQLLEXER_OK
            );
            // Reading everything back is the point: a bad descriptor is only a
            // memory error once someone dereferences it.
            let processed = slice_bytes(result.sql);
            for list in [
                result.tables,
                result.comments,
                result.commands,
                result.procedures,
            ] {
                assert!(list_bytes(list).iter().all(|v| v.len() <= input.len() + 64));
            }

            let mut normalized = FfiResult::EMPTY;
            assert_eq!(
                unsafe { sqllexer_normalize(processor, ptr, len, dbms, &mut normalized) },
                SQLLEXER_OK
            );
            let normalized_sql = slice_bytes(normalized.sql);

            let mut obfuscated = Slice::EMPTY;
            assert_eq!(
                unsafe { sqllexer_obfuscate(processor, ptr, len, dbms, &mut obfuscated) },
                SQLLEXER_OK
            );
            let obfuscated_sql = slice_bytes(obfuscated);

            let mut tokens = TokenList::EMPTY;
            assert_eq!(
                unsafe { sqllexer_tokenize(processor, ptr, len, dbms, &mut tokens) },
                SQLLEXER_OK
            );
            if !tokens.ptr.is_null() {
                let scanned = unsafe { std::slice::from_raw_parts(tokens.ptr, tokens.len) };
                let joined: Vec<u8> = scanned.iter().flat_map(|t| slice_bytes(t.value)).collect();
                // A prefix rather than the whole input: a NUL byte ends the scan,
                // exactly as it does in the Go lexer.
                assert!(
                    input.starts_with(&joined),
                    "token values must reconstruct a prefix of the input"
                );
            }

            // Each mode writes into the same handle buffers, so a mode that fails
            // to reset them shows up as output that keeps growing.
            for out in [&processed, &normalized_sql, &obfuscated_sql] {
                assert!(
                    out.len() <= input.len() * 2 + 64,
                    "output grew past the input: {} vs {}",
                    out.len(),
                    input.len()
                );
            }
        }
    }

    unsafe { sqllexer_processor_free(processor) };
}

/// Handles created and destroyed in a loop, each doing a little work. Under
/// LeakSanitizer, anything the handle owns and fails to release is reported here.
#[test]
fn creates_and_destroys_many_handles() {
    for i in 0..ROUNDS as u32 {
        let processor = sqllexer_processor_new(i % 128, i % 512);
        assert!(!processor.is_null());
        let sql = format!("SELECT c{i} FROM t{i} WHERE x = {i} AND s = 'lit{i}'");
        let mut result = FfiResult::EMPTY;
        assert_eq!(
            unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), i % 6, &mut result) },
            SQLLEXER_OK
        );
        assert!(!slice_bytes(result.sql).is_empty());
        let mut tokens = TokenList::EMPTY;
        assert_eq!(
            unsafe { sqllexer_tokenize(processor, sql.as_ptr(), sql.len(), i % 6, &mut tokens) },
            SQLLEXER_OK
        );
        unsafe { sqllexer_processor_free(processor) };
    }
}

/// A handle is `Send` but not `Sync`: one handle per thread is the supported
/// model, and this is what the cgo binding does with one processor per worker.
#[test]
fn one_handle_per_thread_is_independent() {
    let threads: Vec<_> = (0..4u32)
        .map(|id| {
            std::thread::spawn(move || {
                let processor = sqllexer_processor_new(ALL_FLAGS, ALL_FLAGS);
                // Table names avoid digits: the handle runs with replace_digits on.
                let table = "t".repeat(id as usize + 1);
                for round in 0..ROUNDS as u32 {
                    let sql = format!("SELECT * FROM {table} WHERE id = {round}");
                    let mut result = FfiResult::EMPTY;
                    assert_eq!(
                        unsafe {
                            sqllexer_process(processor, sql.as_ptr(), sql.len(), 2, &mut result)
                        },
                        SQLLEXER_OK
                    );
                    assert_eq!(list_bytes(result.tables), [table.clone().into_bytes()]);
                }
                unsafe { sqllexer_processor_free(processor) };
            })
        })
        .collect();
    for thread in threads {
        thread.join().unwrap();
    }
}

/// Deliberate use-after-free, ignored by default: it exists so a sanitizer run can
/// show it *does* catch a violation of the documented lifetime rule, rather than
/// reporting "clean" without ever being challenged. Expected to abort with a
/// heap-use-after-free under ASan; do not run it otherwise.
#[test]
#[ignore = "deliberate use-after-free; only meaningful as an ASan positive control"]
fn reading_a_result_after_free_is_a_use_after_free() {
    let processor = sqllexer_processor_new(0, 1);
    let sql = b"SELECT * FROM users WHERE id = 1";
    let mut result = FfiResult::EMPTY;
    assert_eq!(
        unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 0, &mut result) },
        SQLLEXER_OK
    );
    unsafe { sqllexer_processor_free(processor) };
    // The descriptors still point into the freed handle's buffers.
    let after = slice_bytes(result.sql);
    println!("read {} bytes after free", after.len());
}

/// Aliasing probe, ignored by default: hands a previous result's buffer back in
/// as the next call's input. The entry points take the handle as `&mut` and the
/// input as `&[u8]`, so an input that points into the handle's own buffers is
/// undefined behavior even though nothing is freed and nothing is read out of
/// bounds. Only Miri sees it; ASan and valgrind cannot. The cgo binding never
/// does this — it copies every result into Go memory — but a C caller could.
#[test]
#[ignore = "deliberate aliasing UB; only meaningful under Miri"]
fn feeding_a_result_back_as_input_aliases_handle_memory() {
    let processor = sqllexer_processor_new(0, 1);
    let sql = b"SELECT * FROM aliased WHERE id = 1";
    let mut result = FfiResult::EMPTY;
    assert_eq!(
        unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 0, &mut result) },
        SQLLEXER_OK
    );
    let mut again = FfiResult::EMPTY;
    let status =
        unsafe { sqllexer_process(processor, result.sql.ptr, result.sql.len, 0, &mut again) };
    assert_eq!(status, SQLLEXER_OK);
    println!("re-processed {} bytes", again.sql.len);
    unsafe { sqllexer_processor_free(processor) };
}

/// Deliberate leak, ignored by default: the LeakSanitizer counterpart of the
/// use-after-free control above. A run that reports nothing here is a run whose
/// leak detection was not actually on.
#[test]
#[ignore = "deliberate leak; only meaningful as a LeakSanitizer positive control"]
fn leaking_a_handle_is_reported() {
    let processor = sqllexer_processor_new(0, 1);
    let sql = b"SELECT * FROM leaked";
    let mut result = FfiResult::EMPTY;
    assert_eq!(
        unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 0, &mut result) },
        SQLLEXER_OK
    );
    // No sqllexer_processor_free: the handle and its buffers stay allocated.
    let _ = processor;
}

/// The `panic-probe` feature exports an entry point that panics inside the same
/// `catch_unwind` wrapper the real ones use. This checks containment in-process;
/// `harness/rustffi` checks the same thing across cgo, which is where an escaping
/// unwind would actually be fatal.
#[cfg(feature = "panic-probe")]
#[test]
fn a_panic_is_contained_and_leaves_the_handle_usable() {
    use sqllexer_ffi::{sqllexer_panic_probe, SQLLEXER_ERR_PANIC};

    let processor = sqllexer_processor_new(0, 1);
    let mut out = Slice::EMPTY;
    let previous = std::panic::take_hook();
    std::panic::set_hook(Box::new(|_| {}));
    let status = unsafe { sqllexer_panic_probe(processor, &mut out) };
    std::panic::set_hook(previous);

    assert_eq!(status, SQLLEXER_ERR_PANIC);
    assert!(
        out.ptr.is_null(),
        "no result may be published after a panic"
    );

    let sql = b"SELECT * FROM after_panic";
    let mut result = FfiResult::EMPTY;
    assert_eq!(
        unsafe { sqllexer_process(processor, sql.as_ptr(), sql.len(), 0, &mut result) },
        SQLLEXER_OK
    );
    assert_eq!(slice_bytes(result.sql), b"SELECT * FROM after_panic");
    unsafe { sqllexer_processor_free(processor) };
}
