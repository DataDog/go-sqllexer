package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// FuzzParity is the continuous differential fuzzer: every input is answered by the
// Go oracle in this process and by the Rust implementation over the harness
// protocol, and the two answers must agree.
//
// The Rust side is a long-lived subprocess rather than a linked library. That costs
// a pipe round trip per execution, but it keeps the fuzzer independent of any
// binding: what is being proven here is that the rewrite is faithful, not that some
// particular integration is.
//
//	(cd rust && cargo build --release)
//	go test -run xxx -fuzz FuzzParity -fuzztime 10m ./harness/cmd/gorunner/
func FuzzParity(f *testing.F) {
	rust := startRustRunner(f)

	for _, seed := range []string{
		"SELECT * FROM users WHERE id = 42",
		"SELECT id1, id2 FROM t /* comment */ WHERE b = true AND c IS NULL",
		"INSERT INTO \"Table\" VALUES ($1, :name, ?, 'x'), (E'\\\\x', $$body$$)",
		"UPDATE a SET b = 1 WHERE c IN (SELECT d FROM e) -- trailing",
		"WITH cte AS (SELECT 1) SELECT * FROM cte JOIN t ON t.id = cte.id;",
		"BEGIN; EXEC dbo.proc @p = N'x'; COMMIT;",
		"SELECT data->'a'->>'b' FROM t",
		"SELECT * FROM t WHERE s = 'unterminated",
		"\xff\xfe invalid utf8 \x00",
		"",
	} {
		// The selector bytes pick mode, DBMS and option bits; 0 is a valid choice
		// for each, so a bare seed already exercises a complete configuration.
		f.Add(seed, uint8(0), uint8(0), uint8(0), uint16(0))
	}

	f.Fuzz(func(t *testing.T, sql string, mode, dbms, obfuscatorBits uint8, normalizerBits uint16) {
		req := protocol.Request{
			ID:         "fuzz",
			SQL:        protocol.Text(sql),
			Mode:       protocol.Modes[int(mode)%len(protocol.Modes)],
			DBMS:       fuzzDBMS[int(dbms)%len(fuzzDBMS)],
			Obfuscator: obfuscatorFromBits(obfuscatorBits),
			Normalizer: normalizerFromBits(normalizerBits),
		}

		want := handle(req)
		got, err := rust.roundTrip(req)
		if err != nil {
			t.Fatalf("rust runner: %v", err)
		}
		if a, b := canonical(t, want), canonical(t, got); a != b {
			t.Fatalf("parity divergence for %q (mode %s, dbms %q)\n go: %s\nrust: %s",
				sql, req.Mode, req.DBMS, a, b)
		}
	})
}

// Every DBMS the library accepts, including the aliases: the alias table is part of
// what the rewrite had to reproduce.
var fuzzDBMS = []string{
	"", "mssql", "sql-server", "sqlserver", "postgresql", "postgres",
	"mysql", "oracle", "snowflake",
}

func obfuscatorFromBits(bits uint8) *protocol.ObfuscatorConfig {
	return &protocol.ObfuscatorConfig{
		DollarQuotedFunc:           bits&1 != 0,
		ReplaceDigits:              bits&2 != 0,
		ReplacePositionalParameter: bits&4 != 0,
		ReplaceBoolean:             bits&8 != 0,
		ReplaceNull:                bits&16 != 0,
		KeepJsonPath:               bits&32 != 0,
		ReplaceBindParameter:       bits&64 != 0,
	}
}

func normalizerFromBits(bits uint16) *protocol.NormalizerConfig {
	return &protocol.NormalizerConfig{
		CollectTables:                 bits&1 != 0,
		CollectCommands:               bits&2 != 0,
		CollectComments:               bits&4 != 0,
		CollectProcedure:              bits&8 != 0,
		KeepSQLAlias:                  bits&16 != 0,
		UppercaseKeywords:             bits&32 != 0,
		RemoveSpaceBetweenParentheses: bits&64 != 0,
		KeepTrailingSemicolon:         bits&128 != 0,
		KeepIdentifierQuotation:       bits&256 != 0,
	}
}

// canonical renders a response for comparison with metadata sorted. Order within a
// metadata slice is not part of the agreed contract (values and dedup are), and the
// differ reports order-only differences separately rather than failing on them.
func canonical(t *testing.T, resp protocol.Response) string {
	t.Helper()
	resp.ID = ""
	if m := resp.Metadata; m != nil {
		for _, values := range [][]protocol.Text{m.Tables, m.Comments, m.Commands, m.Procedures} {
			sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		}
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(encoded)
}

type rustRunner struct {
	mu  sync.Mutex
	in  io.Writer
	out *bufio.Reader
	enc *json.Encoder
}

func (r *rustRunner) roundTrip(req protocol.Request) (protocol.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var resp protocol.Response
	if err := r.enc.Encode(req); err != nil {
		return resp, err
	}
	if f, ok := r.in.(interface{ Flush() error }); ok {
		if err := f.Flush(); err != nil {
			return resp, err
		}
	}
	line, err := r.out.ReadBytes('\n')
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(line, &resp)
}

// startRustRunner launches the Rust protocol runner once for the whole fuzz run and
// skips the test if it has not been built, so `go test ./...` stays green on a
// checkout where nobody has run cargo.
func startRustRunner(f *testing.F) *rustRunner {
	f.Helper()

	path := os.Getenv("SQLLEXER_RUST_RUNNER")
	if path == "" {
		path = filepath.Join("..", "..", "..", "rust", "target", "release", "sqllexer-runner")
	}
	if _, err := os.Stat(path); err != nil {
		f.Skipf("rust runner not built (%v); run: cd rust && cargo build --release", err)
	}

	// --stream: the runner answers each request before reading the next one, which
	// a request/response fuzzer needs and a bulk corpus replay does not.
	cmd := exec.Command(path, "--stream")
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		f.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		f.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})

	buffered := bufio.NewWriter(stdin)
	return &rustRunner{in: buffered, out: bufio.NewReaderSize(stdout, 1<<20), enc: json.NewEncoder(buffered)}
}
