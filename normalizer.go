package sqllexer

import (
	"strings"
)

type normalizerConfig struct {
	// CollectTables specifies whether the normalizer should also extract the table names that a query addresses
	CollectTables bool

	// CollectCommands specifies whether the normalizer should extract and return commands as SQL metadata
	CollectCommands bool

	// CollectComments specifies whether the normalizer should extract and return comments as SQL metadata
	CollectComments bool

	// KeepSQLAlias reports whether SQL aliases ("AS") should be truncated.
	KeepSQLAlias bool
}

type normalizerOption func(*normalizerConfig)

func WithCollectTables(collectTables bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.CollectTables = collectTables
	}
}

func WithCollectCommands(collectCommands bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.CollectCommands = collectCommands
	}
}

func WithCollectComments(collectComments bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.CollectComments = collectComments
	}
}

func WithKeepSQLAlias(keepSQLAlias bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.KeepSQLAlias = keepSQLAlias
	}
}

type StatementMetadata struct {
	Tables   []string
	Comments []string
	Commands []string
}

type Normalizer struct {
	config *normalizerConfig
}

func NewNormalizer(opts ...normalizerOption) *Normalizer {
	normalizer := Normalizer{
		config: &normalizerConfig{},
	}

	for _, opt := range opts {
		opt(normalizer.config)
	}

	return &normalizer
}

const (
	ArrayPlaceholder   = "( ? )"
	BracketPlaceholder = "[ ? ]"
)

// Normalize takes an input SQL string and returns a normalized SQL string, a StatementMetadata struct, and an error.
// The normalizer collapses input SQL into compact format, groups obfuscated values into single placeholder,
// and collects metadata such as table names, comments, and commands.
func (n *Normalizer) Normalize(input string, lexerOpts ...lexerOption) (normalized string, info *StatementMetadata, err error) {
	lexer := New(
		input,
		lexerOpts...,
	)

	var normalizedSQL string
	var statementMetadata = &StatementMetadata{
		Tables:   []string{},
		Comments: []string{},
		Commands: []string{},
	}

	var lastToken *Token // The last token that is not whitespace or comment

	for token := range lexer.ScanAllTokens() {
		if token.Type == COMMENT || token.Type == MULTILINE_COMMENT {
			// Collect comments
			if n.config.CollectComments {
				statementMetadata.Comments = append(statementMetadata.Comments, token.Value)
			}
		} else if token.Type == IDENT {
			if isCommand(strings.ToUpper(token.Value)) && n.config.CollectCommands {
				// Collect commands
				statementMetadata.Commands = append(statementMetadata.Commands, strings.ToUpper(token.Value))
			} else if lastToken != nil && isTableIndicator(strings.ToUpper(lastToken.Value)) {
				// Collect table names
				if n.config.CollectTables {
					statementMetadata.Tables = append(statementMetadata.Tables, token.Value)
				}
			}
		}

		normalizedSQL = normalizeSQL(token, lastToken, normalizedSQL)

		// TODO: We rely on the WS token to determine if we should add a whitespace
		// This is not ideal, as SQLs with slightly different formatting will NOT be normalized into single family
		// e.g. "SELECT * FROM table where id = ?" and "SELECT * FROM table where id= ?" will be normalized into different family
		if token.Type != WS && token.Type != COMMENT && token.Type != MULTILINE_COMMENT {
			lastToken = token
		}
	}

	// We use regex to group consecutive obfuscated values into single placeholder.
	// This is "less" performant than token by token processing,
	// but it is much simpler to implement and maintain.
	// The trade off made here is assuming normalization runs on backend
	// where performance is not as critical as the agent.
	normalizedSQL = groupObfuscatedValues(normalizedSQL)
	if !n.config.KeepSQLAlias {
		normalizedSQL = DiscardSQLAlias(normalizedSQL)
	}

	// Dedupe collected metadata
	dedupeStatementMetadata(statementMetadata)

	return strings.TrimSpace(normalizedSQL), statementMetadata, nil
}

func normalizeSQL(token *Token, lastToken *Token, statement string) string {
	if token.Type == WS || token.Type == COMMENT || token.Type == MULTILINE_COMMENT {
		// We don't rely on the WS token to determine if we should add a whitespace
		return statement
	}

	// determine if we should add a whitespace
	statement = appendWhitespace(lastToken, token, statement)

	// UPPER CASE SQL keywords
	if isSQLKeyword(token) {
		statement += strings.ToUpper(token.Value)
		return statement
	}
	statement += token.Value

	return statement
}

// groupObfuscatedValues groups consecutive obfuscated values in a SQL query into a single placeholder.
// It replaces "(?, ?, ...)" and "[?, ?, ...]" with "( ? )" and "[ ? ]", respectively.
// Returns the modified SQL query as a string.
func groupObfuscatedValues(input string) string {
	grouped := groupableRegex.ReplaceAllStringFunc(input, func(match string) string {
		if match[0] == '(' {
			return ArrayPlaceholder
		}
		return BracketPlaceholder
	})
	return grouped
}

// DiscardSQLAlias removes any SQL alias from the input string and returns the modified string.
// It uses a regular expression to match the alias pattern and replace it with an empty string.
// The function is case-insensitive and matches the pattern "AS <alias_name>".
// The input string is not modified in place.
func DiscardSQLAlias(input string) string {
	return sqlAliasRegex.ReplaceAllString(input, "")
}

func dedupeCollectedMetadata(metadata []string) []string {
	// Dedupe collected metadata
	// e.g. [SELECT, JOIN, SELECT, JOIN] -> [SELECT, JOIN]
	//
	var dedupedMetadata = []string{}
	var metadataSeen = make(map[string]struct{})
	for _, m := range metadata {
		if _, seen := metadataSeen[m]; !seen {
			metadataSeen[m] = struct{}{}
			dedupedMetadata = append(dedupedMetadata, m)
		}
	}
	return dedupedMetadata
}

func dedupeStatementMetadata(info *StatementMetadata) {
	info.Tables = dedupeCollectedMetadata(info.Tables)
	info.Comments = dedupeCollectedMetadata(info.Comments)
	info.Commands = dedupeCollectedMetadata(info.Commands)
}

func isSQLKeyword(token *Token) bool {
	return token.Type == IDENT && keywordsRegex.MatchString(token.Value)
}

func appendWhitespace(lastToken *Token, token *Token, normalizedSQL string) string {
	switch token.Value {
	case ",":
	case "=":
		if lastToken != nil && lastToken.Value == ":" {
			// do not add a space before an equals if a colon was
			// present before it.
			break
		}
		fallthrough
	default:
		normalizedSQL += " "
	}

	return normalizedSQL
}
