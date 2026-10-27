package sqllexer

import "testing"

var isPredicateBenchResult string

func BenchmarkKeepISPredicate(b *testing.B) {
	queries := []struct {
		name string
		sql  string
	}{
		{"no_predicate", "SELECT id, flag FROM events WHERE id = 42 AND status = 'ready'"},
		{"one_predicate", "SELECT id FROM events WHERE enabled IS TRUE"},
		{"mixed", "SELECT TRUE, e.enabled IS /* signal */ TRUE, e.deleted_at IS NOT NULL, e.payload ? 'flag' FROM events e WHERE e.id = $1 AND e.flag IS DISTINCT FROM FALSE"},
	}
	lexerOpt := WithDBMS(DBMSPostgres)
	normalizer := NewNormalizer()
	for _, query := range queries {
		for _, keep := range []struct {
			name  string
			value bool
		}{{"off", false}, {"on", true}} {
			obfuscator := NewObfuscator(WithReplaceBoolean(true), WithReplaceNull(true), WithKeepISPredicate(keep.value))
			b.Run("Obfuscate/"+query.name+"/"+keep.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					isPredicateBenchResult = obfuscator.Obfuscate(query.sql, lexerOpt)
				}
			})
			b.Run("ObfuscateAndNormalize/"+query.name+"/"+keep.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var err error
					isPredicateBenchResult, _, err = ObfuscateAndNormalize(query.sql, obfuscator, normalizer, lexerOpt)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
