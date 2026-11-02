package sqllexer

import (
	"fmt"
	"math/bits"
	"strings"
)

type normalizerConfig struct {
	// CollectTables specifies whether the normalizer should also extract the table names that a query addresses
	CollectTables bool `json:"collect_tables"`

	// CollectCommands specifies whether the normalizer should extract and return commands as SQL metadata
	CollectCommands bool `json:"collect_commands"`

	// CollectComments specifies whether the normalizer should extract and return comments as SQL metadata
	CollectComments bool `json:"collect_comments"`

	// CollectProcedure specifies whether the normalizer should extract and return procedure name as SQL metadata
	CollectProcedure bool `json:"collect_procedure"`

	// KeepSQLAlias specifies whether SQL aliases ("AS") should be truncated.
	KeepSQLAlias bool `json:"keep_sql_alias"`

	// UppercaseKeywords specifies whether SQL keywords should be uppercased.
	UppercaseKeywords bool `json:"uppercase_keywords"`

	// RemoveSpaceBetweenParentheses specifies whether spaces should be kept between parentheses.
	// Spaces are inserted between parentheses by default. but this can be disabled by setting this to true.
	RemoveSpaceBetweenParentheses bool `json:"remove_space_between_parentheses"`

	// KeepTrailingSemicolon specifies whether the normalizer should keep the trailing semicolon.
	// The trailing semicolon is removed by default, but this can be disabled by setting this to true.
	// PL/SQL requires a trailing semicolon, so this should be set to true when normalizing PL/SQL.
	KeepTrailingSemicolon bool `json:"keep_trailing_semicolon"`

	// KeepIdentifierQuotation specifies whether the normalizer should keep the quotation of identifiers.
	KeepIdentifierQuotation bool `json:"keep_identifier_quotation"`
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

func WithCollectProcedures(collectProcedure bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.CollectProcedure = collectProcedure
	}
}

func WithRemoveSpaceBetweenParentheses(removeSpaceBetweenParentheses bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.RemoveSpaceBetweenParentheses = removeSpaceBetweenParentheses
	}
}

func WithKeepTrailingSemicolon(keepTrailingSemicolon bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.KeepTrailingSemicolon = keepTrailingSemicolon
	}
}

func WithKeepIdentifierQuotation(keepIdentifierQuotation bool) normalizerOption {
	return func(c *normalizerConfig) {
		c.KeepIdentifierQuotation = keepIdentifierQuotation
	}
}

type StatementMetadata struct {
	Size       int      `json:"size"`
	Tables     []string `json:"tables"`
	Comments   []string `json:"comments"`
	Commands   []string `json:"commands"`
	Procedures []string `json:"procedures"`
}

type metadataSet struct {
	size          int
	tablesSet     map[string]struct{}
	commentsSet   map[string]struct{}
	commandsSet   map[string]struct{}
	proceduresSet map[string]struct{}
}

// addMetadata adds a value to a metadata slice if it doesn't exist in the set.
// The value is cloned to prevent retaining references to the original input string's
// backing array, which could cause memory retention issues if the caller holds on to the result.
func (m *metadataSet) addMetadata(value string, set map[string]struct{}, slice *[]string) {
	if _, exists := set[value]; !exists {
		cloned := strings.Clone(value)
		set[cloned] = struct{}{}
		*slice = append(*slice, cloned)
		m.size += len(value)
	}
}

type colonContext struct {
	// Track the token type before a colon operator to distinguish between
	// Oracle bind variables (:param) vs MySQL labels (label:) vs Snowflake semi-structured (obj:field)
	tokenTypeBeforeColon TokenType
	hasColon             bool
}

type groupablePlaceholder struct {
	groupable bool
}

type headState struct {
	readFirstNonSpaceNonComment         bool
	inLeadingParenthesesExpression      bool
	foundLeadingExpressionInParentheses bool
	standaloneExpressionInParentheses   bool
	expressionInParentheses             strings.Builder
	hasCommandInLeadingParentheses      bool
	parenthesesDepth                    int
}

type ctePhase uint8

const (
	cteName ctePhase = iota
	cteAs
	cteColumns
	cteBodyStart
	cteBody
	cteAfterBody
)

// cteAliasContext tracks CTE headers so that AS MATERIALIZED is not mistaken
// for a projection alias. Only the innermost CTE list has a non-body phase;
// enclosing lists are always inside a body, so they are kept as a depth bitmask.
type cteAliasContext struct {
	enclosing uint64
	depth     int32
	topDepth  int32
	phase     ctePhase
	active    bool
}

func (c *cteAliasContext) pop() {
	if c.enclosing == 0 {
		c.active = false
		return
	}
	c.topDepth = int32(63 - bits.LeadingZeros64(c.enclosing))
	c.enclosing &^= 1 << uint(c.topDepth)
	c.phase = cteBody
}

// tracks reports whether token can change the CTE state. Inside a CTE body only
// parentheses, statement separators, and nested WITH matter.
func (c *cteAliasContext) tracks(token *Token) bool {
	switch token.Type {
	case CTE_INDICATOR:
		return true
	case PUNCTUATION:
		return c.active && (c.phase != cteBody || token.Value == "(" || token.Value == ")" || token.Value == ";")
	case SPACE, COMMENT, MULTILINE_COMMENT, EOF:
		return false
	}
	return c.active && c.phase != cteBody
}

// isCTEMaterialized reports whether token is MATERIALIZED directly following a CTE AS.
func (c *cteAliasContext) isCTEMaterialized(token *Token) bool {
	isPunct := token.Type == PUNCTUATION
	if isPunct && token.Value == ";" {
		*c = cteAliasContext{}
		return false
	}
	result := false
	depth := c.depth
	if token.Type == CTE_INDICATOR {
		if !c.active {
			c.depth, depth = 0, 0
		} else if c.phase == cteBody {
			if c.topDepth >= 64 {
				*c = cteAliasContext{}
				return false
			}
			c.enclosing |= 1 << uint(c.topDepth)
		}
		c.topDepth, c.phase, c.active = depth, cteName, true
	} else {
		for c.active {
			handled := true
			switch c.phase {
			case cteName:
				if depth != c.topDepth {
					handled = false
				} else if token.Type == IDENT || token.Type == QUOTED_IDENT || token.Type == FUNCTION {
					c.phase = cteAs
				} else if token.Type != KEYWORD || !strings.EqualFold(token.Value, "RECURSIVE") {
					handled = false
				}
			case cteAs:
				if depth != c.topDepth {
					handled = false
				} else if token.Type == ALIAS_INDICATOR {
					c.phase = cteBodyStart
				} else if isPunct && token.Value == "(" {
					c.phase = cteColumns
				} else {
					handled = false
				}
			case cteColumns:
				if isPunct && token.Value == ")" && depth == c.topDepth+1 {
					c.phase = cteAs
				}
			case cteBodyStart:
				if isPunct && token.Value == "(" {
					c.phase = cteBody
				} else if token.Type == IDENT && strings.EqualFold(token.Value, "MATERIALIZED") {
					result = true
				} else if token.Type != KEYWORD || !strings.EqualFold(token.Value, "NOT") {
					handled = false
				}
			case cteBody:
				if isPunct && token.Value == ")" && depth == c.topDepth+1 {
					c.phase = cteAfterBody
				}
			case cteAfterBody:
				if isPunct && token.Value == "," && depth == c.topDepth {
					c.phase = cteName
				} else {
					handled = false
				}
			}
			if handled {
				break
			}
			c.pop()
		}
	}
	if isPunct {
		if token.Value == "(" {
			c.depth++
		} else if token.Value == ")" && c.depth > 0 {
			c.depth--
		}
	}
	return result
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

// normalizeToken is a helper function that handles the common normalization logic
func (n *Normalizer) normalizeToken(lexer *Lexer, normalizedSQLBuilder *strings.Builder, meta *metadataSet, statementMetadata *StatementMetadata, preProcessToken func(*Token, *LastValueToken), lexerOpts ...lexerOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error normalizing SQL token: %v", r)
		}
	}()

	var groupablePlaceholder groupablePlaceholder
	var headState headState
	var colonCtx colonContext
	var cteContext cteAliasContext
	var ctes map[string]bool // Lazily initialized when first CTE is encountered
	var inTableList bool

	var lastValueToken *LastValueToken
	stripAliases := !n.config.KeepSQLAlias

	for {
		token := lexer.Scan()
		if preProcessToken != nil {
			// pre-process the token, often used for obfuscation
			preProcessToken(token, lastValueToken)
		}
		if n.shouldCollectMetadata() {
			n.collectMetadata(token, lastValueToken, meta, statementMetadata, &ctes, &inTableList)
		}
		cteMaterialized := false
		if stripAliases && cteContext.tracks(token) {
			cteMaterialized = cteContext.isCTEMaterialized(token)
		}
		n.normalizeSQL(token, lastValueToken, normalizedSQLBuilder, &groupablePlaceholder, &headState, &colonCtx, cteMaterialized, lexerOpts...)
		if token.Type == EOF {
			break
		}
		if isValueToken(token) {
			lastValueToken = token.getLastValueToken()
		}
	}

	return nil
}

func (n *Normalizer) Normalize(input string, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	return n.normalize(input, nil, lexerOpts...)
}

// normalize is the internal implementation that handles the common normalization logic.
// preProcessToken is an optional function to process tokens before normalization (e.g., obfuscation).
func (n *Normalizer) normalize(input string, preProcessToken func(*Token, *LastValueToken), lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	lexer := New(input, lexerOpts...)
	var normalizedSQLBuilder strings.Builder
	normalizedSQLBuilder.Grow(len(input))

	meta := &metadataSet{
		tablesSet:     map[string]struct{}{},
		commentsSet:   map[string]struct{}{},
		commandsSet:   map[string]struct{}{},
		proceduresSet: map[string]struct{}{},
	}

	statementMetadata = &StatementMetadata{
		Tables:     []string{},
		Comments:   []string{},
		Commands:   []string{},
		Procedures: []string{},
	}

	if err = n.normalizeToken(lexer, &normalizedSQLBuilder, meta, statementMetadata, preProcessToken, lexerOpts...); err != nil {
		return "", nil, err
	}

	normalizedSQL = strings.Clone(normalizedSQLBuilder.String())
	statementMetadata.Size = meta.size
	return n.trimNormalizedSQL(normalizedSQL), statementMetadata, nil
}

func (n *Normalizer) shouldCollectMetadata() bool {
	return n.config.CollectTables || n.config.CollectCommands || n.config.CollectComments || n.config.CollectProcedure
}

func (n *Normalizer) collectMetadata(token *Token, lastValueToken *LastValueToken, meta *metadataSet, statementMetadata *StatementMetadata, ctes *map[string]bool, inTableList *bool) {
	if n.config.CollectComments && (token.Type == COMMENT || token.Type == MULTILINE_COMMENT) {
		comment := token.Value
		meta.addMetadata(comment, meta.commentsSet, &statementMetadata.Comments)
	} else if token.Type == COMMAND || token.Type == KEYWORD {
		*inTableList = false
		if n.config.CollectCommands && token.Type == COMMAND {
			command := strings.ToUpper(token.Value)
			meta.addMetadata(command, meta.commandsSet, &statementMetadata.Commands)
		}
	} else if token.Type == PUNCTUATION && (token.Value == "(" || token.Value == ")") {
		*inTableList = false
	} else if token.Type == IDENT || token.Type == QUOTED_IDENT || token.Type == FUNCTION {
		tokenVal := token.Value
		if token.Type == QUOTED_IDENT {
			tokenVal = trimQuotes(token)
			if n.shouldStripIdentifierQuotes(token, lastValueToken) {
				// trim quotes and set the token type to IDENT
				token.Value = tokenVal
				token.Type = IDENT
			}
		}

		// Only collect metadata if we have context from the previous token
		if lastValueToken != nil {
			// Track CTE names so we can exclude them from the tables list
			if lastValueToken.Type == CTE_INDICATOR {
				if *ctes == nil {
					*ctes = make(map[string]bool, 2)
				}
				(*ctes)[tokenVal] = true
			} else if n.config.CollectTables && lastValueToken.isTableIndicator {
				*inTableList = true
				isCTE := *ctes != nil && (*ctes)[tokenVal]
				if !isCTE {
					meta.addMetadata(tokenVal, meta.tablesSet, &statementMetadata.Tables)
				}
			} else if n.config.CollectTables && *inTableList && lastValueToken.Type == PUNCTUATION && lastValueToken.Value == "," {
				isCTE := *ctes != nil && (*ctes)[tokenVal]
				if !isCTE {
					meta.addMetadata(tokenVal, meta.tablesSet, &statementMetadata.Tables)
				}
			} else if n.config.CollectProcedure && lastValueToken.Type == PROC_INDICATOR {
				// Collect procedure names
				meta.addMetadata(tokenVal, meta.proceduresSet, &statementMetadata.Procedures)
			}
		}
	}
}

func (n *Normalizer) normalizeSQL(token *Token, lastValueToken *LastValueToken, normalizedSQLBuilder *strings.Builder, groupablePlaceholder *groupablePlaceholder, headState *headState, colonCtx *colonContext, cteMaterialized bool, lexerOpts ...lexerOption) {
	if token.Type != SPACE && token.Type != COMMENT && token.Type != MULTILINE_COMMENT {
		if token.Type == QUOTED_IDENT && !n.config.KeepIdentifierQuotation {
			if n.shouldStripIdentifierQuotes(token, lastValueToken) {
				token.Value = trimQuotes(token)
			}
		}

		// handle leading expression in parentheses
		if !headState.readFirstNonSpaceNonComment {
			headState.readFirstNonSpaceNonComment = true
			if token.Type == PUNCTUATION && token.Value == "(" {
				headState.inLeadingParenthesesExpression = true
				headState.standaloneExpressionInParentheses = true
				headState.parenthesesDepth = 1
				// Write the opening parenthesis to the buffer and return
				// to avoid double-processing it in the inLeadingParenthesesExpression block below
				headState.expressionInParentheses.WriteString(token.Value)
				return
			}
		}
		if token.Type == EOF {
			if headState.standaloneExpressionInParentheses {
				normalizedSQLBuilder.WriteString(headState.expressionInParentheses.String())
			}
			return
		} else if headState.foundLeadingExpressionInParentheses {
			// If the leading parentheses contained a SQL command (like SELECT),
			// it's a SQL statement, not a parameter declaration, so write it out
			if headState.hasCommandInLeadingParentheses {
				normalizedSQLBuilder.WriteString(headState.expressionInParentheses.String())
			}
			headState.standaloneExpressionInParentheses = false
			headState.foundLeadingExpressionInParentheses = false
		}

		if token.Type == DOLLAR_QUOTED_FUNCTION && token.Value != StringPlaceholder {
			// if the token is a dollar quoted function and it is not obfuscated,
			// we need to recusively normalize the content of the dollar quoted function
			quotedFunc := token.Value[6 : len(token.Value)-6] // remove the $func$ prefix and suffix
			normalizedQuotedFunc, _, err := n.Normalize(quotedFunc)
			if err == nil {
				// replace the content of the dollar quoted function with the normalized content
				// if there is an error, we just keep the original content
				var normalizedDollarQuotedFunc strings.Builder
				normalizedDollarQuotedFunc.Grow(len(normalizedQuotedFunc) + 12)
				normalizedDollarQuotedFunc.WriteString("$func$")
				normalizedDollarQuotedFunc.WriteString(normalizedQuotedFunc)
				normalizedDollarQuotedFunc.WriteString("$func$")
				token.Value = normalizedDollarQuotedFunc.String()
			}
		}

		if !n.config.KeepSQLAlias {
			// discard SQL alias
			if token.Type == ALIAS_INDICATOR {
				return
			}

			if lastValueToken != nil && lastValueToken.Type == ALIAS_INDICATOR {
				if (token.Type == IDENT || token.Type == QUOTED_IDENT) && !cteMaterialized {
					return
				} else {
					// if the last token is AS and the current token is not IDENT,
					// this could be a CTE like WITH ... AS (...),
					// so we do not discard the current token
					builder := normalizedSQLBuilder
					if headState.inLeadingParenthesesExpression {
						builder = &headState.expressionInParentheses
					}
					n.appendSpace(token, lastValueToken, builder, colonCtx)
					n.writeToken(lastValueToken.Type, lastValueToken.Value, builder)
				}
			}
		}

		// group consecutive obfuscated values into single placeholder
		if n.isObfuscatedValueGroupable(token, lastValueToken, groupablePlaceholder, normalizedSQLBuilder) {
			// return the token but not write it to the normalizedSQLBuilder
			return
		}

		if headState.inLeadingParenthesesExpression {
			n.appendSpace(token, lastValueToken, &headState.expressionInParentheses, colonCtx)
			n.writeToken(token.Type, token.Value, &headState.expressionInParentheses)
			// Track if we find a SQL command in the leading parentheses
			if token.Type == COMMAND {
				headState.hasCommandInLeadingParentheses = true
			}
			if token.Type == PUNCTUATION && token.Value == "(" {
				headState.parenthesesDepth++
			} else if token.Type == PUNCTUATION && token.Value == ")" {
				headState.parenthesesDepth--
				if headState.parenthesesDepth == 0 {
					headState.inLeadingParenthesesExpression = false
					headState.foundLeadingExpressionInParentheses = true
				}
			}
		} else {
			n.appendSpace(token, lastValueToken, normalizedSQLBuilder, colonCtx)
			n.writeToken(token.Type, token.Value, normalizedSQLBuilder)
		}

		// Track colon context for next token
		if token.Value == ":" {
			colonCtx.hasColon = true
			if lastValueToken != nil {
				colonCtx.tokenTypeBeforeColon = lastValueToken.Type
			} else {
				colonCtx.tokenTypeBeforeColon = EOF // Use EOF as "no token"
			}
		} else if colonCtx.hasColon {
			// Reset after processing the token following the colon
			colonCtx.hasColon = false
			colonCtx.tokenTypeBeforeColon = EOF
		}
	}
}

func (n *Normalizer) shouldStripIdentifierQuotes(token *Token, lastValueToken *LastValueToken) bool {
	if n.config.KeepIdentifierQuotation {
		return false
	}
	if token == nil {
		return true
	}
	if isAliasContext(lastValueToken) && !token.isSimpleIdentifier {
		return false
	}
	// Bracket-quoted identifiers whose content contains whitespace cannot be
	// safely unquoted: the stripped form would produce multiple SQL tokens.
	if strings.ContainsAny(token.Value, " \t\n\r") {
		return false
	}
	return true
}

func isAliasContext(lastValueToken *LastValueToken) bool {
	return lastValueToken != nil && lastValueToken.Type == ALIAS_INDICATOR
}

func (n *Normalizer) writeToken(tokenType TokenType, tokenValue string, normalizedSQLBuilder *strings.Builder) {
	if n.config.UppercaseKeywords && (tokenType == COMMAND || tokenType == KEYWORD) {
		normalizedSQLBuilder.WriteString(strings.ToUpper(tokenValue))
	} else {
		normalizedSQLBuilder.WriteString(tokenValue)
	}
}

func (n *Normalizer) isObfuscatedValueGroupable(token *Token, lastValueToken *LastValueToken, groupablePlaceholder *groupablePlaceholder, normalizedSQLBuilder *strings.Builder) bool {
	if token.Value == NumberPlaceholder || token.Value == StringPlaceholder {
		if lastValueToken == nil {
			// if the last token is nil, we know it's the start of groupable placeholders
			return false
		}
		if lastValueToken.Value == "(" || lastValueToken.Value == "[" {
			// if the last token is "(" or "[", and the current token is a placeholder,
			// we know it's the start of groupable placeholders
			// we don't return here because we still need to write the first placeholder
			groupablePlaceholder.groupable = true
		} else if lastValueToken.Value == "," && groupablePlaceholder.groupable {
			return true
		}
	}

	if lastValueToken != nil && (lastValueToken.Value == NumberPlaceholder || lastValueToken.Value == StringPlaceholder) && token.Value == "," && groupablePlaceholder.groupable {
		return true
	}

	if groupablePlaceholder.groupable && (token.Value == ")" || token.Value == "]") {
		// end of groupable placeholders
		groupablePlaceholder.groupable = false
		return false
	}

	if groupablePlaceholder.groupable && token.Value != NumberPlaceholder && token.Value != StringPlaceholder && lastValueToken != nil && lastValueToken.Value == "," {
		// This is a tricky edge case. If we are inside a groupbale block, and the current token is not a placeholder,
		// we not only want to write the current token to the normalizedSQLBuilder, but also write the last comma that we skipped.
		// For example, (?, ARRAY[?, ?, ?]) should be normalized as (?, ARRAY[?])
		normalizedSQLBuilder.WriteString(lastValueToken.Value)
		return false
	}

	return false
}

func (n *Normalizer) appendSpace(token *Token, lastValueToken *LastValueToken, normalizedSQLBuilder *strings.Builder, colonCtx *colonContext) {
	// do not add a space between parentheses if RemoveSpaceBetweenParentheses is true
	if n.config.RemoveSpaceBetweenParentheses && lastValueToken != nil && (lastValueToken.Type == FUNCTION || lastValueToken.Value == "(" || lastValueToken.Value == "[") {
		return
	}

	if n.config.RemoveSpaceBetweenParentheses && (token.Value == ")" || token.Value == "]") {
		return
	}

	// do not add a space between a dot-suffixed identifier and the following identifier,
	// because the lexer splits e.g. t.[Col] into "t." and "[Col]" as separate tokens
	if lastValueToken != nil && len(lastValueToken.Value) > 1 && lastValueToken.Value[len(lastValueToken.Value)-1] == '.' && (token.Type == IDENT || token.Type == QUOTED_IDENT) {
		return
	}

	// do not add a space after a colon when followed by an identifier or quoted identifier,
	// BUT only if the token before the colon was NOT an identifier (to preserve MySQL labels and Snowflake semi-structured access)
	// This handles Oracle bind variables like :param or :"param"
	if lastValueToken != nil && lastValueToken.Value == ":" && (token.Type == IDENT || token.Type == QUOTED_IDENT) {
		if colonCtx.hasColon && colonCtx.tokenTypeBeforeColon != IDENT && colonCtx.tokenTypeBeforeColon != QUOTED_IDENT {
			// This is likely a bind variable (e.g., = :param), not a label (e.g., label : LOOP)
			return
		}
	}

	switch token.Value {
	case ",", ";":
		return
	case "=":
		if lastValueToken != nil && lastValueToken.Value == ":" {
			return
		}
		fallthrough
	default:
		normalizedSQLBuilder.WriteString(" ")
	}
}

func (n *Normalizer) trimNormalizedSQL(normalizedSQL string) string {
	if !n.config.KeepTrailingSemicolon {
		// Remove trailing semicolon
		normalizedSQL = strings.TrimSuffix(normalizedSQL, ";")
	}
	return strings.TrimSpace(normalizedSQL)
}
