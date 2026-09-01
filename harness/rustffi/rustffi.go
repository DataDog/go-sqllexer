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

typedef struct { uint32_t token_type; sqllexer_slice value; } sqllexer_token;
typedef struct { const sqllexer_token* ptr; size_t len; } sqllexer_token_list;

void* sqllexer_processor_new(uint32_t obfuscator_flags, uint32_t normalizer_flags);
void sqllexer_processor_free(void* processor);
int32_t sqllexer_process(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_result* out);
int32_t sqllexer_obfuscate(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_slice* out);
int32_t sqllexer_normalize(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_result* out);
int32_t sqllexer_tokenize(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_token_list* out);
*/
import "C"

import (
	"errors"
	"runtime"
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

var errNull = errors.New("sqllexer: invalid argument passed to rust")

// Processor is a reusable handle. It is not safe for concurrent use: it owns the
// buffers results are written into, so each goroutine needs its own.
type Processor struct {
	handle unsafe.Pointer
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

// Close releases the handle. It is idempotent.
func (p *Processor) Close() {
	if p.handle == nil {
		return
	}
	C.sqllexer_processor_free(p.handle)
	p.handle = nil
}

// ObfuscateAndNormalize is the Rust equivalent of sqllexer.ObfuscateAndNormalize.
//
// The returned values are copied out of the Rust buffers, so they stay valid
// after the next call.
func (p *Processor) ObfuscateAndNormalize(sql string, dbms sqllexer.DBMSType) (string, *sqllexer.StatementMetadata, error) {
	var result C.sqllexer_result
	err := p.call(sql, dbms, func(ptr *C.uint8_t, length C.size_t, code C.uint32_t) C.int32_t {
		return C.sqllexer_process(p.handle, ptr, length, code, &result)
	})
	if err != nil {
		return "", nil, err
	}
	return goString(result.sql), goMetadata(result), nil
}

// Normalize is the Rust equivalent of (*sqllexer.Normalizer).Normalize.
func (p *Processor) Normalize(sql string, dbms sqllexer.DBMSType) (string, *sqllexer.StatementMetadata, error) {
	var result C.sqllexer_result
	err := p.call(sql, dbms, func(ptr *C.uint8_t, length C.size_t, code C.uint32_t) C.int32_t {
		return C.sqllexer_normalize(p.handle, ptr, length, code, &result)
	})
	if err != nil {
		return "", nil, err
	}
	return goString(result.sql), goMetadata(result), nil
}

// Obfuscate is the Rust equivalent of (*sqllexer.Obfuscator).Obfuscate.
func (p *Processor) Obfuscate(sql string, dbms sqllexer.DBMSType) (string, error) {
	var result C.sqllexer_slice
	err := p.call(sql, dbms, func(ptr *C.uint8_t, length C.size_t, code C.uint32_t) C.int32_t {
		return C.sqllexer_obfuscate(p.handle, ptr, length, code, &result)
	})
	if err != nil {
		return "", err
	}
	return goString(result), nil
}

// Token mirrors what sqllexer.Lexer.Scan yields, with the same numeric types.
type Token struct {
	Type  sqllexer.TokenType
	Value string
}

// Tokenize is the Rust equivalent of driving sqllexer.Lexer.Scan to EOF. The EOF
// token is not included, matching how callers loop.
func (p *Processor) Tokenize(sql string, dbms sqllexer.DBMSType) ([]Token, error) {
	var list C.sqllexer_token_list
	err := p.call(sql, dbms, func(ptr *C.uint8_t, length C.size_t, code C.uint32_t) C.int32_t {
		return C.sqllexer_tokenize(p.handle, ptr, length, code, &list)
	})
	if err != nil {
		return nil, err
	}
	if list.ptr == nil || list.len == 0 {
		return nil, nil
	}
	tokens := make([]Token, 0, int(list.len))
	for _, token := range unsafe.Slice(list.ptr, int(list.len)) {
		tokens = append(tokens, Token{
			Type:  sqllexer.TokenType(token.token_type),
			Value: goString(token.value),
		})
	}
	// The Rust side borrows the input for token values, so it must outlive the copy.
	runtime.KeepAlive(sql)
	return tokens, nil
}

// call handles the parts every entry point shares: the closed-handle check, the
// borrowed input pointer and its lifetime, and the status code.
func (p *Processor) call(sql string, dbms sqllexer.DBMSType, invoke func(*C.uint8_t, C.size_t, C.uint32_t) C.int32_t) error {
	if p.handle == nil {
		return errors.New("sqllexer: processor is closed")
	}
	var ptr *C.uint8_t
	if len(sql) > 0 {
		ptr = (*C.uint8_t)(unsafe.Pointer(unsafe.StringData(sql)))
	}
	status := invoke(ptr, C.size_t(len(sql)), C.uint32_t(dbmsCode(dbms)))
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

func goMetadata(result C.sqllexer_result) *sqllexer.StatementMetadata {
	return &sqllexer.StatementMetadata{
		Size:       int(result.metadata_size),
		Tables:     goStrings(result.tables),
		Comments:   goStrings(result.comments),
		Commands:   goStrings(result.commands),
		Procedures: goStrings(result.procedures),
	}
}

func goString(slice C.sqllexer_slice) string {
	if slice.ptr == nil || slice.len == 0 {
		return ""
	}
	return C.GoStringN((*C.char)(unsafe.Pointer(slice.ptr)), C.int(slice.len))
}

// goStrings always returns a non-nil slice so results compare equal to the Go
// implementation's, which initializes its metadata slices eagerly.
func goStrings(list C.sqllexer_slice_list) []string {
	out := make([]string, 0, int(list.len))
	if list.ptr == nil {
		return out
	}
	slices := unsafe.Slice(list.ptr, int(list.len))
	for _, slice := range slices {
		out = append(out, goString(slice))
	}
	return out
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
