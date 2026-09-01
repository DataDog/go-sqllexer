//go:build rustffi && rustffi_panicprobe

package rustffi_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

// B1: a Rust panic must not unwind into the Go runtime. If it did, this test
// would not fail — the process would abort, which is the point: the assertion is
// that execution continues at all, and that the handle is still usable
// afterwards.
func TestRustPanicIsContainedAtTheBoundary(t *testing.T) {
	processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables)
	defer processor.Close()

	for i := 0; i < 100; i++ {
		if err := processor.PanicProbe(); !errors.Is(err, rustffi.ErrPanic) {
			t.Fatalf("panic probe %d: expected ErrPanic, got %v", i, err)
		}
		// A caught panic leaves the handle's buffers in whatever state the
		// panicking call left them; the next call must still be correct.
		sql, metadata, err := processor.ObfuscateAndNormalize("SELECT * FROM after_panic WHERE id = 3", "")
		if err != nil {
			t.Fatalf("call after panic %d: %v", i, err)
		}
		if sql != "SELECT * FROM after_panic WHERE id = ?" {
			t.Fatalf("call after panic %d returned %q", i, sql)
		}
		if len(metadata.Tables) != 1 || metadata.Tables[0] != "after_panic" {
			t.Fatalf("call after panic %d returned tables %q", i, metadata.Tables)
		}
	}

	// The Go runtime has to be intact, not just alive: a corrupted stack from a
	// foreign unwind usually shows up as a crash in the allocator or the GC.
	runtime.GC()
	junk := make([][]byte, 1024)
	for i := range junk {
		junk[i] = make([]byte, 1024)
	}
	runtime.GC()
	runtime.KeepAlive(junk)
}

// The same panic on a goroutine that is not the main one, so the unwinder sees a
// Go stack that started on a small, growable stack rather than the system stack.
func TestRustPanicIsContainedOnAGoroutineStack(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		processor := rustffi.NewProcessor(0, 0)
		defer processor.Close()
		done <- deepCall(64, processor.PanicProbe)
	}()
	if err := <-done; !errors.Is(err, rustffi.ErrPanic) {
		t.Fatalf("expected ErrPanic from a goroutine, got %v", err)
	}
}

// Recursion forces at least one stack growth before the cgo call, so the probe
// runs on a stack that Go has already moved.
func deepCall(depth int, fn func() error) error {
	if depth == 0 {
		return fn()
	}
	var padding [512]byte
	err := deepCall(depth-1, fn)
	runtime.KeepAlive(padding)
	return err
}
