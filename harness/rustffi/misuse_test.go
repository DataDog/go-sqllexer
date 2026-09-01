//go:build rustffi

package rustffi_test

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

// Deliberate misuse of the FFI object model. Each case pins one of the rules the
// binding documents; together with the sanitizer runs in harness/sanitizers they
// are the evidence for acceptance criteria B1-B4.
//
// Two forms of misuse are deliberately *not* exercised from Go, because they are
// undefined behavior that no Go-level assertion can observe: reading a result
// after the handle is freed, and passing a handle's own output buffer back in as
// input. Both live in rust/sqllexer-ffi/tests/stress.rs, where a sanitizer can
// see them.

func TestClosedHandleIsRejectedByEveryEntryPoint(t *testing.T) {
	processor := rustffi.NewProcessor(0, rustffi.CollectTables)
	processor.Close()
	processor.Close() // idempotent

	if _, _, err := processor.ObfuscateAndNormalize("SELECT 1", ""); err == nil {
		t.Error("ObfuscateAndNormalize on a closed handle returned no error")
	}
	if _, _, err := processor.Normalize("SELECT 1", ""); err == nil {
		t.Error("Normalize on a closed handle returned no error")
	}
	if _, err := processor.Obfuscate("SELECT 1", ""); err == nil {
		t.Error("Obfuscate on a closed handle returned no error")
	}
	if _, err := processor.Tokenize("SELECT 1", ""); err == nil {
		t.Error("Tokenize on a closed handle returned no error")
	}
}

// A null input pointer is what an empty Go string produces, so this is the
// ordinary path rather than an exotic one: it must be accepted, not dereferenced.
func TestEmptyInputPassesANullPointer(t *testing.T) {
	processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables)
	defer processor.Close()

	sql, metadata, err := processor.ObfuscateAndNormalize("", "")
	if err != nil || sql != "" || metadata == nil || len(metadata.Tables) != 0 {
		t.Fatalf("empty input: sql=%q metadata=%+v err=%v", sql, metadata, err)
	}
	if tokens, err := processor.Tokenize("", ""); err != nil || len(tokens) != 0 {
		t.Fatalf("empty input tokenize: tokens=%v err=%v", tokens, err)
	}
	if obfuscated, err := processor.Obfuscate("", ""); err != nil || obfuscated != "" {
		t.Fatalf("empty input obfuscate: %q %v", obfuscated, err)
	}
}

// Results are copies, so destroying the handle they came from cannot invalidate
// them. Without the copy in the binding this reads freed Rust memory.
func TestResultsOutliveTheHandleThatProducedThem(t *testing.T) {
	processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables|rustffi.CollectCommands)

	sql, metadata, err := processor.ObfuscateAndNormalize("/* c */ SELECT * FROM survivors WHERE id = 7", "")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := processor.Tokenize("SELECT * FROM survivors", "")
	if err != nil {
		t.Fatal(err)
	}

	processor.Close()
	// Give the allocator every chance to reuse the freed buffers before the
	// values are read back.
	runtime.GC()
	scratch := make([][]byte, 64)
	for i := range scratch {
		scratch[i] = []byte(strings.Repeat("x", 4096))
	}
	runtime.GC()

	if sql != "SELECT * FROM survivors WHERE id = ?" {
		t.Errorf("sql was corrupted after Close: %q", sql)
	}
	if len(metadata.Tables) != 1 || metadata.Tables[0] != "survivors" {
		t.Errorf("metadata was corrupted after Close: %q", metadata.Tables)
	}
	var joined strings.Builder
	for _, token := range tokens {
		joined.WriteString(token.Value)
	}
	if joined.String() != "SELECT * FROM survivors" {
		t.Errorf("token values were corrupted after Close: %q", joined.String())
	}
	runtime.KeepAlive(scratch)
}

// Token values borrow the caller's input on the Rust side, so the binding has to
// copy them before the input can be collected. A garbage-collected input with
// live token values is the case that would otherwise read freed Go memory.
func TestTokenValuesDoNotBorrowTheInput(t *testing.T) {
	processor := rustffi.NewProcessor(0, 0)
	defer processor.Close()

	input := string(append([]byte("SELECT * FROM "), []byte("borrowed_table")...))
	tokens, err := processor.Tokenize(input, "")
	if err != nil {
		t.Fatal(err)
	}
	input = ""
	runtime.GC()
	runtime.GC()

	var joined strings.Builder
	for _, token := range tokens {
		joined.WriteString(token.Value)
	}
	if joined.String() != "SELECT * FROM borrowed_table" {
		t.Fatalf("token values did not survive the input: %q", joined.String())
	}
}

// Concurrent use of one handle is unsupported. It is rejected rather than
// silently allowed, because the corruption it causes is in Rust-owned memory,
// where neither the Go race detector nor ASan would report it.
func TestConcurrentUseOfOneHandleIsRejected(t *testing.T) {
	processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables)
	defer processor.Close()

	release := processor.AcquireForTest()
	if _, _, err := processor.ObfuscateAndNormalize("SELECT 1", ""); !errors.Is(err, rustffi.ErrConcurrentUse) {
		t.Fatalf("expected ErrConcurrentUse while a call is in flight, got %v", err)
	}
	if _, err := processor.Tokenize("SELECT 1", ""); !errors.Is(err, rustffi.ErrConcurrentUse) {
		t.Fatalf("expected ErrConcurrentUse from Tokenize, got %v", err)
	}
	release()

	// The guard must not leak: the handle works again once the call is done.
	if _, _, err := processor.ObfuscateAndNormalize("SELECT * FROM t WHERE id = 1", ""); err != nil {
		t.Fatalf("handle unusable after the guard was released: %v", err)
	}
}

// The supported model: one handle per goroutine, which is what ffirunner and the
// throughput harness do. Run with -race to check the Go side of it.
func TestOneHandlePerGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables)
			defer processor.Close()
			table := strings.Repeat("t", worker+1)
			for i := 0; i < 500; i++ {
				sql, metadata, err := processor.ObfuscateAndNormalize("SELECT * FROM "+table, sqllexer.DBMSPostgres)
				if err != nil {
					t.Errorf("worker %d: %v", worker, err)
					return
				}
				if sql != "SELECT * FROM "+table || len(metadata.Tables) != 1 || metadata.Tables[0] != table {
					t.Errorf("worker %d saw another worker's output: %q %q", worker, sql, metadata.Tables)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

// The same misuse under real contention rather than a held guard. Opt-in,
// because whether two goroutines actually overlap is a scheduling accident: with
// the guard in place the failure mode is a returned error, but a run that
// happens never to overlap proves nothing, so it must not be able to fail CI.
// harness/sanitizers/run.sh runs it and records how many collisions it saw.
func TestConcurrentMisuseUnderLoad(t *testing.T) {
	if os.Getenv("SQLLEXER_FFI_MISUSE_STRESS") == "" {
		t.Skip("set SQLLEXER_FFI_MISUSE_STRESS=1 to run the deliberate concurrent-misuse stress")
	}
	processor := rustffi.NewProcessor(rustffi.ReplaceDigits, rustffi.CollectTables)
	defer processor.Close()

	var rejected, completed, corrupted int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	deadline := time.Now().Add(5 * time.Second)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				sql, metadata, err := processor.ObfuscateAndNormalize("SELECT * FROM shared WHERE id = 1", "")
				mu.Lock()
				switch {
				case errors.Is(err, rustffi.ErrConcurrentUse):
					rejected++
				case err != nil:
					corrupted++
				case sql != "SELECT * FROM shared WHERE id = ?" || len(metadata.Tables) != 1 || metadata.Tables[0] != "shared":
					corrupted++
				default:
					completed++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	t.Logf("concurrent misuse: %d calls rejected, %d completed, %d produced wrong output", rejected, completed, corrupted)
	if corrupted != 0 {
		t.Fatalf("%d calls produced corrupted output: the guard let overlapping calls through", corrupted)
	}
	if rejected == 0 {
		t.Log("no overlap was observed in this run; the guard was never exercised")
	}
}
