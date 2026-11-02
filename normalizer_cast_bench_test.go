package sqllexer

import (
	"strings"
	"testing"
)

var normalizedCastBenchmarkResult string

func BenchmarkCastNormalization(b *testing.B) {
	body := "SELECT a.id, b.name, c.total FROM accounts a JOIN billing b ON a.id = b.account_id JOIN charges c ON c.bid = b.id WHERE a.status = 'active' AND c.total > 100 AND b.region IN ('us', 'eu')"
	queries := []struct {
		name  string
		query string
	}{
		{"NoCast", "SELECT id AS identifier FROM users AS u WHERE id = 42"},
		{"Simple", "SELECT CAST(? AS integer) FROM t"},
		{"MultiWordType", "SELECT CAST(created_at AS TIMESTAMP WITH TIME ZONE) AS created FROM events AS e"},
		{"Nested", "SELECT CAST(CAST(? AS text) AS varchar(10)) FROM t"},
		{"TryCast", "SELECT TRY_CAST(amount AS DECIMAL(10, 2)) AS amount FROM payments AS p"},
		{"SubqueryInCast", "SELECT CAST((SELECT max(id) AS m FROM t AS x) AS bigint) AS top FROM dual"},
		{"ManyCasts", "SELECT " + strings.TrimSuffix(strings.Repeat("CAST(col AS bigint) AS c, ", 10), ", ") + " FROM t"},
		{"NoCastLarge", body + " UNION ALL " + body},
	}
	normalizer := NewNormalizer(WithKeepSQLAlias(false))
	for _, benchmark := range queries {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(benchmark.query)))
			for i := 0; i < b.N; i++ {
				output, _, err := normalizer.Normalize(benchmark.query)
				if err != nil {
					b.Fatal(err)
				}
				normalizedCastBenchmarkResult = output
			}
		})
	}
}
