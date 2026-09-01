//go:build rustffi

package main

import (
	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

// The Rust engine under load. The handle is created once per worker and reused
// for every statement, so what the numbers include is the cgo transition plus the
// scan — not object construction. -reuse=false is not meaningful here: creating a
// handle per call would measure malloc, not the port.
func init() {
	engines["rust"] = func(bool) engine {
		return &rustEngine{processor: rustffi.NewProcessor(
			rustffi.ReplaceDigits|rustffi.ReplaceBoolean|rustffi.ReplaceNull,
			rustffi.CollectTables|rustffi.CollectCommands|rustffi.CollectComments,
		)}
	}
}

type rustEngine struct {
	processor *rustffi.Processor
}

func (e *rustEngine) process(sql string, dbms sqllexer.DBMSType) (int, error) {
	out, metadata, err := e.processor.ObfuscateAndNormalize(sql, dbms)
	if err != nil {
		return 0, err
	}
	return len(out) + metadata.Size, nil
}

func (e *rustEngine) close() { e.processor.Close() }
