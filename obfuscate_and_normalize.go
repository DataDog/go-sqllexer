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
		Tables:   []string{},
		Comments: []string{},
		Commands: []string{},
	}

	var lastToken Token // The last token that is not whitespace or comment

	for _, token := range lexer.ScanAll() {
		obfuscatedToken := Token{Type: token.Type, Value: obfuscator.ObfuscateTokenValue(token, lexerOpts...)}
		normalizer.collectMetadata(obfuscatedToken, lastToken, statementMetadata)
		lastToken = normalizer.normalizeSQL(obfuscatedToken, lastToken, &normalizedSQLBuilder)
	}

	normalizedSQL = normalizedSQLBuilder.String()

	normalizedSQL = groupObfuscatedValues(normalizedSQL)
	if !normalizer.config.KeepSQLAlias {
		normalizedSQL = discardSQLAlias(normalizedSQL)
	}

	// Dedupe collected metadata
	dedupeStatementMetadata(statementMetadata)

	return strings.TrimSpace(normalizedSQL), statementMetadata, nil
}
