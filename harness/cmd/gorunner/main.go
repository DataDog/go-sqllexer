// Command gorunner is the Go reference implementation of the harness protocol.
//
// It reads protocol.Request objects as newline-delimited JSON on stdin and writes
// one protocol.Response per line to stdout, preserving input order. The future Rust
// implementation must ship a binary that is byte-for-byte interchangeable with this
// one, which is what makes the differential driver possible.
//
//	go run ./harness/cmd/gorunner < corpus/testdata.jsonl > go.out.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

const maxLine = 8 << 20

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), maxLine)
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	defer out.Flush()
	enc := json.NewEncoder(out)

	for in.Scan() {
		if len(in.Bytes()) == 0 {
			continue
		}
		var req protocol.Request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			fmt.Fprintf(os.Stderr, "malformed request: %v\n", err)
			os.Exit(1)
		}
		if err := enc.Encode(handle(req)); err != nil {
			fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
			os.Exit(1)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
		os.Exit(1)
	}
}

func handle(req protocol.Request) protocol.Response {
	resp := protocol.Response{ID: req.ID}

	switch req.Mode {
	case protocol.ModeTokenize:
		lexer := sqllexer.New(string(req.SQL), sqllexer.WithDBMS(sqllexer.DBMSType(req.DBMS)))
		for {
			token := lexer.Scan()
			if token.Type == sqllexer.EOF {
				break
			}
			resp.Tokens = append(resp.Tokens, protocol.Token{Type: int(token.Type), Value: protocol.Text(token.Value)})
		}
	case protocol.ModeObfuscate:
		resp.Output = protocol.Text(newObfuscator(req).Obfuscate(string(req.SQL), sqllexer.WithDBMS(sqllexer.DBMSType(req.DBMS))))
	case protocol.ModeNormalize:
		output, metadata, err := newNormalizer(req).Normalize(string(req.SQL), sqllexer.WithDBMS(sqllexer.DBMSType(req.DBMS)))
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		resp.Output = protocol.Text(output)
		resp.Metadata = convertMetadata(metadata)
	case protocol.ModeObfuscateAndNormalize:
		output, metadata, err := sqllexer.ObfuscateAndNormalize(
			string(req.SQL), newObfuscator(req), newNormalizer(req), sqllexer.WithDBMS(sqllexer.DBMSType(req.DBMS)),
		)
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		resp.Output = protocol.Text(output)
		resp.Metadata = convertMetadata(metadata)
	default:
		resp.Error = fmt.Sprintf("unknown mode %q", req.Mode)
	}
	return resp
}

func newObfuscator(req protocol.Request) *sqllexer.Obfuscator {
	cfg := protocol.DefaultObfuscatorConfig()
	if req.Obfuscator != nil {
		cfg = *req.Obfuscator
	}
	return sqllexer.NewObfuscator(
		sqllexer.WithDollarQuotedFunc(cfg.DollarQuotedFunc),
		sqllexer.WithReplaceDigits(cfg.ReplaceDigits),
		sqllexer.WithReplacePositionalParameter(cfg.ReplacePositionalParameter),
		sqllexer.WithReplaceBoolean(cfg.ReplaceBoolean),
		sqllexer.WithReplaceNull(cfg.ReplaceNull),
		sqllexer.WithKeepJsonPath(cfg.KeepJsonPath),
		sqllexer.WithReplaceBindParameter(cfg.ReplaceBindParameter),
	)
}

func newNormalizer(req protocol.Request) *sqllexer.Normalizer {
	cfg := protocol.DefaultNormalizerConfig()
	if req.Normalizer != nil {
		cfg = *req.Normalizer
	}
	return sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(cfg.CollectTables),
		sqllexer.WithCollectCommands(cfg.CollectCommands),
		sqllexer.WithCollectComments(cfg.CollectComments),
		sqllexer.WithCollectProcedures(cfg.CollectProcedure),
		sqllexer.WithKeepSQLAlias(cfg.KeepSQLAlias),
		sqllexer.WithUppercaseKeywords(cfg.UppercaseKeywords),
		sqllexer.WithRemoveSpaceBetweenParentheses(cfg.RemoveSpaceBetweenParentheses),
		sqllexer.WithKeepTrailingSemicolon(cfg.KeepTrailingSemicolon),
		sqllexer.WithKeepIdentifierQuotation(cfg.KeepIdentifierQuotation),
	)
}

func convertMetadata(m *sqllexer.StatementMetadata) *protocol.Metadata {
	if m == nil {
		return nil
	}
	return &protocol.Metadata{
		Size:       m.Size,
		Tables:     texts(m.Tables),
		Comments:   texts(m.Comments),
		Commands:   texts(m.Commands),
		Procedures: texts(m.Procedures),
	}
}

func texts(values []string) []protocol.Text {
	if values == nil {
		return nil
	}
	out := make([]protocol.Text, len(values))
	for i, v := range values {
		out[i] = protocol.Text(v)
	}
	return out
}
