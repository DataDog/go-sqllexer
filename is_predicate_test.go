package sqllexer

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func checkISPredicateOutputs(t *testing.T, input string, obfuscator *Obfuscator, dbms DBMSType, wantObfuscated, wantNormalized string) {
	t.Helper()
	if got := obfuscator.Obfuscate(input, WithDBMS(dbms)); got != wantObfuscated {
		t.Errorf("Obfuscate: got %q, want %q", got, wantObfuscated)
	}
	got, _, err := ObfuscateAndNormalize(input, obfuscator, NewNormalizer(), WithDBMS(dbms))
	if err != nil {
		t.Fatal(err)
	}
	if got != wantNormalized {
		t.Errorf("ObfuscateAndNormalize: got %q, want %q", got, wantNormalized)
	}
}

// The off goldens were captured from origin/main before this option was added.
func TestKeepISPredicateGoldens(t *testing.T) {
	const input = "SELECT TRUE, NULL, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM TRUE, e IS NOT DISTINCT FROM NULL, TRUE = FALSE FROM t WHERE $1 IS TRUE"
	tests := []struct {
		name           string
		replaceBoolean bool
		replaceNull    bool
		off            string
		on             string
	}{
		{
			name: "neither",
			off:  "SELECT TRUE, NULL, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM TRUE, e IS NOT DISTINCT FROM NULL, TRUE = FALSE FROM t WHERE ? IS TRUE",
			on:   "SELECT TRUE, NULL, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM TRUE, e IS NOT DISTINCT FROM NULL, TRUE = FALSE FROM t WHERE ? IS TRUE",
		},
		{
			name:        "null",
			replaceNull: true,
			off:         "SELECT TRUE, ?, a IS TRUE, b IS NOT FALSE, c IS ?, d IS DISTINCT FROM TRUE, e IS NOT DISTINCT FROM ?, TRUE = FALSE FROM t WHERE ? IS TRUE",
			on:          "SELECT TRUE, ?, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM TRUE, e IS NOT DISTINCT FROM ?, TRUE = FALSE FROM t WHERE ? IS TRUE",
		},
		{
			name:           "boolean",
			replaceBoolean: true,
			off:            "SELECT ?, NULL, a IS ?, b IS NOT ?, c IS NULL, d IS DISTINCT FROM ?, e IS NOT DISTINCT FROM NULL, ? = ? FROM t WHERE ? IS ?",
			on:             "SELECT ?, NULL, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM ?, e IS NOT DISTINCT FROM NULL, ? = ? FROM t WHERE ? IS TRUE",
		},
		{
			name:           "both",
			replaceBoolean: true,
			replaceNull:    true,
			off:            "SELECT ?, ?, a IS ?, b IS NOT ?, c IS ?, d IS DISTINCT FROM ?, e IS NOT DISTINCT FROM ?, ? = ? FROM t WHERE ? IS ?",
			on:             "SELECT ?, ?, a IS TRUE, b IS NOT FALSE, c IS NULL, d IS DISTINCT FROM ?, e IS NOT DISTINCT FROM ?, ? = ? FROM t WHERE ? IS TRUE",
		},
	}

	for _, tt := range tests {
		for _, keep := range []bool{false, true} {
			t.Run(tt.name+"/keep="+strconv.FormatBool(keep), func(t *testing.T) {
				obfuscator := NewObfuscator(
					WithReplaceBoolean(tt.replaceBoolean),
					WithReplaceNull(tt.replaceNull),
					WithReplacePositionalParameter(true),
					WithKeepISPredicate(keep),
				)
				want := tt.off
				if keep {
					want = tt.on
				}
				checkISPredicateOutputs(t, input, obfuscator, DBMSPostgres, want, want)
			})
		}
	}

	// Omitting the option must have exactly the same behavior as setting false.
	checkISPredicateOutputs(t, input, NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithReplacePositionalParameter(true)), DBMSPostgres, tests[3].off, tests[3].off)
}

func TestKeepISPredicateForms(t *testing.T) {
	for _, prefix := range []string{"IS", "IS NOT"} {
		for _, literal := range []string{"TRUE", "FALSE", "NULL"} {
			input := "SELECT x " + prefix + " " + literal
			t.Run(prefix+"/"+literal, func(t *testing.T) {
				obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithKeepISPredicate(true))
				checkISPredicateOutputs(t, input, obfuscator, DBMSPostgres, input, input)
			})
		}
	}
}

func TestKeepISPredicateDoesNotChangeDistinctFrom(t *testing.T) {
	for _, prefix := range []string{"IS DISTINCT FROM", "IS NOT DISTINCT FROM"} {
		for _, literal := range []string{"TRUE", "FALSE", "NULL"} {
			input := "SELECT x " + prefix + " " + literal
			want := "SELECT x " + prefix + " ?"
			t.Run(prefix+"/"+literal, func(t *testing.T) {
				obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithKeepISPredicate(true))
				checkISPredicateOutputs(t, input, obfuscator, DBMSPostgres, want, want)
			})
		}
	}
}

func TestKeepISPredicateTokenBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		obfuscated string
		normalized string
	}{
		{
			name:       "casts aliases JSON and invalid bind",
			input:      "SELECT (x::boolean) IS /* a */ tRuE AS ok, NULL IS NOT FALSE, data ? 'IS TRUE' FROM t WHERE $1 IS TRUE AND x IS $1 AND x IS DISTINCT FROM 42 AND x IS NOT UNKNOWN",
			obfuscated: "SELECT (x::boolean) IS /* a */ tRuE AS ok, ? IS NOT FALSE, data ? ? FROM t WHERE ? IS TRUE AND x IS ? AND x IS DISTINCT FROM ? AND x IS NOT UNKNOWN",
			normalized: "SELECT ( x :: boolean ) IS tRuE, ? IS NOT FALSE, data ? ? FROM t WHERE ? IS TRUE AND x IS ? AND x IS DISTINCT FROM ? AND x IS NOT UNKNOWN",
		},
		{
			name:       "comments strings and quoted identifiers",
			input:      `SELECT x IS /* NOT TRUE */ TRUE, x IS NOT /* IS TRUE */ DISTINCT /*hi*/ FROM FALSE, x IS DISTINCT FROM NULL, 'x IS TRUE', "IS TRUE" FROM t`,
			obfuscated: `SELECT x IS /* NOT TRUE */ TRUE, x IS NOT /* IS TRUE */ DISTINCT /*hi*/ FROM ?, x IS DISTINCT FROM ?, ?, "IS TRUE" FROM t`,
			normalized: `SELECT x IS TRUE, x IS NOT DISTINCT FROM ?, x IS DISTINCT FROM ?, ?, "IS TRUE" FROM t`,
		},
		{
			name:       "binds and nonboolean distinct operand",
			input:      "SELECT $1 IS TRUE, x IS $1, x IS DISTINCT FROM $2, x IS DISTINCT FROM 42, x IS NOT UNKNOWN",
			obfuscated: "SELECT ? IS TRUE, x IS ?, x IS DISTINCT FROM ?, x IS DISTINCT FROM ?, x IS NOT UNKNOWN",
			normalized: "SELECT ? IS TRUE, x IS ?, x IS DISTINCT FROM ?, x IS DISTINCT FROM ?, x IS NOT UNKNOWN",
		},
		{
			name:       "ordinary values",
			input:      "VALUES (TRUE, FALSE, NULL)",
			obfuscated: "VALUES (?, ?, ?)",
			normalized: "VALUES ( ? )",
		},
		{
			name:       "nonboolean left operands",
			input:      "SELECT 42 IS NULL, 'value' IS NOT NULL",
			obfuscated: "SELECT ? IS NULL, ? IS NOT NULL",
			normalized: "SELECT ? IS NULL, ? IS NOT NULL",
		},
		{
			name:       "case line comments and nested parentheses",
			input:      "SELECT (x iS -- comment IS NOT NULL\n tRuE) IS FALSE, y Is NoT DiStInCt -- comment FROM TRUE\n FrOm nUlL, TRUE::boolean IS FALSE",
			obfuscated: "SELECT (x iS -- comment IS NOT NULL\n tRuE) IS FALSE, y Is NoT DiStInCt -- comment FROM TRUE\n FrOm ?, ?::boolean IS FALSE",
			normalized: "SELECT ( x iS tRuE ) IS FALSE, y Is NoT DiStInCt FrOm ?, ? :: boolean IS FALSE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithReplacePositionalParameter(true), WithKeepISPredicate(true))
			checkISPredicateOutputs(t, tt.input, obfuscator, DBMSPostgres, tt.obfuscated, tt.normalized)
		})
	}
}

// These representative texts were observed on PostgreSQL 18.6 with
// pg_stat_statements enabled. Excluding DISTINCT FROM keeps these pairs aligned.
func TestKeepISPredicatePgStatStatementsRepresentative(t *testing.T) {
	obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithReplacePositionalParameter(true), WithKeepISPredicate(true))
	checkISPredicateOutputs(t, "SELECT TRUE IS DISTINCT FROM FALSE", obfuscator, DBMSPostgres, "SELECT ? IS DISTINCT FROM ?", "SELECT ? IS DISTINCT FROM ?")
	checkISPredicateOutputs(t, "SELECT $1 IS DISTINCT FROM $2", obfuscator, DBMSPostgres, "SELECT ? IS DISTINCT FROM ?", "SELECT ? IS DISTINCT FROM ?")
	checkISPredicateOutputs(t, "SELECT TRUE IS NOT DISTINCT FROM FALSE", obfuscator, DBMSPostgres, "SELECT ? IS NOT DISTINCT FROM ?", "SELECT ? IS NOT DISTINCT FROM ?")
	checkISPredicateOutputs(t, "SELECT $1 IS NOT DISTINCT FROM $2", obfuscator, DBMSPostgres, "SELECT ? IS NOT DISTINCT FROM ?", "SELECT ? IS NOT DISTINCT FROM ?")
	checkISPredicateOutputs(t, "SELECT TRUE IS TRUE", obfuscator, DBMSPostgres, "SELECT ? IS TRUE", "SELECT ? IS TRUE")
	checkISPredicateOutputs(t, "SELECT $1 IS TRUE", obfuscator, DBMSPostgres, "SELECT ? IS TRUE", "SELECT ? IS TRUE")
}

func TestKeepISPredicateOnlyPostgres(t *testing.T) {
	const input = "SELECT TRUE, x IS TRUE, x IS NOT DISTINCT FROM NULL"
	const want = "SELECT ?, x IS ?, x IS NOT DISTINCT FROM ?"
	for _, dbms := range []DBMSType{DBMSMySQL, DBMSOracle, DBMSSQLServer, DBMSSnowflake, ""} {
		t.Run(string(dbms), func(t *testing.T) {
			obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithKeepISPredicate(true))
			checkISPredicateOutputs(t, input, obfuscator, dbms, want, want)
		})
	}
	checkISPredicateOutputs(t, input, NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithKeepISPredicate(true)), DBMSPostgresAlias1, "SELECT ?, x IS TRUE, x IS NOT DISTINCT FROM ?", "SELECT ?, x IS TRUE, x IS NOT DISTINCT FROM ?")
}

func TestKeepISPredicateConfigJSON(t *testing.T) {
	got, err := json.Marshal(NewObfuscator(WithKeepISPredicate(true)).config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"keep_is_predicate":true`) {
		t.Fatalf("missing keep_is_predicate in %s", got)
	}
	if NewObfuscator().config.KeepISPredicate {
		t.Fatal("KeepISPredicate must default to false")
	}
}
