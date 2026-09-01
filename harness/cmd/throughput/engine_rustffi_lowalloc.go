//go:build rustffi

package main

import (
	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

// The two lower-allocation shapes of the same work, so ../../reports/ZERO-COPY.md
// can price them against "rust" under the same load rather than only in a
// microbenchmark.
//
// rust-borrowed is the shape a caller that serializes the result immediately
// has: the strings alias the handle's buffers and die on the next call, which is
// exactly the lifetime a request-scoped consumer needs. rust-size is the shape a
// caller that only reports the metadata size has.
func init() {
	engines["rust-borrowed"] = func(bool) engine {
		return &rustBorrowedEngine{processor: newLowAllocProcessor()}
	}
	engines["rust-size"] = func(bool) engine {
		return &rustSizeEngine{processor: newLowAllocProcessor()}
	}
}

// Same configuration as the "rust" engine, so the numbers are comparable.
func newLowAllocProcessor() *rustffi.Processor {
	return rustffi.NewProcessor(
		rustffi.ReplaceDigits|rustffi.ReplaceBoolean|rustffi.ReplaceNull,
		rustffi.CollectTables|rustffi.CollectCommands|rustffi.CollectComments,
	)
}

type rustBorrowedEngine struct {
	processor *rustffi.Processor
	borrowed  rustffi.Borrowed
}

func (e *rustBorrowedEngine) process(sql string, dbms sqllexer.DBMSType) (int, error) {
	if err := e.processor.ObfuscateAndNormalizeInto(sql, dbms, &e.borrowed); err != nil {
		return 0, err
	}
	// Read the borrowed values while they are still valid, which is also what
	// keeps the compiler from eliding the work.
	return len(e.borrowed.SQL) + e.borrowed.Size, nil
}

func (e *rustBorrowedEngine) close() { e.processor.Close() }

type rustSizeEngine struct {
	processor *rustffi.Processor
}

func (e *rustSizeEngine) process(sql string, dbms sqllexer.DBMSType) (int, error) {
	out, size, err := e.processor.ObfuscateAndNormalizeSize(sql, dbms)
	if err != nil {
		return 0, err
	}
	return len(out) + size, nil
}

func (e *rustSizeEngine) close() { e.processor.Close() }
