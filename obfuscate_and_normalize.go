package sqllexer

import "strings"

// ObfuscateAndNormalize takes an input SQL string and returns an normalized SQL string with metadata
// This function is a convenience function that combines the Obfuscator and Normalizer in one pass
func ObfuscateAndNormalize(input string, obfuscator *Obfuscator, normalizer *Normalizer, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	lexer := New(
		input,
		lexerOpts...,
	)

	var normalizedSQLBuilder strings.Builder

	statementMetadata = &StatementMetadata{
		Tables:     []string{},
		Comments:   []string{},
		Commands:   []string{},
		Procedures: []string{},
	}

	var groupablePlaceholder groupablePlaceholder
	var headState headState

	ctes := make(map[string]bool) // Holds the CTEs that are currently being processed

	for {
		token := lexer.Scan()
		obfuscator.ObfuscateTokenValue(token, lexerOpts...)
		normalizer.collectMetadata(token, statementMetadata, ctes)
		normalizer.normalizeSQL(token, &normalizedSQLBuilder, &groupablePlaceholder, &headState, lexerOpts...)
		if token.Type == EOF {
			break
		}
	}

	normalizedSQL = normalizedSQLBuilder.String()

	// Dedupe collected metadata
	dedupeStatementMetadata(statementMetadata)

	return normalizer.trimNormalizedSQL(normalizedSQL), statementMetadata, nil
}
