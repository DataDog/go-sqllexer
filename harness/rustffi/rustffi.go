//go:build rustffi

// Package rustffi is the cgo binding for the Rust core.
//
// It is the integration model the migration assumes: one reusable handle per
// configuration, created once and used for every statement, so the per-call cost
// is the cgo transition plus scanning rather than object construction. It is also
// just another implementation of the harness protocol, which lets the differ
// validate the FFI layer with the same corpora that validated the pure Rust core.
//
// Everything here is behind the `rustffi` build tag so the module still builds
// and tests without a Rust toolchain. Build the static library first:
//
//	cd rust && cargo build --release -p sqllexer-ffi
//	go test -tags rustffi ./harness/...
package rustffi

/*
// The archive is named explicitly rather than with -l so the cdylib built
// alongside it is never picked up, which would make the test binary depend on a
// shared library at runtime.
#cgo LDFLAGS: ${SRCDIR}/../../rust/target/release/libsqllexer_ffi.a -lm -ldl -pthread
#include <stdint.h>
#include <stdlib.h>

typedef struct { const uint8_t* ptr; size_t len; } sqllexer_slice;
typedef struct { const sqllexer_slice* ptr; size_t len; } sqllexer_slice_list;
typedef struct {
	sqllexer_slice sql;
	size_t metadata_size;
	sqllexer_slice_list tables;
	sqllexer_slice_list comments;
	sqllexer_slice_list commands;
	sqllexer_slice_list procedures;
} sqllexer_result;

typedef struct {
	sqllexer_slice sql;
	sqllexer_slice values;
	const size_t* lens;
	size_t counts[4];
	size_t metadata_size;
} sqllexer_packed;

typedef struct { sqllexer_slice sql; size_t metadata_size; } sqllexer_size_result;

typedef struct { uint32_t token_type; sqllexer_slice value; } sqllexer_token;
typedef struct { const sqllexer_token* ptr; size_t len; } sqllexer_token_list;

void* sqllexer_processor_new(uint32_t obfuscator_flags, uint32_t normalizer_flags);
void sqllexer_processor_free(void* processor);
int32_t sqllexer_process(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_result* out);
int32_t sqllexer_process_packed(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_packed* out);
int32_t sqllexer_process_size(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_size_result* out);
int32_t sqllexer_obfuscate(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_slice* out);
int32_t sqllexer_normalize(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_result* out);
int32_t sqllexer_normalize_packed(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_packed* out);
int32_t sqllexer_tokenize(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_token_list* out);
*/
import "C"

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/DataDog/go-sqllexer"
)

// Obfuscator option bits, matching the constants in rust/sqllexer-ffi/src/lib.rs.
const (
	DollarQuotedFunc uint32 = 1 << iota
	ReplaceDigits
	ReplacePositionalParameter
	ReplaceBoolean
	ReplaceNull
	KeepJsonPath
	ReplaceBindParameter
)

// Normalizer option bits, matching the constants in rust/sqllexer-ffi/src/lib.rs.
const (
	CollectTables uint32 = 1 << iota
	CollectCommands
	CollectComments
	CollectProcedure
	KeepSQLAlias
	UppercaseKeywords
	RemoveSpaceBetweenParentheses
	KeepTrailingSemicolon
	KeepIdentifierQuotation
)

// DBMS codes, matching dbms_from_code in rust/sqllexer-ffi/src/lib.rs.
const (
	dbmsNone uint32 = iota
	dbmsSQLServer
	dbmsPostgres
	dbmsMySQL
	dbmsOracle
	dbmsSnowflake
)

// ErrPanic is returned when the Rust side panicked. The panic is caught at the
// boundary rather than unwinding into the Go runtime, mirroring the Go
// normalizer, which recovers panics into an error.
var ErrPanic = errors.New("sqllexer: rust panicked")

// ErrConcurrentUse is returned when a second call starts on a handle while the
// first is still running or still copying its result out. A handle owns the
// buffers its results are written into, so overlapping calls would corrupt each
// other's output and race on Rust memory that neither the Go race detector nor a
// memory sanitizer inspects.
// Rejecting the call turns that undefined behavior into an error; it does not
// make a shared handle supported. One handle per goroutine remains the model.
var ErrConcurrentUse = errors.New("sqllexer: processor used concurrently by more than one goroutine")

var errNull = errors.New("sqllexer: invalid argument passed to rust")

var errClosed = errors.New("sqllexer: processor is closed")

// Processor is a reusable handle. It is not safe for concurrent use: it owns the
// buffers results are written into, so each goroutine needs its own.
type Processor struct {
	handle unsafe.Pointer

	// The out-parameters live on the handle instead of on each call's stack:
	// taking the address of a local to hand to C moves it to the heap, which is
	// an allocation per statement. None of them holds a Go pointer, so passing
	// them to C is allowed, and the handle is single-threaded by contract.
	packed C.sqllexer_packed
	result C.sqllexer_result
	sized  C.sqllexer_size_result
	slice  C.sqllexer_slice
	tokens C.sqllexer_token_list

	// busy is held from the start of a call until its result has been copied
	// into Go memory. Its only purpose is to detect misuse; it never makes a
	// caller wait.
	busy atomic.Bool
}

// NewProcessor creates a handle for one obfuscator/normalizer configuration.
// Close must be called to release it.
func NewProcessor(obfuscatorFlags, normalizerFlags uint32) *Processor {
	handle := C.sqllexer_processor_new(C.uint32_t(obfuscatorFlags), C.uint32_t(normalizerFlags))
	if handle == nil {
		return nil
	}
	return &Processor{handle: handle}
}

// Close releases the handle. It is idempotent. Closing a handle while another
// goroutine is calling into it is unsupported and not detected: the guard only
// covers overlapping calls.
func (p *Processor) Close() {
	if p.handle == nil {
		return
	}
	C.sqllexer_processor_free(p.handle)
	p.handle = nil
}

// ObfuscateAndNormalize is the Rust equivalent of sqllexer.ObfuscateAndNormalize.
//
// The returned values own their memory: they are decoded into one Go allocation
// that nothing writes to again, so they stay valid after the next call and after
// Close. See [Processor.ObfuscateAndNormalizeInto] for the borrowing variant.
func (p *Processor) ObfuscateAndNormalize(sql string, dbms sqllexer.DBMSType) (string, *sqllexer.StatementMetadata, error) {
	if err := p.acquire(); err != nil {
		return "", nil, err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return "", nil, err
	}
	status := C.sqllexer_process_packed(p.handle, ptr, length, code, &p.packed)
	if err := finish(sql, status); err != nil {
		return "", nil, err
	}
	out, metadata := decodePacked(&p.packed)
	return out, metadata, nil
}

// Normalize is the Rust equivalent of (*sqllexer.Normalizer).Normalize. Its
// results own their memory, as [Processor.ObfuscateAndNormalize]'s do.
func (p *Processor) Normalize(sql string, dbms sqllexer.DBMSType) (string, *sqllexer.StatementMetadata, error) {
	if err := p.acquire(); err != nil {
		return "", nil, err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return "", nil, err
	}
	status := C.sqllexer_normalize_packed(p.handle, ptr, length, code, &p.packed)
	if err := finish(sql, status); err != nil {
		return "", nil, err
	}
	out, metadata := decodePacked(&p.packed)
	return out, metadata, nil
}

// ObfuscateAndNormalizeSize is [Processor.ObfuscateAndNormalize] for callers that
// report the metadata size but never read the values: the metadata is collected,
// since the size is defined by it, but no list is materialized on either side.
//
// The returned string owns its memory.
func (p *Processor) ObfuscateAndNormalizeSize(sql string, dbms sqllexer.DBMSType) (string, int, error) {
	if err := p.acquire(); err != nil {
		return "", 0, err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return "", 0, err
	}
	status := C.sqllexer_process_size(p.handle, ptr, length, code, &p.sized)
	if err := finish(sql, status); err != nil {
		return "", 0, err
	}
	return copyString(p.sized.sql), int(p.sized.metadata_size), nil
}

// Borrowed is the output of [Processor.ObfuscateAndNormalizeInto].
//
// UNLIKE EVERY OTHER RESULT IN THIS PACKAGE, ITS STRINGS DO NOT OWN THEIR
// MEMORY. They point directly into buffers owned by the Processor they came
// from, and are invalidated by the next call on that Processor and by its Close.
// Reading one afterwards reads whatever the handle has written since, or freed
// memory. Copy anything that has to outlive the next call (strings.Clone).
//
// The value is also the caller's scratch space: pass the same one back on every
// call and the binding reuses its slices, which is what makes the path
// allocation-free. Zero values are fine to start from.
type Borrowed struct {
	// SQL is the normalized statement.
	SQL string
	// Size is StatementMetadata.Size.
	Size int

	Tables     []string
	Comments   []string
	Commands   []string
	Procedures []string

	// values backs the four lists above, so one slice is grown instead of four.
	values []string
}

func (b *Borrowed) reset() {
	b.SQL = ""
	b.Size = 0
	b.Tables, b.Comments, b.Commands, b.Procedures = nil, nil, nil, nil
}

// ObfuscateAndNormalizeInto is [Processor.ObfuscateAndNormalize] without the copy
// out of Rust memory: it fills dst with strings that alias the handle's buffers.
//
// The lifetime contract is [Borrowed]'s, and it is strictly weaker than the rest
// of the package's: the results are only valid until the next call on p. It
// exists because for a caller that serializes the result immediately — the shape
// a tracer has — that copy is the entire remaining per-statement allocation.
func (p *Processor) ObfuscateAndNormalizeInto(sql string, dbms sqllexer.DBMSType, dst *Borrowed) error {
	if dst == nil {
		return errNull
	}
	dst.reset()
	if err := p.acquire(); err != nil {
		return err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return err
	}
	status := C.sqllexer_process(p.handle, ptr, length, code, &p.result)
	if err := finish(sql, status); err != nil {
		return err
	}
	result := &p.result
	dst.SQL = borrowString(result.sql)
	dst.Size = int(result.metadata_size)

	total := int(result.tables.len + result.comments.len + result.commands.len + result.procedures.len)
	if cap(dst.values) < total {
		dst.values = make([]string, total)
	}
	values := dst.values[:total]
	next := 0
	dst.Tables, next = borrowList(values, next, result.tables)
	dst.Comments, next = borrowList(values, next, result.comments)
	dst.Commands, next = borrowList(values, next, result.commands)
	dst.Procedures, _ = borrowList(values, next, result.procedures)
	return nil
}

// Obfuscate is the Rust equivalent of (*sqllexer.Obfuscator).Obfuscate.
func (p *Processor) Obfuscate(sql string, dbms sqllexer.DBMSType) (string, error) {
	if err := p.acquire(); err != nil {
		return "", err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return "", err
	}
	status := C.sqllexer_obfuscate(p.handle, ptr, length, code, &p.slice)
	if err := finish(sql, status); err != nil {
		return "", err
	}
	return copyString(p.slice), nil
}

// Token mirrors what sqllexer.Lexer.Scan yields, with the same numeric types.
type Token struct {
	Type  sqllexer.TokenType
	Value string
}

// Tokenize is the Rust equivalent of driving sqllexer.Lexer.Scan to EOF. The EOF
// token is not included, matching how callers loop.
func (p *Processor) Tokenize(sql string, dbms sqllexer.DBMSType) ([]Token, error) {
	if err := p.acquire(); err != nil {
		return nil, err
	}
	defer p.release()

	ptr, length, code, err := p.args(sql, dbms)
	if err != nil {
		return nil, err
	}
	status := C.sqllexer_tokenize(p.handle, ptr, length, code, &p.tokens)
	if err := finish(sql, status); err != nil {
		return nil, err
	}
	list := p.tokens
	if list.ptr == nil || list.len == 0 {
		return nil, nil
	}
	// Most token values are spans of the input the Rust side never had to rewrite,
	// and the input is this Go string, so those are substrings of it rather than
	// copies: same bytes, no allocation, and they own their memory exactly as much
	// as sql does. Only the values the lexer materialized (the unescaping paths)
	// live in Rust memory and have to be copied.
	base := uintptr(unsafe.Pointer(unsafe.StringData(sql)))
	end := base + uintptr(len(sql))
	tokens := make([]Token, int(list.len))
	for i, token := range unsafe.Slice(list.ptr, int(list.len)) {
		value := ""
		switch at := uintptr(unsafe.Pointer(token.value.ptr)); {
		case token.value.ptr == nil || token.value.len == 0:
		case at >= base && at+uintptr(token.value.len) <= end:
			offset := int(at - base)
			value = sql[offset : offset+int(token.value.len)]
		default:
			value = copyString(token.value)
		}
		tokens[i] = Token{Type: sqllexer.TokenType(token.token_type), Value: value}
	}
	// The Rust side borrows the input for token values, so it must outlive the copy.
	runtime.KeepAlive(sql)
	return tokens, nil
}

// acquire claims the handle for one call. The claim has to outlive the call
// itself: the result descriptors point into buffers the handle owns, so a second
// call that starts while the first is still copying them out overwrites the
// bytes being read. Every entry point holds it until it has finished copying.
func (p *Processor) acquire() error {
	if !p.busy.CompareAndSwap(false, true) {
		return ErrConcurrentUse
	}
	if p.handle == nil {
		p.busy.Store(false)
		return errClosed
	}
	return nil
}

func (p *Processor) release() { p.busy.Store(false) }

// args is the entry half every call shares once the handle is claimed: the
// borrowed input pointer. The C call itself is written out at each call site
// rather than passed in as a closure, which would escape and cost an allocation
// on every statement.
func (p *Processor) args(sql string, dbms sqllexer.DBMSType) (*C.uint8_t, C.size_t, C.uint32_t, error) {
	if p.handle == nil {
		return nil, 0, 0, errClosed
	}
	var ptr *C.uint8_t
	if len(sql) > 0 {
		ptr = (*C.uint8_t)(unsafe.Pointer(unsafe.StringData(sql)))
	}
	return ptr, C.size_t(len(sql)), C.uint32_t(dbmsCode(dbms)), nil
}

// finish is the exit half: it ends the input's borrow and maps the status code.
func finish(sql string, status C.int32_t) error {
	// sql is passed to Rust as a borrowed pointer; keep it alive for the call.
	runtime.KeepAlive(sql)
	switch status {
	case 0:
		return nil
	case -2:
		return ErrPanic
	default:
		return errNull
	}
}

// borrowList views one metadata list as strings aliasing the handle's buffers,
// stored in values from next on, and reports where the next list starts.
func borrowList(values []string, next int, list C.sqllexer_slice_list) ([]string, int) {
	start := next
	if list.ptr != nil {
		for _, slice := range unsafe.Slice(list.ptr, int(list.len)) {
			values[next] = borrowString(slice)
			next++
		}
	}
	return values[start:next:next], next
}

// decodePacked turns one packed result into owned Go values with three
// allocations regardless of how much metadata there is: the bytes, the string
// headers, and the metadata struct.
//
// The normalized SQL and every metadata value share one byte slice, filled by
// two memmoves and never written to again, and the strings are windows onto it.
// That is what makes unsafe.String legal here: the memory is Go-owned, kept
// alive by the strings themselves, and immutable from the moment it is handed
// out — so the results survive later calls on the handle, and Close, like the
// per-value copies they replace.
func decodePacked(packed *C.sqllexer_packed) (string, *sqllexer.StatementMetadata) {
	sqlLen := int(packed.sql.len)
	valueLen := int(packed.values.len)
	buf := make([]byte, sqlLen+valueLen)
	if sqlLen > 0 {
		copy(buf, unsafe.Slice((*byte)(unsafe.Pointer(packed.sql.ptr)), sqlLen))
	}
	if valueLen > 0 {
		copy(buf[sqlLen:], unsafe.Slice((*byte)(unsafe.Pointer(packed.values.ptr)), valueLen))
	}

	out := ""
	if sqlLen > 0 {
		out = unsafe.String(unsafe.SliceData(buf), sqlLen)
	}

	total := 0
	for _, count := range packed.counts {
		total += int(count)
	}
	// Always non-nil, so the metadata compares equal to the Go implementation's,
	// which initializes its lists eagerly.
	values := make([]string, total)
	if total > 0 {
		offset := sqlLen
		for i, length := range unsafe.Slice(packed.lens, total) {
			if length == 0 {
				continue
			}
			values[i] = unsafe.String(&buf[offset], int(length))
			offset += int(length)
		}
	}

	metadata := &sqllexer.StatementMetadata{Size: int(packed.metadata_size)}
	next := 0
	take := func(count int) []string {
		start := next
		next += count
		return values[start:next:next]
	}
	metadata.Tables = take(int(packed.counts[0]))
	metadata.Comments = take(int(packed.counts[1]))
	metadata.Commands = take(int(packed.counts[2]))
	metadata.Procedures = take(int(packed.counts[3]))
	return out, metadata
}

// borrowString views Rust-owned bytes as a Go string without copying them. The
// result is only valid until the handle that owns the buffer is called again;
// every caller of this is documented as borrowing.
func borrowString(slice C.sqllexer_slice) string {
	if slice.ptr == nil || slice.len == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(slice.ptr)), int(slice.len))
}

func copyString(slice C.sqllexer_slice) string {
	if slice.ptr == nil || slice.len == 0 {
		return ""
	}
	return C.GoStringN((*C.char)(unsafe.Pointer(slice.ptr)), C.int(slice.len))
}

// dbmsCode resolves the aliases WithDBMS resolves (sql-server, sqlserver,
// postgres), since the Rust side takes a code rather than a name.
func dbmsCode(dbms sqllexer.DBMSType) uint32 {
	switch dbms {
	case sqllexer.DBMSSQLServer, sqllexer.DBMSSQLServerAlias1, sqllexer.DBMSSQLServerAlias2:
		return dbmsSQLServer
	case sqllexer.DBMSPostgres, sqllexer.DBMSPostgresAlias1:
		return dbmsPostgres
	case sqllexer.DBMSMySQL:
		return dbmsMySQL
	case sqllexer.DBMSOracle:
		return dbmsOracle
	case sqllexer.DBMSSnowflake:
		return dbmsSnowflake
	default:
		return dbmsNone
	}
}
