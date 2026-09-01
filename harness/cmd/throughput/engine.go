package main

import (
	"fmt"
	"sort"

	"github.com/DataDog/go-sqllexer"
)

// engine is one worker's view of an implementation under load. Each worker owns
// its own engine, which is what lets the Rust side hold a handle per worker (the
// handles are deliberately not safe for concurrent use) and the Go side hold its
// own obfuscator/normalizer pair.
type engine interface {
	// process runs one statement. Output is returned so the caller can keep the
	// compiler and the allocator honest; errors are counted, not fatal.
	process(sql string, dbms sqllexer.DBMSType) (int, error)
	close()
}

// engines maps -impl values to per-worker constructors. The Rust entries register
// themselves from a build-tagged file, so this command still builds without a
// Rust toolchain.
var engines = map[string]func(reuse bool) engine{
	"go": func(reuse bool) engine { return &goEngine{reuse: reuse} },
}

func engineNames() string {
	names := make([]string, 0, len(engines))
	for name := range engines {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprint(names)
}

// goEngine is the baseline: the package as it exists today.
type goEngine struct {
	reuse      bool
	obfuscator *sqllexer.Obfuscator
	normalizer *sqllexer.Normalizer
}

func (e *goEngine) process(sql string, dbms sqllexer.DBMSType) (int, error) {
	obfuscator, normalizer := e.obfuscator, e.normalizer
	if !e.reuse || obfuscator == nil {
		obfuscator, normalizer = defaultObfuscator(), defaultNormalizer()
		if e.reuse {
			e.obfuscator, e.normalizer = obfuscator, normalizer
		}
	}
	out, metadata, err := sqllexer.ObfuscateAndNormalize(sql, obfuscator, normalizer, sqllexer.WithDBMS(dbms))
	if err != nil {
		return 0, err
	}
	return len(out) + metadata.Size, nil
}

func (e *goEngine) close() {}

func defaultObfuscator() *sqllexer.Obfuscator {
	return sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(true),
		sqllexer.WithReplaceBoolean(true),
		sqllexer.WithReplaceNull(true),
	)
}

func defaultNormalizer() *sqllexer.Normalizer {
	return sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
		sqllexer.WithCollectCommands(true),
		sqllexer.WithCollectComments(true),
	)
}
