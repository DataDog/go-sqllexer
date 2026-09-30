package sqllexer

import (
	"strings"
	"testing"
)

func TestNormalizerCastASAndAliases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "SELECT CAST(1 AS integer)", "SELECT CAST ( 1 AS integer )"},
		{"spaced and mixed case", "SELECT cAsT (1 aS integer)", "SELECT cAsT ( 1 aS integer )"},
		{"nested casts", "SELECT CAST(CAST(1 AS text) AS varchar(10))", "SELECT CAST ( CAST ( 1 AS text ) AS varchar ( 10 ) )"},
		{"deeply nested casts", "SELECT " + strings.Repeat("CAST(", 5) + "1" + strings.Repeat(" AS integer)", 5), "SELECT " + strings.Repeat("CAST ( ", 5) + "1" + strings.Repeat(" AS integer )", 5)},
		{"expression with nested alias", "SELECT CAST((SELECT x AS y FROM t) AS integer)", "SELECT CAST ( ( SELECT x FROM t ) AS integer )"},
		{"projection and table aliases", "SELECT CAST(1 AS integer) AS amount FROM t AS alias", "SELECT CAST ( 1 AS integer ) FROM t"},
		{"cast in CTE", "WITH c AS (SELECT CAST(1 AS integer) AS value) SELECT * FROM c AS alias", "WITH c AS ( SELECT CAST ( 1 AS integer ) ) SELECT * FROM c"},
		{"ordinary aliases", "SELECT x AS amount FROM t AS alias", "SELECT x FROM t"},
		{"cast as alias", "SELECT x AS cast FROM t", "SELECT x FROM t"},
	}
	normalizer := NewNormalizer(WithKeepSQLAlias(false))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, err := normalizer.Normalize(test.input, WithDBMS(DBMSPostgres))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("Normalize(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestNormalizerCastKeepAliases(t *testing.T) {
	input := "SELECT CAST(1 AS integer) AS amount FROM t AS alias"
	got, _, err := NewNormalizer(WithKeepSQLAlias(true)).Normalize(input, WithDBMS(DBMSPostgres))
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT CAST ( 1 AS integer ) AS amount FROM t AS alias"
	if got != want {
		t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
	}
}
