package sqllexer

import "testing"

var normalizedCastBenchmarkResult string

func BenchmarkCastNormalization(b *testing.B) {
	queries := []struct {
		name  string
		query string
	}{
		{"NoCast", "SELECT id AS identifier FROM users AS u WHERE id = ?"},
		{"Simple", "SELECT CAST(? AS integer) FROM t"},
		{"Nested", "SELECT CAST(CAST(? AS text) AS varchar(10)) FROM t"},
		{"Expression", "SELECT CAST(COALESCE(x, ?) + ? AS numeric(10,2)) AS amount FROM t AS alias"},
		{"CTEWithCast", "WITH c AS (SELECT CAST(? AS integer) AS value) SELECT * FROM c AS alias"},
	}
	normalizer := NewNormalizer(WithKeepSQLAlias(false))
	for _, benchmark := range queries {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(benchmark.query)))
			for i := 0; i < b.N; i++ {
				output, _, err := normalizer.Normalize(benchmark.query, WithDBMS(DBMSPostgres))
				if err != nil {
					b.Fatal(err)
				}
				normalizedCastBenchmarkResult = output
			}
		})
	}
}
