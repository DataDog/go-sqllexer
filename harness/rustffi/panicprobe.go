//go:build rustffi && rustffi_panicprobe

// The panic probe is behind its own build tag because the symbol it calls only
// exists when the Rust archive is built with the `panic-probe` feature:
//
//	(cd rust && cargo build --release -p sqllexer-ffi --features panic-probe)
//	go test -a -tags "rustffi rustffi_panicprobe" ./harness/rustffi/
//
// Its only purpose is to prove that a Rust panic is contained at the boundary
// rather than unwinding into the Go runtime, which is not something the ordinary
// entry points can be made to do on demand.
package rustffi

/*
#cgo LDFLAGS: ${SRCDIR}/../../rust/target/release/libsqllexer_ffi.a -lm -ldl -pthread
#include <stdint.h>
#include <stdlib.h>

typedef struct { const uint8_t* ptr; size_t len; } sqllexer_slice;

int32_t sqllexer_panic_probe(void* processor, sqllexer_slice* out);
*/
import "C"

import "errors"

var errProbeClosed = errors.New("sqllexer: processor is closed")

// PanicProbe calls a Rust entry point that panics on purpose. It returns
// ErrPanic if the panic was caught at the boundary; if it were not, the process
// would die rather than return.
func (p *Processor) PanicProbe() error {
	if !p.busy.CompareAndSwap(false, true) {
		return ErrConcurrentUse
	}
	defer p.busy.Store(false)
	if p.handle == nil {
		return errProbeClosed
	}
	var out C.sqllexer_slice
	switch C.sqllexer_panic_probe(p.handle, &out) {
	case 0:
		return nil
	case -2:
		return ErrPanic
	default:
		return errNull
	}
}
