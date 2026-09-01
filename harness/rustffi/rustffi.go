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

void* sqllexer_processor_new(uint32_t obfuscator_flags, uint32_t normalizer_flags);
void sqllexer_processor_free(void* processor);
int32_t sqllexer_process(void* processor, const uint8_t* sql, size_t sql_len, uint32_t dbms, sqllexer_result* out);
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
	if p.handle == nil {
		return "", nil, errors.New("sqllexer: processor is closed")
	}

	var result C.sqllexer_result
	var ptr *C.uint8_t
	if len(sql) > 0 {
		ptr = (*C.uint8_t)(unsafe.Pointer(unsafe.StringData(sql)))
	}
	status := C.sqllexer_process(p.handle, ptr, C.size_t(len(sql)), C.uint32_t(dbmsCode(dbms)), &result)
	// sql is passed to Rust as a borrowed pointer; keep it alive for the call.
	runtime.KeepAlive(sql)

	switch status {
	case 0:
	case -2:
		return "", nil, ErrPanic
	default:
		return "", nil, errNull
	}

	metadata := &sqllexer.StatementMetadata{
		Size:       int(result.metadata_size),
		Tables:     goStrings(result.tables),
		Comments:   goStrings(result.comments),
		Commands:   goStrings(result.commands),
		Procedures: goStrings(result.procedures),
	}
	return goString(result.sql), metadata, nil
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
