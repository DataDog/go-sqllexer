package sqllexer

import (
	"strings"
	"testing"
)

var normalizedCTEBenchmarkResult string

func BenchmarkCTENormalization(b *testing.B) {
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
