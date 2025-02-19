package sqllexer

import "strings"

// ObfuscateAndNormalize takes an input SQL string and returns an normalized SQL string with metadata
// This function is a convenience function that combines the Obfuscator and Normalizer in one pass
func ObfuscateAndNormalize(input string, obfuscator *Obfuscator, normalizer *Normalizer, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	lexer := New(input, lexerOpts...)
	normalizedSQLBuilder := new(strings.Builder)
	normalizedSQLBuilder.Grow(len(input))

	// Always allocate metadata for backward compatibility
	statementMetadata = statementMetadataPool.Get().(*StatementMetadata)
	statementMetadata.reset()
	defer statementMetadataPool.Put(statementMetadata)

	var groupablePlaceholder groupablePlaceholder
	var headState headState
	var ctes map[string]bool

	// Only allocate CTEs map if collecting tables
	if normalizer.config.CollectTables {
		ctes = make(map[string]bool, 2)
	}

	var lastValueToken *LastValueToken

	for {
		token := lexer.Scan()
		obfuscator.ObfuscateTokenValue(token, lastValueToken, lexerOpts...)
		if normalizer.shouldCollectMetadata() {
			normalizer.collectMetadata(token, lastValueToken, statementMetadata, ctes)
		}
		normalizer.normalizeSQL(token, lastValueToken, normalizedSQLBuilder, &groupablePlaceholder, &headState, lexerOpts...)
		if token.Type == EOF {
			break
		}
		if isValueToken(token) {
			lastValueToken = token.getLastValueToken()
		}
	}

	normalizedSQL = normalizedSQLBuilder.String()
	return normalizer.trimNormalizedSQL(normalizedSQL), statementMetadata, nil
}
