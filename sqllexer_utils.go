package sqllexer

import (
	"strings"
	"unicode"
)

type DBMSType string

const (
	// DBMSSQLServer is a MS SQL
	DBMSSQLServer DBMSType = "mssql"
	// DBMSPostgres is a PostgreSQL Server
	DBMSPostgres DBMSType = "postgresql"
	// DBMSMySQL is a MySQL Server
	DBMSMySQL DBMSType = "mysql"
	// DBMSOracle is a Oracle Server
	DBMSOracle DBMSType = "oracle"
	// DBMSSnowflake is a Snowflake Server
	DBMSSnowflake DBMSType = "snowflake"
)

func PrecomputeCaseInsensitiveKeys[T any](input map[string]T) map[string]T {
	result := make(map[string]T, len(input)*3)
	for key, value := range input {
		result[key] = value
		result[strings.ToLower(key)] = value
		result[strings.ToUpper(key)] = value
	}
	return result
}

var commands = map[string]bool{
	"Select":        true,
	"Insert":        true,
	"Update":        true,
	"Delete":        true,
	"Create":        true,
	"Alter":         true,
	"Drop":          true,
	"Join":          true,
	"Grant":         true,
	"Revoke":        true,
	"Commit":        true,
	"Begin":         true,
	"Truncate":      true,
	"Merge":         true,
	"Execute":       true,
	"Exec":          true,
	"Explain":       true,
	"Straight_Join": true,
	"Use":           true,
	"Clone":         true,
}

var commandsMap = PrecomputeCaseInsensitiveKeys(commands)

var tableIndicators = map[string]bool{
	"From":          true,
	"Join":          true,
	"Into":          true,
	"Update":        true,
	"Table":         true,
	"Exists":        true, // Drop Table If Exists
	"Straight_Join": true, // MySQL
	"Clone":         true, // Snowflake
	"Only":          true, // PostgreSQL
}

var tableIndicatorsMap = PrecomputeCaseInsensitiveKeys(tableIndicators)

var keywords = map[string]bool{
	"Select":     true,
	"Insert":     true,
	"Update":     true,
	"Delete":     true,
	"Create":     true,
	"Alter":      true,
	"Drop":       true,
	"Grant":      true,
	"Revoke":     true,
	"Add":        true,
	"All":        true,
	"And":        true,
	"Any":        true,
	"As":         true,
	"Asc":        true,
	"Begin":      true,
	"Between":    true,
	"By":         true,
	"Case":       true,
	"Check":      true,
	"Column":     true,
	"Commit":     true,
	"Constraint": true,
	"Database":   true,
	"Declare":    true,
	"Default":    true,
	"Desc":       true,
	"Distinct":   true,
	"Else":       true,
	"End":        true,
	"Exec":       true,
	"Exists":     true,
	"Foreign":    true,
	"From":       true,
	"Group":      true,
	"Having":     true,
	"In":         true,
	"Index":      true,
	"Inner":      true,
	"Into":       true,
	"Is":         true,
	"Join":       true,
	"Key":        true,
	"Left":       true,
	"Like":       true,
	"Limit":      true,
	"Not":        true,
	"On":         true,
	"Or":         true,
	"Order":      true,
	"Outer":      true,
	"Primary":    true,
	"Procedure":  true,
	"Replace":    true,
	"Returns":    true,
	"Right":      true,
	"Rollback":   true,
	"Rownum":     true,
	"Set":        true,
	"Some":       true,
	"Table":      true,
	"Top":        true,
	"Truncate":   true,
	"Union":      true,
	"Unique":     true,
	"Use":        true,
	"Values":     true,
	"View":       true,
	"Where":      true,
	"Cube":       true,
	"Rollup":     true,
	"Literal":    true,
	"Window":     true,
	"Vaccum":     true,
	"Analyze":    true,
	"Ilike":      true,
	"Using":      true,
	"Assertion":  true,
	"Domain":     true,
	"Cluster":    true,
	"Copy":       true,
	"Explain":    true,
	"Plpgsql":    true,
	"Trigger":    true,
	"Temporary":  true,
	"Unlogged":   true,
	"Recursive":  true,
	"Returning":  true,
	"Offset":     true,
	"Of":         true,
	"Skip":       true,
	"If":         true,
	"Only":       true,
}

var keywordsMap = PrecomputeCaseInsensitiveKeys(keywords)

var jsonOperators = map[string]bool{
	"->":  true,
	"->>": true,
	"#>":  true,
	"#>>": true,
	"@?":  true,
	"@@":  true,
	"?|":  true,
	"?&":  true,
	"@>":  true,
	"<@":  true,
	"#-":  true,
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func isExpontent(ch rune) bool {
	return ch == 'e' || ch == 'E'
}

func isLeadingSign(ch rune) bool {
	return ch == '+' || ch == '-'
}

func isLetter(ch rune) bool {
	// Fast path: ASCII letters and underscore
	if ch <= 127 {
		return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
	}
	// Fallback to Unicode
	return unicode.IsLetter(ch)
}

func isAlphaNumeric(ch rune) bool {
	// Check if it's a digit first, then letter (faster for numbers)
	return isDigit(ch) || isLetter(ch)
}

func isDoubleQuote(ch rune) bool {
	return ch == '"'
}

func isSingleQuote(ch rune) bool {
	return ch == '\''
}

func isOperator(ch rune) bool {
	return ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '=' || ch == '<' || ch == '>' || ch == '!' || ch == '&' || ch == '|' || ch == '^' || ch == '%' || ch == '~' || ch == '?' || ch == '@' || ch == ':' || ch == '#'
}

func isWildcard(ch rune) bool {
	return ch == '*'
}

func isSingleLineComment(ch rune, nextCh rune) bool {
	return ch == '-' && nextCh == '-'
}

func isMultiLineComment(ch rune, nextCh rune) bool {
	return ch == '/' && nextCh == '*'
}

func isPunctuation(ch rune) bool {
	return ch == '(' || ch == ')' || ch == ',' || ch == ';' || ch == '.' || ch == ':' || ch == '[' || ch == ']' || ch == '{' || ch == '}'
}

func isEOF(ch rune) bool {
	return ch == 0
}

func isCommand(ident string) bool {
	_, ok := commandsMap[ident]
	return ok
}

func isTableIndicator(ident string) bool {
	_, ok := tableIndicatorsMap[ident]
	return ok
}

func isSQLKeyword(ident string) bool {
	_, ok := keywordsMap[ident]
	return ok
}

func isProcedure(token *Token) bool {
	if token.Type != IDENT {
		return false
	}
	return token.Value == "PROCEDURE" || token.Value == "procedure" || token.Value == "Procedure" || token.Value == "PROC" || token.Value == "proc" || token.Value == "Proc"
}

func isBoolean(ident string) bool {
	// allocation free fast path for common cases
	return ident == "true" || ident == "false" || ident == "TRUE" || ident == "FALSE" || ident == "True" || ident == "False"
}

func isNull(ident string) bool {
	// allocation free fast path for common cases
	return ident == "null" || ident == "NULL" || ident == "Null"
}

func isWith(ident string) bool {
	return ident == "WITH" || ident == "with" || ident == "With"
}

func isAs(ident string) bool {
	return ident == "AS" || ident == "as" || ident == "As"
}

func isJsonOperator(token *Token) bool {
	if token.Type != OPERATOR {
		return false
	}
	_, ok := jsonOperators[token.Value]
	return ok
}

func replaceDigits(input string, placeholder string) string {
	var builder strings.Builder
	n := len(input)
	i := 0

	for i < n {
		// Skip over non-digit characters
		start := i
		for i < n && !isDigit(rune(input[i])) {
			i++
		}
		// Write non-digit substring (if any)
		if start < i {
			builder.WriteString(input[start:i])
		}

		// Replace consecutive digits with the placeholder
		if i < n && isDigit(rune(input[i])) {
			builder.WriteString(placeholder)
			// Skip over all consecutive digits
			for i < n && isDigit(rune(input[i])) {
				i++
			}
		}
	}

	return builder.String()
}

var (
	doubleQuotesReplacer  = strings.NewReplacer("\"", "")
	backQuotesReplacer    = strings.NewReplacer("`", "")
	bracketQuotesReplacer = strings.NewReplacer("[", "", "]", "")
)

func trimQuotes(input string, delim string, closingDelim string) string {
	var replacer *strings.Replacer
	switch {
	// common quote types get an already allocated replacer
	case delim == closingDelim && delim == "\"":
		replacer = doubleQuotesReplacer
	case delim == closingDelim && delim == "`":
		replacer = backQuotesReplacer
	case delim == "[" && closingDelim == "]":
		replacer = bracketQuotesReplacer

	// common case of `delim` and `closingDelim` being the same, gets a simpler replacer
	case delim == closingDelim:
		replacer = strings.NewReplacer(delim, "")

	default:
		replacer = strings.NewReplacer(delim, "", closingDelim, "")
	}

	return replacer.Replace(input)
}
