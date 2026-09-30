package sqllexer

// ObfuscateAndNormalize takes an input SQL string and returns an normalized SQL string with metadata
// This function is a convenience function that combines the Obfuscator and Normalizer in one pass
func ObfuscateAndNormalize(input string, obfuscator *Obfuscator, normalizer *Normalizer, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	var ec extractContext
	var predicate isPredicateContext
	keepISPredicate := false
	if obfuscator.config.KeepISPredicate {
		lexerConfig := LexerConfig{}
		for _, opt := range lexerOpts {
			opt(&lexerConfig)
		}
		keepISPredicate = lexerConfig.DBMS == DBMSPostgres
	}
	obfuscate := func(token *Token, lastValueToken *LastValueToken) {
		if !keepISPredicate || !predicate.keep(token) {
			obfuscator.ObfuscateTokenValue(token, lastValueToken, lexerOpts...)
		}
		ec.maybeReplaceExtractField(token)
		ec.update(token)
	}
	return normalizer.normalize(input, obfuscate, lexerOpts...)
}
