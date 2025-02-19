package sqllexer

import "strings"

// ObfuscateAndNormalize takes an input SQL string and returns an normalized SQL string with metadata
// This function is a convenience function that combines the Obfuscator and Normalizer in one pass
func ObfuscateAndNormalize(input string, obfuscator *Obfuscator, normalizer *Normalizer, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	lexer := New(
		input,
		lexerOpts...,
	)

	normalizedSQLBuilder := new(strings.Builder)
	normalizedSQLBuilder.Grow(len(input))

	statementMetadata = &StatementMetadata{
		Tables:     []string{},
		Comments:   []string{},
		Commands:   []string{},
		Procedures: []string{},
	}

	var groupablePlaceholder groupablePlaceholder
	var headState headState

	ctes := make(map[string]bool) // Holds the CTEs that are currently being processed

	var lastValueToken *Token

	for {
		token := lexer.Scan()
		obfuscator.ObfuscateTokenValue(token, lastValueToken, lexerOpts...)
		normalizer.collectMetadata(token, lastValueToken, statementMetadata, ctes)
		normalizer.normalizeSQL(token, lastValueToken, normalizedSQLBuilder, &groupablePlaceholder, &headState, lexerOpts...)
		if token.Type == EOF {
			break
		}
		if isValueToken(token) {
			lastValueToken = token
		}
	}

	normalizedSQL = normalizedSQLBuilder.String()

	// Dedupe collected metadata
	dedupeStatementMetadata(statementMetadata)

	return normalizer.trimNormalizedSQL(normalizedSQL), statementMetadata, nil
}
