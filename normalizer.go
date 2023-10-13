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

	// UppercaseKeywords reports whether SQL keywords should be uppercased.
	UppercaseKeywords bool
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

func WithUppercaseKeywords(uppercaseKeywords bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.UppercaseKeywords = uppercaseKeywords
	}
}

type StatementMetadata struct {
	Size     int
	Tables   []string
	Comments []string
	Commands []string
}

type GroupablePlaceholder struct {
	groupable bool
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

// Normalize takes an input SQL string and returns a normalized SQL string, a StatementMetadata struct, and an error.
// The normalizer collapses input SQL into compact format, groups obfuscated values into single placeholder,
// and collects metadata such as table names, comments, and commands.
func (n *Normalizer) Normalize(input string, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
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
	var groupablePlaceholder GroupablePlaceholder

	for _, token := range lexer.ScanAll() {
		n.collectMetadata(&token, &lastToken, statementMetadata)
		n.normalizeSQL(&token, &lastToken, &normalizedSQLBuilder, &groupablePlaceholder)
	}

	normalizedSQL = normalizedSQLBuilder.String()

	// Dedupe collected metadata
	dedupeStatementMetadata(statementMetadata)

	return strings.TrimSpace(normalizedSQL), statementMetadata, nil
}

func (n *Normalizer) collectMetadata(token *Token, lastToken *Token, statementMetadata *StatementMetadata) {
	if n.config.CollectComments && (token.Type == COMMENT || token.Type == MULTILINE_COMMENT) {
		// Collect comments
		statementMetadata.Comments = append(statementMetadata.Comments, token.Value)
	} else if token.Type == IDENT {
		if n.config.CollectCommands && isCommand(strings.ToUpper(token.Value)) {
			// Collect commands
			statementMetadata.Commands = append(statementMetadata.Commands, strings.ToUpper(token.Value))
		} else if n.config.CollectTables && isTableIndicator(strings.ToUpper(lastToken.Value)) {
			// Collect table names
			statementMetadata.Tables = append(statementMetadata.Tables, token.Value)
		}
	}
}

func (n *Normalizer) normalizeSQL(token *Token, lastToken *Token, normalizedSQLBuilder *strings.Builder, groupablePlaceholder *GroupablePlaceholder) {
	if token.Type != WS && token.Type != COMMENT && token.Type != MULTILINE_COMMENT {
		if !n.config.KeepSQLAlias {
			// discard SQL alias
			if strings.ToUpper(token.Value) == "AS" {
				*lastToken = *token
				return
			}

			if strings.ToUpper(lastToken.Value) == "AS" {
				if token.Type == IDENT {
					*lastToken = *token
					return
				} else {
					appendWhitespace(lastToken, token, normalizedSQLBuilder)
					n.writeToken(lastToken, normalizedSQLBuilder)
				}
			}
		}

		// group consecutive obfuscated values into single placeholder
		if n.isObfuscatedValueGroupable(token, lastToken, groupablePlaceholder) {
			// return the token but not write it to the normalizedSQLBuilder
			*lastToken = *token
			return
		}

		// determine if we should add a whitespace
		appendWhitespace(lastToken, token, normalizedSQLBuilder)
		n.writeToken(token, normalizedSQLBuilder)

		*lastToken = *token
	}
}

func (n *Normalizer) writeToken(token *Token, normalizedSQLBuilder *strings.Builder) {
	if n.config.UppercaseKeywords && isSQLKeyword(token) {
		normalizedSQLBuilder.WriteString(strings.ToUpper(token.Value))
	} else {
		normalizedSQLBuilder.WriteString(token.Value)
	}
}

func (n *Normalizer) isObfuscatedValueGroupable(token *Token, lastToken *Token, groupablePlaceholder *GroupablePlaceholder) bool {
	if token.Value == NumberPlaceholder || token.Value == StringPlaceholder {
		if lastToken.Value == "(" || lastToken.Value == "[" {
			groupablePlaceholder.groupable = true
		} else if lastToken.Value == "," && groupablePlaceholder.groupable {
			return true
		}
	}

	if (lastToken.Value == NumberPlaceholder || lastToken.Value == StringPlaceholder) && token.Value == "," && groupablePlaceholder.groupable {
		return true
	}

	if groupablePlaceholder.groupable && (token.Value == ")" || token.Value == "]") {
		groupablePlaceholder.groupable = false
	}

	return false
}

func dedupeCollectedMetadata(metadata []string) (dedupedMetadata []string, size int) {
	// Dedupe collected metadata
	// e.g. [SELECT, JOIN, SELECT, JOIN] -> [SELECT, JOIN]
	dedupedMetadata = []string{}
	var metadataSeen = make(map[string]struct{})
	for _, m := range metadata {
		if _, seen := metadataSeen[m]; !seen {
			metadataSeen[m] = struct{}{}
			dedupedMetadata = append(dedupedMetadata, m)
			size += len(m)
		}
	}
	return dedupedMetadata, size
}

func dedupeStatementMetadata(info *StatementMetadata) {
	var tablesSize, commentsSize, commandsSize int
	info.Tables, tablesSize = dedupeCollectedMetadata(info.Tables)
	info.Comments, commentsSize = dedupeCollectedMetadata(info.Comments)
	info.Commands, commandsSize = dedupeCollectedMetadata(info.Commands)
	info.Size += tablesSize + commentsSize + commandsSize
}

func appendWhitespace(lastToken *Token, token *Token, normalizedSQLBuilder *strings.Builder) {
	switch token.Value {
	case ",":
	case "=":
		if lastToken.Value == ":" {
			// do not add a space before an equals if a colon was
			// present before it.
			break
		}
		fallthrough
	default:
		normalizedSQLBuilder.WriteString(" ")
	}
}
