//go:build rustffi

// Command ffirunner speaks the harness protocol backed by the Rust core through
// cgo, so the integration path can be diffed with the same corpora that validate
// the pure Rust core. A difference between this and sqllexer-runner is a bug in
// the binding, not in the port.
//
// All four protocol modes go through the binding, so the whole exported surface
// is diffed against Go and not just the combined path.
//
//	cd rust && cargo build --release -p sqllexer-ffi
//	go run -tags rustffi ./harness/cmd/ffirunner < corpus/testdata.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

const maxLine = 8 << 20

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), maxLine)
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	enc := json.NewEncoder(out)

	// One handle per configuration, reused across requests, as production would.
	processors := map[configKey]*rustffi.Processor{}
	defer func() {
		for _, processor := range processors {
			processor.Close()
		}
	}()

	for in.Scan() {
		if len(in.Bytes()) == 0 {
			continue
		}
		var req protocol.Request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			fmt.Fprintf(os.Stderr, "malformed request: %v\n", err)
			os.Exit(1)
		}
		if err := enc.Encode(handle(req, processors)); err != nil {
			fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
			os.Exit(1)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
		os.Exit(1)
	}
}

type configKey struct {
	obfuscator protocol.ObfuscatorConfig
	normalizer protocol.NormalizerConfig
}

func handle(req protocol.Request, processors map[configKey]*rustffi.Processor) protocol.Response {
	resp := protocol.Response{ID: req.ID}
	key := configKey{obfuscator: protocol.DefaultObfuscatorConfig(), normalizer: protocol.DefaultNormalizerConfig()}
	if req.Obfuscator != nil {
		key.obfuscator = *req.Obfuscator
	}
	if req.Normalizer != nil {
		key.normalizer = *req.Normalizer
	}
	processor, ok := processors[key]
	if !ok {
		processor = rustffi.NewProcessor(obfuscatorFlags(key.obfuscator), normalizerFlags(key.normalizer))
		processors[key] = processor
	}

	dbms := sqllexer.DBMSType(req.DBMS)
	var (
		output   string
		metadata *sqllexer.StatementMetadata
		err      error
	)
	switch req.Mode {
	case protocol.ModeTokenize:
		tokens, err := processor.Tokenize(string(req.SQL), dbms)
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		for _, token := range tokens {
			resp.Tokens = append(resp.Tokens, protocol.Token{Type: int(token.Type), Value: protocol.Text(token.Value)})
		}
		return resp
	case protocol.ModeObfuscate:
		output, err = processor.Obfuscate(string(req.SQL), dbms)
	case protocol.ModeNormalize:
		output, metadata, err = processor.Normalize(string(req.SQL), dbms)
	case protocol.ModeObfuscateAndNormalize:
		output, metadata, err = processor.ObfuscateAndNormalize(string(req.SQL), dbms)
	default:
		resp.Error = fmt.Sprintf("unknown mode %q", req.Mode)
		return resp
	}
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	resp.Output = protocol.Text(output)
	if metadata != nil {
		resp.Metadata = &protocol.Metadata{
			Size:       metadata.Size,
			Tables:     texts(metadata.Tables),
			Comments:   texts(metadata.Comments),
			Commands:   texts(metadata.Commands),
			Procedures: texts(metadata.Procedures),
		}
	}
	return resp
}

func obfuscatorFlags(cfg protocol.ObfuscatorConfig) uint32 {
	var flags uint32
	for _, bit := range []struct {
		set  bool
		flag uint32
	}{
		{cfg.DollarQuotedFunc, rustffi.DollarQuotedFunc},
		{cfg.ReplaceDigits, rustffi.ReplaceDigits},
		{cfg.ReplacePositionalParameter, rustffi.ReplacePositionalParameter},
		{cfg.ReplaceBoolean, rustffi.ReplaceBoolean},
		{cfg.ReplaceNull, rustffi.ReplaceNull},
		{cfg.KeepJsonPath, rustffi.KeepJsonPath},
		{cfg.ReplaceBindParameter, rustffi.ReplaceBindParameter},
	} {
		if bit.set {
			flags |= bit.flag
		}
	}
	return flags
}

func normalizerFlags(cfg protocol.NormalizerConfig) uint32 {
	var flags uint32
	for _, bit := range []struct {
		set  bool
		flag uint32
	}{
		{cfg.CollectTables, rustffi.CollectTables},
		{cfg.CollectCommands, rustffi.CollectCommands},
		{cfg.CollectComments, rustffi.CollectComments},
		{cfg.CollectProcedure, rustffi.CollectProcedure},
		{cfg.KeepSQLAlias, rustffi.KeepSQLAlias},
		{cfg.UppercaseKeywords, rustffi.UppercaseKeywords},
		{cfg.RemoveSpaceBetweenParentheses, rustffi.RemoveSpaceBetweenParentheses},
		{cfg.KeepTrailingSemicolon, rustffi.KeepTrailingSemicolon},
		{cfg.KeepIdentifierQuotation, rustffi.KeepIdentifierQuotation},
	} {
		if bit.set {
			flags |= bit.flag
		}
	}
	return flags
}

func texts(values []string) []protocol.Text {
	out := make([]protocol.Text, len(values))
	for i, v := range values {
		out[i] = protocol.Text(v)
	}
	return out
}
