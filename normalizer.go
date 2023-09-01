package sqllexer

import (
	"regexp"
	"strings"
)

type SQLNormalizerConfig struct {
	DBMS string `json:"dbms"`

	// CollectTables specifies whether the normalizer should also extract the table names that a query addresses
	CollectTables bool `json:"collect_tables"`

	// CollectCommands specifies whether the normalizer should extract and return commands as SQL metadata
	CollectCommands bool `json:"collect_commands"`

	// CollectComments specifies whether the normalizer should extract and return comments as SQL metadata
	CollectComments bool `json:"collect_comments"`

	// KeepSQLAlias reports whether SQL aliases ("AS") should be truncated.
	KeepSQLAlias bool `json:"keep_sql_alias"`
}

type NormalizedInfo struct {
	Tables   []string
	Comments []string
	Commands []string
}

type SQLNormalizer struct {
	config *SQLNormalizerConfig
}

func NewSQLNormalizer(config *SQLNormalizerConfig) *SQLNormalizer {
	return &SQLNormalizer{config: config}
}

const (
	ArrayPlaceholder   = "( ? )"
	BracketPlaceholder = "[ ? ]"
)

// Normalize takes an input SQL string and returns a normalized SQL string, a NormalizedInfo struct, and an error.
// The normalizer collapses input SQL into compact format, groups obfuscated values into single placeholder,
// and collects metadata such as table names, comments, and commands.
func (n *SQLNormalizer) Normalize(input string) (string, *NormalizedInfo, error) {
	lexer := NewSQLLexer(input)

	var normalizedSQL string
	var normalizedInfo = &NormalizedInfo{
		Tables:   []string{},
		Comments: []string{},
		Commands: []string{},
	}

	var lastToken *Token // The last token that is not whitespace or comment

	for token := range lexer.ScanAllTokens() {
		if token.Type == COMMENT || token.Type == MULTILINE_COMMENT {
			// Collect comments
			if n.config.CollectComments {
				normalizedInfo.Comments = append(normalizedInfo.Comments, token.Value)
			}
		} else if token.Type == IDENT {
			if isCommand(strings.ToUpper(token.Value)) && n.config.CollectCommands {
				// Collect commands
				normalizedInfo.Commands = append(normalizedInfo.Commands, strings.ToUpper(token.Value))
			} else if lastToken != nil && isTableIndicator(strings.ToUpper(lastToken.Value)) {
				// Collect table names
				if n.config.CollectTables {
					normalizedInfo.Tables = append(normalizedInfo.Tables, token.Value)
				}
			}
		}

		writeNormalizedSQL(token, lastToken, &normalizedSQL)

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
	dedupeNormalizedInfo(normalizedInfo)

	return strings.TrimSpace(normalizedSQL), normalizedInfo, nil
}

func writeNormalizedSQL(token *Token, lastToken *Token, normalizedSQL *string) {
	if token.Type == WS || token.Type == COMMENT || token.Type == MULTILINE_COMMENT {
		// We don't rely on the WS token to determine if we should add a whitespace
		return
	}

	// determine if we should add a whitespace
	writeWhitespace(lastToken, token, normalizedSQL)

	// UPPER CASE SQL keywords
	if isSQLKeyword(token) {
		*normalizedSQL += strings.ToUpper(token.Value)
		return
	}
	*normalizedSQL += token.Value
}

// groupObfuscatedValues groups consecutive obfuscated values in a SQL query into a single placeholder.
// It replaces "(?, ?, ...)" and "[?, ?, ...]" with "( ? )" and "[ ? ]", respectively.
// Returns the modified SQL query as a string.
func groupObfuscatedValues(input string) string {
	groupable_regex := regexp.MustCompile(`(\()\s*\?(?:\s*,\s*\?\s*)*\s*(\))|(\[)\s*\?(?:\s*,\s*\?\s*)*\s*(\])`)
	grouped := groupable_regex.ReplaceAllStringFunc(input, func(match string) string {
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
	return regexp.MustCompile(`(?i)\s+AS\s+[\w?]+`).ReplaceAllString(input, "")
}

func dedupeCollectedMetadata(metadata []string) []string {
	// Dedupe collected metadata
	// e.g. [SELECT, JOIN, SELECT, JOIN] -> [SELECT, JOIN]
	//
	var dedupedMetadata = []string{}
	var metadataSeen = make(map[string]bool)
	for _, m := range metadata {
		if _, seen := metadataSeen[m]; !seen {
			metadataSeen[m] = true
			dedupedMetadata = append(dedupedMetadata, m)
		}
	}
	return dedupedMetadata
}

func dedupeNormalizedInfo(info *NormalizedInfo) {
	info.Tables = dedupeCollectedMetadata(info.Tables)
	info.Comments = dedupeCollectedMetadata(info.Comments)
	info.Commands = dedupeCollectedMetadata(info.Commands)
}

func isSQLKeyword(token *Token) bool {
	return token.Type == IDENT && keywordsRegex.MatchString(token.Value)
}

func writeWhitespace(lastToken *Token, token *Token, normalizedSQL *string) {
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
		*normalizedSQL += " "
	}
}
