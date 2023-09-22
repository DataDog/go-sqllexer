package sqllexer

import (
	"regexp"
	"strings"
	"unicode"
)

type DBMSType string

const (
	// DBMSSQLServer is a MS SQL Server
	DBMSSQLServer DBMSType = "mssql"
	// DBMSPostgres is a PostgreSQL Server
	DBMSPostgres DBMSType = "postgresql"
	// DBMSMySQL is a MySQL Server
	DBMSMySQL DBMSType = "mysql"
	// DBMSOracle is a Oracle Server
	DBMSOracle DBMSType = "oracle"
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
	"EXECUTE":  true,
	"EXEC":     true,
	"EXPLAIN":  true,
}

var tableIndicators = map[string]bool{
	"FROM":   true,
	"JOIN":   true,
	"INTO":   true,
	"UPDATE": true,
	"TABLE":  true,
}

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
	return unicode.IsLetter(ch) || ch == '_'
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
	_, ok := Commands[ident]
	return ok
}

func isTableIndicator(ident string) bool {
	_, ok := tableIndicators[ident]
	return ok
}

func isSQLKeyword(token Token) bool {
	return token.Type == IDENT && keywordsRegex.MatchString(token.Value)
}

func isBoolean(ident string) bool {
	return strings.ToUpper(ident) == "TRUE" || strings.ToUpper(ident) == "FALSE"
}

func isNull(ident string) bool {
	return strings.ToUpper(ident) == "NULL"
}

func replaceDigits(input string, placeholder string) string {
	var builder strings.Builder

	i := 0
	for i < len(input) {
		if isDigit(rune(input[i])) {
			builder.WriteString(placeholder)
			for i < len(input) && isDigit(rune(input[i])) {
				i++
			}
		} else {
			builder.WriteByte(input[i])
			i++
		}
	}

	return builder.String()
}
