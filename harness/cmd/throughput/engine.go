package main

import (
	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// goEngine is one worker's view of the Go implementation: its own
// obfuscator/normalizer pair, reused for every statement, which is how a caller
// under sustained load is expected to hold them.
type goEngine struct {
	obfuscator *sqllexer.Obfuscator
	normalizer *sqllexer.Normalizer
}

func newEngine() *goEngine {
	return &goEngine{obfuscator: defaultObfuscator(), normalizer: defaultNormalizer()}
}

// process runs one statement. Output size is returned so the caller can keep the
// compiler and the allocator honest; errors are counted, not fatal.
func (e *goEngine) process(sql string, dbms sqllexer.DBMSType) (int, error) {
	out, metadata, err := sqllexer.ObfuscateAndNormalize(sql, e.obfuscator, e.normalizer, sqllexer.WithDBMS(dbms))
	if err != nil {
		return 0, err
	}
	return len(out) + metadata.Size, nil
}

// The benchmark uses the harness's fixture defaults so that both implementations
// do the same amount of work per statement.
func defaultObfuscator() *sqllexer.Obfuscator {
	cfg := protocol.DefaultObfuscatorConfig()
	return sqllexer.NewObfuscator(
		sqllexer.WithDollarQuotedFunc(cfg.DollarQuotedFunc),
		sqllexer.WithReplaceDigits(cfg.ReplaceDigits),
		sqllexer.WithReplacePositionalParameter(cfg.ReplacePositionalParameter),
		sqllexer.WithReplaceBoolean(cfg.ReplaceBoolean),
		sqllexer.WithReplaceNull(cfg.ReplaceNull),
		sqllexer.WithKeepJsonPath(cfg.KeepJsonPath),
		sqllexer.WithReplaceBindParameter(cfg.ReplaceBindParameter),
	)
}

func defaultNormalizer() *sqllexer.Normalizer {
	cfg := protocol.DefaultNormalizerConfig()
	return sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(cfg.CollectTables),
		sqllexer.WithCollectCommands(cfg.CollectCommands),
		sqllexer.WithCollectComments(cfg.CollectComments),
		sqllexer.WithCollectProcedures(cfg.CollectProcedure),
		sqllexer.WithKeepSQLAlias(cfg.KeepSQLAlias),
		sqllexer.WithUppercaseKeywords(cfg.UppercaseKeywords),
		sqllexer.WithRemoveSpaceBetweenParentheses(cfg.RemoveSpaceBetweenParentheses),
		sqllexer.WithKeepTrailingSemicolon(cfg.KeepTrailingSemicolon),
		sqllexer.WithKeepIdentifierQuotation(cfg.KeepIdentifierQuotation),
	)
}
