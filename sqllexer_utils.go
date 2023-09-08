package sqllexer

import (
	"regexp"
	"unicode"
)

const (
	// DBMSSQLServer is a MS SQL Server
	DBMSSQLServer = "mssql"
	// DBMSPostgres is a PostgreSQL Server
	DBMSPostgres = "postgresql"
	// DBMSMySQL is a MySQL Server
	DBMSMySQL = "mysql"
	// DBMSOracle is a Oracle Server
	DBMSOracle = "oracle"
)

var Commands = map[string]bool{
	"SELECT":   true,
	"INSERT":   true,
	"UPDATE":   true,
	"DELETE":   true,
	"CREATE":   true,
	"ALTER":    true,
	"DROP":     true,
	"JOIN":     true,
	"GRANT":    true,
	"REVOKE":   true,
	"COMMIT":   true,
	"BEGIN":    true,
	"TRUNCATE": true,
	"MERGE":    true,
}

var tableIndicators = map[string]bool{
	"FROM":   true,
	"JOIN":   true,
	"INTO":   true,
	"UPDATE": true,
	"TABLE":  true,
}

var digitsRegex = regexp.MustCompile(`\d+`)

var keywordsRegex = regexp.MustCompile(`(?i)^(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|GRANT|REVOKE|ADD|ALL|AND|ANY|AS|ASC|BEGIN|BETWEEN|BY|CASE|CHECK|COLUMN|COMMIT|CONSTRAINT|DATABASE|DECLARE|DEFAULT|DESC|DISTINCT|ELSE|END|EXEC|EXISTS|FOREIGN|FROM|GROUP|HAVING|IN|INDEX|INNER|INTO|IS|JOIN|KEY|LEFT|LIKE|LIMIT|NOT|ON|OR|ORDER|OUTER|PRIMARY|PROCEDURE|REPLACE|RETURNS|RIGHT|ROLLBACK|ROWNUM|SET|SOME|TABLE|TOP|TRUNCATE|UNION|UNIQUE|USE|VALUES|VIEW|WHERE|CUBE|ROLLUP|LITERAL|WINDOW|VACCUM|ANALYZE|ILIKE|USING|ASSERTION|DOMAIN|CLUSTER|COPY|EXPLAIN|PLPGSQL|TRIGGER|TEMPORARY|UNLOGGED|RECURSIVE|RETURNING)$`)

var groupableRegex = regexp.MustCompile(`(\()\s*\?(?:\s*,\s*\?\s*)*\s*(\))|(\[)\s*\?(?:\s*,\s*\?\s*)*\s*(\])`)

var sqlAliasRegex = regexp.MustCompile(`(?i)\s+AS\s+[\w?]+`)

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
	return unicode.IsLetter(ch) || ch == '_' || ch == '#'
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

func isDollarQuotedFunction(chs []rune) bool {
	return string(chs) == "$func$"
}

func isCommand(ident string) bool {
	_, ok := Commands[ident]
	return ok
}

func isTableIndicator(ident string) bool {
	_, ok := tableIndicators[ident]
	return ok
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch) - '0'
	case 'a' <= ch && ch <= 'f':
		return int(ch) - 'a' + 10
	case 'A' <= ch && ch <= 'F':
		return int(ch) - 'A' + 10
	}
	return 16 // larger than any legal digit val
}

func collapseWhitespace(val string) string {
	collapse_whitespace_regex := regexp.MustCompile(`[\s]+`)
	return collapse_whitespace_regex.ReplaceAllString(val, " ")
}
