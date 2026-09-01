//go:build rustffi

package rustffi_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/internal/corpus"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

// The configuration the fixture suite uses, which is also the one the tracer
// ships; parity is checked against the Go implementation configured identically.
const (
	fixtureObfuscatorFlags = rustffi.DollarQuotedFunc | rustffi.ReplaceDigits |
		rustffi.ReplacePositionalParameter | rustffi.ReplaceBoolean | rustffi.ReplaceNull
	fixtureNormalizerFlags = rustffi.CollectTables | rustffi.CollectCommands |
		rustffi.CollectComments | rustffi.CollectProcedure
)

func goImplementation() (*sqllexer.Obfuscator, *sqllexer.Normalizer) {
	obfuscator := sqllexer.NewObfuscator(
		sqllexer.WithDollarQuotedFunc(true),
		sqllexer.WithReplaceDigits(true),
		sqllexer.WithReplacePositionalParameter(true),
		sqllexer.WithReplaceBoolean(true),
		sqllexer.WithReplaceNull(true),
	)
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
		sqllexer.WithCollectCommands(true),
		sqllexer.WithCollectComments(true),
		sqllexer.WithCollectProcedures(true),
	)
	return obfuscator, normalizer
}

func assertParity(t *testing.T, processor *rustffi.Processor, sql string, dbms sqllexer.DBMSType) {
	t.Helper()
	obfuscator, normalizer := goImplementation()

	wantSQL, wantMetadata, wantErr := sqllexer.ObfuscateAndNormalize(sql, obfuscator, normalizer, sqllexer.WithDBMS(dbms))
	gotSQL, gotMetadata, gotErr := processor.ObfuscateAndNormalize(sql, dbms)

	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("error mismatch for %q (dbms %q): go=%v rust=%v", sql, dbms, wantErr, gotErr)
	}
	if wantErr != nil {
		return
	}
	if gotSQL != wantSQL {
		t.Fatalf("sql mismatch for %q (dbms %q):\n go   %q\n rust %q", sql, dbms, wantSQL, gotSQL)
	}
	if gotMetadata.Size != wantMetadata.Size {
		t.Fatalf("metadata size mismatch for %q: go=%d rust=%d", sql, wantMetadata.Size, gotMetadata.Size)
	}
	for _, field := range []struct {
		name string
		want []string
		got  []string
	}{
		{"tables", wantMetadata.Tables, gotMetadata.Tables},
		{"comments", wantMetadata.Comments, gotMetadata.Comments},
		{"commands", wantMetadata.Commands, gotMetadata.Commands},
		{"procedures", wantMetadata.Procedures, gotMetadata.Procedures},
	} {
		if strings.Join(field.want, "\x00") != strings.Join(field.got, "\x00") {
			t.Fatalf("%s mismatch for %q:\n go   %q\n rust %q", field.name, sql, field.want, field.got)
		}
	}
}

func TestParityAgainstFixtures(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	if processor == nil {
		t.Fatal("NewProcessor returned nil")
	}
	defer processor.Close()

	// The fixture SQL, reached through the shared corpus so no existing test file
	// has to change. Regenerate with harness/cmd/corpusgen.
	corpusPath := filepath.Join("..", "corpus", "testdata.jsonl")
	requests, err := corpus.Read(corpusPath)
	if err != nil {
		t.Skipf("corpus not generated (%v); run harness/cmd/corpusgen", err)
	}
	seen := 0
	for _, req := range requests {
		if req.Mode != protocol.ModeObfuscateAndNormalize {
			continue
		}
		if req.Obfuscator == nil || *req.Obfuscator != protocol.FixtureObfuscatorConfig() {
			continue
		}
		if req.Normalizer == nil || *req.Normalizer != protocol.FixtureNormalizerConfig() {
			continue
		}
		seen++
		assertParity(t, processor, string(req.SQL), sqllexer.DBMSType(req.DBMS))
	}
	if seen == 0 {
		t.Fatal("no fixture requests matched the shipped configuration")
	}
	t.Logf("verified %d fixtures through cgo", seen)
}

func TestParityOnEdgeCases(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	for _, dbms := range []sqllexer.DBMSType{
		"", sqllexer.DBMSPostgres, sqllexer.DBMSMySQL, sqllexer.DBMSSQLServer,
		sqllexer.DBMSOracle, sqllexer.DBMSSnowflake, "sql-server", "postgres",
	} {
		for _, sql := range []string{
			"",
			"   ",
			"SELECT 1",
			"SELECT * FROM users WHERE id = 42 AND b = 'x'",
			"/* only a comment */",
			"SELECT 'unterminated",
			"SELECT $func$ SELECT 1 $func$",
			"SELECT \"a\", `b`, [c], @d, :e, $1, ?",
			"WITH cte AS (SELECT 1) SELECT * FROM cte JOIN t ON 1=1",
			"SELECT \xff\xfe\x00 FROM t",
			"sélect 😀 from tëst",
			strings.Repeat("SELECT * FROM t WHERE id IN (1,2,3); ", 50),
		} {
			assertParity(t, processor, sql, dbms)
		}
	}
}

func TestClosedProcessorReportsAnError(t *testing.T) {
	processor := rustffi.NewProcessor(0, 0)
	processor.Close()
	processor.Close() // idempotent
	if _, _, err := processor.ObfuscateAndNormalize("SELECT 1", ""); err == nil {
		t.Fatal("expected an error from a closed processor")
	}
}

func TestResultsSurviveSubsequentCalls(t *testing.T) {
	// Rust hands back slices into its own buffers; the binding must copy them, or
	// the second call would corrupt the first result.
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	first, firstMetadata, err := processor.ObfuscateAndNormalize("SELECT * FROM first_table", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := processor.ObfuscateAndNormalize("SELECT * FROM another_table WHERE x = 1", ""); err != nil {
		t.Fatal(err)
	}
	if first != "SELECT * FROM first_table" {
		t.Fatalf("first result was corrupted: %q", first)
	}
	if len(firstMetadata.Tables) != 1 || firstMetadata.Tables[0] != "first_table" {
		t.Fatalf("first metadata was corrupted: %q", firstMetadata.Tables)
	}
}

// FuzzParity is the continuous differential check: same input, same config, two
// implementations, in one process. Seeds come from the shared corpus so a nightly
// run starts from the same interesting inputs the differ uses.
func FuzzParity(f *testing.F) {
	for _, seed := range []string{
		"SELECT * FROM users WHERE id = 42",
		"SELECT $func$SELECT 1$func$",
		"/* c */ SELECT a FROM b -- x",
		"SELECT 'a' || \"b\" || `c`",
		"@@version :bind $1 ?",
		"WITH a AS (SELECT 1) SELECT * FROM a",
		"\xff\xfe",
		"SELECT 1;;;",
	} {
		for _, dbms := range []string{"", "postgresql", "mysql", "mssql", "oracle", "snowflake"} {
			f.Add(seed, dbms)
		}
	}
	if requests, err := corpus.Read(filepath.Join("..", "corpus", "fuzzseeds.jsonl")); err == nil {
		for _, req := range requests {
			f.Add(string(req.SQL), req.DBMS)
		}
	}

	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	f.Cleanup(processor.Close)

	f.Fuzz(func(t *testing.T, sql string, dbms string) {
		assertParity(t, processor, sql, sqllexer.DBMSType(dbms))
	})
}
