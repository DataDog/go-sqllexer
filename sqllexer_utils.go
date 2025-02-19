package sqllexer

import (
	"strings"
	"unicode"
)

type DBMSType string

const (
	// DBMSSQLServer is a MS SQL
	DBMSSQLServer       DBMSType = "mssql"
	DBMSSQLServerAlias1 DBMSType = "sql-server" // .Net tracer
	DBMSSQLServerAlias2 DBMSType = "sqlserver"  // Java tracer
	// DBMSPostgres is a PostgreSQL Server
	DBMSPostgres       DBMSType = "postgresql"
	DBMSPostgresAlias1 DBMSType = "postgres" // Ruby, JavaScript tracers
	// DBMSMySQL is a MySQL Server
	DBMSMySQL DBMSType = "mysql"
	// DBMSOracle is a Oracle Server
	DBMSOracle DBMSType = "oracle"
	// DBMSSnowflake is a Snowflake Server
	DBMSSnowflake DBMSType = "snowflake"
)

var dbmsAliases = map[DBMSType]DBMSType{
	DBMSSQLServerAlias1: DBMSSQLServer,
	DBMSSQLServerAlias2: DBMSSQLServer,
	DBMSPostgresAlias1:  DBMSPostgres,
}

func getDBMSFromAlias(alias DBMSType) DBMSType {
	if canonical, exists := dbmsAliases[alias]; exists {
		return canonical
	}
	return alias
}

var commands = []string{
	"SELECT",
	"INSERT",
	"UPDATE",
	"DELETE",
	"CREATE",
	"ALTER",
	"DROP",
	"JOIN",
	"GRANT",
	"REVOKE",
	"COMMIT",
	"BEGIN",
	"TRUNCATE",
	"MERGE",
	"EXECUTE",
	"EXEC",
	"EXPLAIN",
	"STRAIGHT_JOIN",
	"USE",
	"CLONE",
}

var tableIndicatorCommands = []string{
	"JOIN",
	"UPDATE",
	"STRAIGHT_JOIN", // MySQL
	"CLONE",         // Snowflake
}

var tableIndicatorKeywords = []string{
	"FROM",
	"INTO",
	"TABLE",
	"EXISTS", // Drop Table If Exists
	"ONLY",   // PostgreSQL
}

var keywords = []string{
	"ADD",
	"ALL",
	"AND",
	"ANY",
	"ASC",
	"BETWEEN",
	"BY",
	"CASE",
	"CHECK",
	"COLUMN",
	"CONSTRAINT",
	"DATABASE",
	"DECLARE",
	"DEFAULT",
	"DESC",
	"DISTINCT",
	"ELSE",
	"END",
	"EXISTS",
	"FOREIGN",
	"FROM",
	"GROUP",
	"HAVING",
	"IN",
	"INDEX",
	"INNER",
	"INTO",
	"IS",
	"KEY",
	"LEFT",
	"LIKE",
	"LIMIT",
	"NOT",
	"ON",
	"OR",
	"ORDER",
	"OUT",
	"OUTER",
	"PRIMARY",
	"PROCEDURE",
	"REPLACE",
	"RETURNS",
	"RIGHT",
	"ROLLBACK",
	"ROWNUM",
	"SET",
	"SOME",
	"TABLE",
	"TOP",
	"UNION",
	"UNIQUE",
	"VALUES",
	"VIEW",
	"WHERE",
	"CUBE",
	"ROLLUP",
	"LITERAL",
	"WINDOW",
	"VACCUM",
	"ANALYZE",
	"ILIKE",
	"USING",
	"ASSERTION",
	"DOMAIN",
	"CLUSTER",
	"COPY",
	"PLPGSQL",
	"TRIGGER",
	"TEMPORARY",
	"UNLOGGED",
	"RECURSIVE",
	"RETURNING",
	"OFFSET",
	"OF",
	"SKIP",
	"IF",
	"ONLY",
}

var (
	// Pre-defined constants for common values
	booleanValues = []string{
		"TRUE",
		"FALSE",
	}

	nullValues = []string{
		"NULL",
	}

	procedureNames = []string{
		"PROCEDURE",
		"PROC",
	}

	ctes = []string{
		"WITH",
	}

	alias = []string{
		"AS",
	}
)

// buildCombinedTrie combines all keywords into a single trie
func buildCombinedTrie() *trieNode {
	root := &trieNode{children: make(map[rune]*trieNode)}

	// Add all types of keywords
	addToTrie(root, commands, COMMAND, false)
	addToTrie(root, keywords, KEYWORD, false)
	addToTrie(root, tableIndicatorCommands, COMMAND, true)
	addToTrie(root, tableIndicatorKeywords, KEYWORD, true)
	addToTrie(root, booleanValues, BOOLEAN, false)
	addToTrie(root, nullValues, NULL, false)
	addToTrie(root, procedureNames, PROC_INDICATOR, false)
	addToTrie(root, ctes, CTE_INDICATOR, false)
	addToTrie(root, alias, ALIAS_INDICATOR, false)

	return root
}

func addToTrie(root *trieNode, words []string, tokenType TokenType, isTableIndicator bool) {
	for _, word := range words {
		node := root
		// Convert to uppercase for case-insensitive matching
		for _, ch := range strings.ToUpper(word) {
			if next, exists := node.children[ch]; exists {
				node = next
			} else {
				next = &trieNode{children: make(map[rune]*trieNode)}
				node.children[ch] = next
				node = next
			}
		}
		node.isEnd = true
		node.tokenType = tokenType
		node.isTableIndicator = isTableIndicator
	}
}

var keywordRoot = buildCombinedTrie()

// TODO: Optimize these functions to work with rune positions instead of string operations
// They are currently used by obfuscator and normalizer, which we'll optimize later
func replaceDigits(source *string, token *Token, placeholder string) string {
	var replacedToken = new(strings.Builder)
	replacedToken.Grow(token.End - token.Start)

	start := token.Start

	// loop over token.digits indexes, write start:token.digits[i] to builder
	// write placeholder to builder if no consecutive digits
	// write start:token.End to builder
	for i := 0; i < len(token.ExtraInfo.Digits); i++ {
		if token.ExtraInfo.Digits[i]-start >= 1 {
			replacedToken.WriteString((*source)[start:token.ExtraInfo.Digits[i]])
		}
		if i == 0 || token.ExtraInfo.Digits[i] != token.ExtraInfo.Digits[i-1]+1 {
			replacedToken.WriteString(placeholder)
		}
		start = token.ExtraInfo.Digits[i] + 1
	}

	// write start:token.End to builder
	replacedToken.WriteString((*source)[start:token.End])
	token.ExtraInfo.Digits = nil
	return replacedToken.String()
}

func trimQuotes(source *string, token *Token) string {
	var trimmedToken = new(strings.Builder)
	trimmedToken.Grow(token.End - token.Start - len(token.ExtraInfo.Quotes))

	start := token.Start

	// loop over token.digits indexes, write start:token.digits[i] to builder
	// write placeholder to builder if no consecutive digits
	// write start:token.End to builder
	for i := 0; i < len(token.ExtraInfo.Quotes); i++ {
		if token.ExtraInfo.Quotes[i]-start >= 1 {
			trimmedToken.WriteString((*source)[start:token.ExtraInfo.Quotes[i]])
		}
		start = token.ExtraInfo.Quotes[i] + 1
	}

	// write start:token.End to builder
	trimmedToken.WriteString((*source)[start:token.End])
	token.ExtraInfo.Quotes = nil
	return trimmedToken.String()
}

// Character classification functions
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isLeadingSign(ch rune) bool {
	return ch == '+' || ch == '-'
}

func isExpontent(ch rune) bool {
	return ch == 'e' || ch == 'E'
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isAsciiLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isLetter(ch rune) bool {
	return isAsciiLetter(ch) || ch == '_' ||
		(ch > 127 && unicode.IsLetter(ch))
}

func isAlphaNumeric(ch rune) bool {
	return isLetter(ch) || isDigit(ch) ||
		(ch > 127 && unicode.IsNumber(ch))
}

func isDoubleQuote(ch rune) bool {
	return ch == '"'
}

func isSingleQuote(ch rune) bool {
	return ch == '\''
}

func isOperator(ch rune) bool {
	return ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '=' || ch == '<' || ch == '>' ||
		ch == '!' || ch == '&' || ch == '|' || ch == '^' || ch == '%' || ch == '~' || ch == '?' ||
		ch == '@' || ch == ':' || ch == '#'
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
	return ch == '(' || ch == ')' || ch == ',' || ch == ';' || ch == '.' || ch == ':' ||
		ch == '[' || ch == ']' || ch == '{' || ch == '}'
}

func isEOF(ch rune) bool {
	return ch == 0
}

func isValueToken(token *Token) bool {
	return token.Type != EOF && token.Type != WS && token.Type != COMMENT && token.Type != MULTILINE_COMMENT
}

func isIdentifier(ch rune) bool {
	return ch == '.' || ch == '?' || ch == '$' || ch == '#' || ch == '/' || ch == '@' || ch == '!' || isLetter(ch) || isDigit(ch)
}
