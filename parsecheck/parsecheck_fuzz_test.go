// Package parsecheck fuzzes go-sqllexer's obfuscate-and-normalize output
// against a real Postgres parser.
//
// It lives in its own module so that go-sqllexer itself does not take on a
// cgo dependency (pg_query_go compiles the Postgres parser from C).
package parsecheck

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DataDog/go-sqllexer"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// newAgentPostgresPipeline mirrors how datadog-agent's pkg/obfuscate
// (ObfuscateWithSQLLexer) builds the obfuscator and normalizer for DBM
// Postgres query samples, using the agent's default SQLConfig values. That
// output is the query text dd-source later hands to pg_query.
func newAgentPostgresPipeline() (*sqllexer.Obfuscator, *sqllexer.Normalizer) {
	obfuscator := sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(false),
		sqllexer.WithDollarQuotedFunc(true),
		sqllexer.WithReplacePositionalParameter(true),
		sqllexer.WithReplaceBoolean(true),
		sqllexer.WithReplaceNull(true),
		sqllexer.WithKeepJsonPath(false),
		sqllexer.WithReplaceBindParameter(false),
	)
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectComments(true),
		sqllexer.WithCollectCommands(true),
		sqllexer.WithCollectTables(true),
		sqllexer.WithKeepSQLAlias(true),
		sqllexer.WithKeepIdentifierQuotation(false),
	)
	return obfuscator, normalizer
}

// fillPlaceholders turns obfuscated output back into something a parser can
// check. `IS ?` comes from obfuscating NULL/TRUE/FALSE and has no valid
// single-literal form, so it is rewritten the way dd-source's pgsyntax does.
// Other `?` become a string literal rather than dd-source's `$1`, because a
// string is accepted in more positions (`INTERVAL '1 day'` -> `INTERVAL ?`
// is only valid again with a string), so a failure here points at the lexer
// rather than at the placeholder choice.
var fillPlaceholders = strings.NewReplacer("IS NOT ?", "IS NOT TRUE", "IS ?", "IS TRUE", "?", "'?'")

// knownBugs are lexer/normalizer bugs this test has already found. A failing
// input that matches one is skipped so the fuzzer keeps looking for new
// classes of failure. Delete an entry once its bug is fixed.
var knownBugs = []struct {
	reason string
	match  func(input, output string) bool
}{
	{
		// `"my-table"` -> `my-table`, which Postgres reads as `my - table`.
		reason: "normalizer strips quotes from identifiers that need them",
		match:  inputOnly(parsesWithIdentifierQuotesKept),
	},
	{
		// `"a""b"` -> `"a" "b"`
		reason: "escaped quote inside a quoted identifier splits it in two",
		match:  inputOnly(func(input string) bool { return strings.Contains(input, `""`) }),
	},
	{
		// `VALUES (1 + a, 2)` -> `VALUES ( ? + a, )`; also `,,` and `, ]`
		reason: "collapsing a list of values drops the last one when an earlier item is an expression",
		match:  func(_, output string) bool { return droppedListItem.MatchString(output) },
	},
	{
		// `0009` -> `? ?`
		reason: "number with several leading zeros followed by 8 or 9 is split in two",
		match:  inputOnly(regexp.MustCompile(`(^|[^0-9A-Za-z_.])00[0-7]*[89]`).MatchString),
	},
	{
		// `a+1` -> `a ?`
		reason: "`+` or `-` before a number is taken as its sign even after an operand",
		match:  inputOnly(regexp.MustCompile(`[A-Za-z0-9_)\]"'$]\s*[+-]\.?[0-9]`).MatchString),
	},
	{
		// `.5` -> `. ?`
		reason: "number without a leading zero is lexed as `.` plus a number",
		match:  inputOnly(regexp.MustCompile(`(^|[^0-9])\.[0-9]`).MatchString),
	},
	{
		reason: "quoted identifier written directly after a word, with no space, is lexed incorrectly",
		match:  inputOnly(regexp.MustCompile(`[A-Za-z0-9_]"`).MatchString),
	},
	{
		// `*!0` -> `* ! ?`. Postgres allows user-defined operators made of any
		// run of operator characters, but the lexer only knows a fixed list.
		reason: "operator the lexer doesn't recognize is split into pieces",
		match:  inputOnly(hasAdjacentOperators),
	},
	{
		// `data#>>'{a}'` -> `data# >> ?`, `a!%0` -> `a! % ?`
		reason: "operator character written directly after a word is lexed as part of that word",
		match:  inputOnly(hasOperatorCharInIdentifier),
	},
	{
		// `E'abc'` -> `E ?`
		reason: "string prefix (E, B, X, N, U&) is left behind as a separate word",
		match:  inputOnly(regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_])(e|b|x|n|u&)'`).MatchString),
	},
}

var droppedListItem = regexp.MustCompile(`,\s*[)\],]`)

// opChars are the characters Postgres allows in operator names.
const opChars = "+-*/<>=~!@#%^&|`"

func inputOnly(match func(string) bool) func(string, string) bool {
	return func(input, _ string) bool { return match(input) }
}

func hasAdjacentOperators(input string) bool {
	isOp := func(v string) bool { return v != "" && strings.Trim(v, opChars) == "" }
	lexer := sqllexer.New(input, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	prev := ""
	for tok := lexer.Scan(); tok.Type != sqllexer.EOF; tok = lexer.Scan() {
		if isOp(prev) && isOp(tok.Value) {
			return true
		}
		prev = tok.Value
	}
	return false
}

func hasOperatorCharInIdentifier(input string) bool {
	lexer := sqllexer.New(input, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	for tok := lexer.Scan(); tok.Type != sqllexer.EOF; tok = lexer.Scan() {
		if tok.Type == sqllexer.IDENT && strings.ContainsAny(tok.Value, opChars) {
			return true
		}
	}
	return false
}

func parsesWithIdentifierQuotesKept(input string) bool {
	obfuscator, _ := newAgentPostgresPipeline()
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectComments(true),
		sqllexer.WithCollectCommands(true),
		sqllexer.WithCollectTables(true),
		sqllexer.WithKeepSQLAlias(true),
		sqllexer.WithKeepIdentifierQuotation(true),
	)
	out, _, err := sqllexer.ObfuscateAndNormalize(input, obfuscator, normalizer, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return false
	}
	_, err = pg_query.Parse(fillPlaceholders.Replace(out))
	return err == nil
}

func addPostgresSeeds(f *testing.F) {
	f.Helper()
	seeds := []string{
		`SELECT * FROM "my-table" WHERE id = 1`,
		`SELECT "user-id", "first name" FROM "public"."order-items" oi WHERE oi."user-id" = 42`,
		`UPDATE "Users" SET "Email" = 'a@b.c' WHERE "Id" = 7`,
		`SELECT * FROM users WHERE created_at > now() - INTERVAL '1 day' AND deleted_at IS NULL`,
		`SELECT data->>'name' FROM docs WHERE data @> '{"a":1}'::jsonb LIMIT 10`,
		`INSERT INTO t (a, b) VALUES ($1, $2) ON CONFLICT (a) DO UPDATE SET b = EXCLUDED.b`,
		`WITH x AS (SELECT 1 AS n) SELECT n FROM x WHERE n IN (1, 2, 3)`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Reuse the library's per-DBMS fixtures as realistic seed SQL.
	root := filepath.Join("..", "testdata", "postgresql")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tc struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(raw, &tc); err != nil {
			return err
		}
		f.Add(tc.Input)
		return nil
	})
	if err != nil {
		f.Fatalf("loading postgres fixtures: %v", err)
	}
}

// FuzzPostgresOutputParses checks that obfuscating and normalizing a query
// Postgres accepts still produces a query Postgres accepts.
func FuzzPostgresOutputParses(f *testing.F) {
	addPostgresSeeds(f)
	obfuscator, normalizer := newAgentPostgresPipeline()

	f.Fuzz(func(t *testing.T, input string) {
		// A `?` in the input (e.g. the jsonb `?` operator) cannot be told
		// apart from an obfuscation placeholder in the output.
		if strings.Contains(input, "?") {
			t.Skip()
		}
		// Only valid input can be expected to give valid output.
		if _, err := pg_query.Parse(input); err != nil {
			t.Skip()
		}

		out, _, err := sqllexer.ObfuscateAndNormalize(input, obfuscator, normalizer, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
		if err != nil {
			t.Fatalf("ObfuscateAndNormalize(%q): %v", input, err)
		}

		final := fillPlaceholders.Replace(out)
		if _, err := pg_query.Parse(final); err != nil {
			for _, bug := range knownBugs {
				if bug.match(input, out) {
					t.Skipf("known bug: %s", bug.reason)
				}
			}
			t.Fatalf("output no longer parses as Postgres\ninput:  %q\noutput: %q\nparsed: %q\nerror:  %v", input, out, final, err)
		}
	})
}
