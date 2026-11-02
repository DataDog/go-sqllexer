package sqllexer

import (
	"strings"
	"testing"
)

var normalizedCTEBenchmarkResult string

func BenchmarkCTENormalization(b *testing.B) {
	body := "SELECT a.id, b.name, c.total FROM accounts a JOIN billing b ON a.id = b.account_id JOIN charges c ON c.bid = b.id WHERE a.status = 'active' AND c.total > 100 AND b.region IN ('us', 'eu')"
	queries := []struct {
		name  string
		query string
	}{
		{"NoCTE", "SELECT id AS identifier FROM users AS u WHERE id = 42"},
		{"Plain", "WITH c AS (SELECT 1) SELECT * FROM c"},
		{"Materialized", "WITH c AS MATERIALIZED (SELECT 1 AS value) SELECT * FROM c AS alias"},
		{"NotMaterialized", "WITH c AS NOT MATERIALIZED (SELECT 1) SELECT * FROM c"},
		{"Nested", "WITH c AS (WITH d AS (SELECT 1) SELECT * FROM d) SELECT * FROM c"},
		{"DeepNested", strings.Repeat("WITH c AS (", 5) + "SELECT 1" + strings.Repeat(") SELECT * FROM c", 5)},
		{"DeepNested8", strings.Repeat("WITH c AS (", 8) + "SELECT 1" + strings.Repeat(") SELECT * FROM c", 8)},
		{"LargeBody", "WITH c AS MATERIALIZED (" + body + ") SELECT * FROM c"},
		{"ManyCTEs", "WITH " + strings.TrimSuffix(strings.Repeat("c AS ("+body+"), ", 4), ", ") + " SELECT * FROM c"},
		{"NoCTELarge", body + " UNION ALL " + body},
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
				normalizedCTEBenchmarkResult = output
			}
		})
	}
}
