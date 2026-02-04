package sqllexer

import (
	"fmt"
	"testing"
)

// TokenSpec is a simplified token specification for testing
type TokenSpec struct {
	Type  TokenType
	Value string
}

func TestLexer(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []TokenSpec
		lexerOpts []lexerOption
	}{
		{
			name:  "simple select",
			input: "SELECT * FROM users",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "simple select with mixed case keywords",
			input: "sElEcT * fRoM users",
			expected: []TokenSpec{
				{COMMAND, "sElEcT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "fRoM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "select with number",
			input: "SELECT id FROM users WHERE id = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "WHERE"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
		},
		{
			name:  "simple select with number",
			input: "SELECT * FROM users where id = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
		},
		{
			name:  "simple select with number in quotes",
			input: "SELECT * FROM users where id = '1'",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{STRING, "'1'"},
			},
		},
		{
			name:  "simple select with negative number",
			input: "SELECT * FROM users where id = -1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "-1"},
			},
		},
		{
			name:  "simple select with string",
			input: "SELECT * FROM users where id = '12'",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{STRING, "'12'"},
			},
		},
		{
			name:  "simple select with boolean",
			input: "SELECT * FROM users where id = true",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BOOLEAN, "true"},
			},
		},
		{
			name:  "simple select with null",
			input: "SELECT * FROM users where id = null",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NULL, "null"},
			},
		},
		{
			name:  "simple select with double quoted identifier",
			input: "SELECT * FROM \"users`table\" where id = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, "\"users`table\""},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
		},
		{
			name:  "simple select with single line comment",
			input: "SELECT * FROM users where id = 1 -- comment here",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
				{SPACE, " "},
				{COMMENT, "-- comment here"},
			},
		},
		{
			name: "simple select with multi line comment",
			input: `SELECT * /* comment here */ FROM users where id = 1/* comment ` + `
here */`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{MULTILINE_COMMENT, "/* comment here */"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
				{MULTILINE_COMMENT, "/* comment \nhere */"},
			},
		},
		{
			name:  "simple malformed select",
			input: "SELECT * FROM users where id = 1 and name = 'j",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
				{SPACE, " "},
				{KEYWORD, "and"},
				{SPACE, " "},
				{IDENT, "name"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{INCOMPLETE_STRING, "'j"},
			},
		},
		{
			name:  "truncated sql",
			input: "SELECT * FROM users where id = ",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
			},
		},
		{
			name:  "simple select with array of literals",
			input: "SELECT * FROM users where id in (1, '2')",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{KEYWORD, "in"},
				{SPACE, " "},
				{PUNCTUATION, "("},
				{NUMBER, "1"},
				{PUNCTUATION, ","},
				{SPACE, " "},
				{STRING, "'2'"},
				{PUNCTUATION, ")"},
			},
		},
		{
			name:  "dollar quoted function",
			input: "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{DOLLAR_QUOTED_FUNCTION, "$func$INSERT INTO table VALUES ('a', 1, 2)$func$"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "dollar quoted string",
			input: "SELECT * FROM users where id = $tag$test$tag$",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{DOLLAR_QUOTED_STRING, "$tag$test$tag$"},
			},
		},
		{
			name:  "dollar quoted string",
			input: "SELECT * FROM users where id = $$test$$",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{DOLLAR_QUOTED_STRING, "$$test$$"},
			},
		},
		{
			name:  "numbered parameter",
			input: "SELECT * FROM users where id = $1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{POSITIONAL_PARAMETER, "$1"},
			},
		},
		{
			name:  "identifier with underscore and period",
			input: "SELECT * FROM users where user_id = 2 and users.name = 'j'",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "user_id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "2"},
				{SPACE, " "},
				{KEYWORD, "and"},
				{SPACE, " "},
				{IDENT, "users.name"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{STRING, "'j'"},
			},
		},
		{
			name:  "select with hex and octal numbers",
			input: "SELECT * FROM users where id = 0x123 and id = 0123",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "0x123"},
				{SPACE, " "},
				{KEYWORD, "and"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "0123"},
			},
		},
		{
			name:  "select with float numbers and scientific notation",
			input: "SELECT 1.2,1.2e3,1.2e-3,1.2E3,1.2E-3 FROM users",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{NUMBER, "1.2"},
				{PUNCTUATION, ","},
				{NUMBER, "1.2e3"},
				{PUNCTUATION, ","},
				{NUMBER, "1.2e-3"},
				{PUNCTUATION, ","},
				{NUMBER, "1.2E3"},
				{PUNCTUATION, ","},
				{NUMBER, "1.2E-3"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "select with double quoted identifier",
			input: `SELECT * FROM "users table"`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, "\"users table\""},
			},
		},
		{
			name:  "select with double quoted identifier with period",
			input: `SELECT * FROM "public"."users table"`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, "\"public\".\"users table\""},
			},
		},
		{
			name:  "select with escaped string",
			input: "SELECT * FROM users where id = 'j\\'s'",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{STRING, "'j\\'s'"},
			},
		},
		{
			name:  "select with escaped string",
			input: "SELECT * FROM users where id =?",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{OPERATOR, "?"},
			},
		},
		{
			name:  "select with bind parameter",
			input: "SELECT * FROM users where id = :id and name = :1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BIND_PARAMETER, ":id"},
				{SPACE, " "},
				{KEYWORD, "and"},
				{SPACE, " "},
				{IDENT, "name"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BIND_PARAMETER, ":1"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSOracle)},
		},
		{
			name:  "select with bind parameter",
			input: "SELECT * FROM users where id = @id and name = @1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BIND_PARAMETER, "@id"},
				{SPACE, " "},
				{KEYWORD, "and"},
				{SPACE, " "},
				{IDENT, "name"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BIND_PARAMETER, "@1"},
			},
		},
		{
			name:  "select with bind parameter using underscore",
			input: "SELECT * FROM users where id = @__my_id",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{BIND_PARAMETER, "@__my_id"},
			},
		},
		{
			name:  "select with system variable",
			input: "SELECT @@VERSION AS SqlServerVersion",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{SYSTEM_VARIABLE, "@@VERSION"},
				{SPACE, " "},
				{ALIAS_INDICATOR, "AS"},
				{SPACE, " "},
				{IDENT, "SqlServerVersion"},
			},
		},
		{
			name:  "SQL Server quoted identifier",
			input: "SELECT [user] FROM [test].[table] WHERE [id] = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{QUOTED_IDENT, "[user]"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, "[test].[table]"},
				{SPACE, " "},
				{KEYWORD, "WHERE"},
				{SPACE, " "},
				{QUOTED_IDENT, "[id]"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSSQLServer)},
		},
		{
			name:  "MySQL backtick quoted identifier",
			input: "SELECT `user` FROM `test`.`table` WHERE `id` = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{QUOTED_IDENT, "`user`"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, "`test`.`table`"},
				{SPACE, " "},
				{KEYWORD, "WHERE"},
				{SPACE, " "},
				{QUOTED_IDENT, "`id`"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSMySQL)},
		},
		{
			name:  "Quoted identifier with non-ascii characters",
			input: `SELECT "test" FROM "fóo"."bar" WHERE "id" = 1`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{QUOTED_IDENT, `"test"`},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{QUOTED_IDENT, `"fóo"."bar"`},
				{SPACE, " "},
				{KEYWORD, "WHERE"},
				{SPACE, " "},
				{QUOTED_IDENT, `"id"`},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
		},
		{
			name:  "Tokenize function",
			input: "SELECT count(*) FROM users",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{FUNCTION, "count"},
				{PUNCTUATION, "("},
				{WILDCARD, "*"},
				{PUNCTUATION, ")"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "Tokenize temp table",
			input: `SELECT * FROM #temp`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "#temp"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSSQLServer)},
		},
		{
			name:  "MySQL comment",
			input: `SELECT * FROM users # comment`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
				{SPACE, " "},
				{COMMENT, "# comment"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSMySQL)},
		},
		{
			name:  "drop table if exists",
			input: `DROP TABLE IF EXISTS users`,
			expected: []TokenSpec{
				{COMMAND, "DROP"},
				{SPACE, " "},
				{KEYWORD, "TABLE"},
				{SPACE, " "},
				{KEYWORD, "IF"},
				{SPACE, " "},
				{KEYWORD, "EXISTS"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "select only",
			input: "SELECT * FROM ONLY tab1 where id = 1",
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{WILDCARD, "*"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{KEYWORD, "ONLY"},
				{SPACE, " "},
				{IDENT, "tab1"},
				{SPACE, " "},
				{KEYWORD, "where"},
				{SPACE, " "},
				{IDENT, "id"},
				{SPACE, " "},
				{OPERATOR, "="},
				{SPACE, " "},
				{NUMBER, "1"},
			},
		},
		{
			name:  "extracts n'th element of JSON array",
			input: `SELECT data::json -> 2 FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "->"},
				{SPACE, " "},
				{NUMBER, "2"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "extracts JSON object field with the given key",
			input: `SELECT data::json -> 'key' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "->"},
				{SPACE, " "},
				{STRING, "'key'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "extracts n'th element of JSON array, as text",
			input: `SELECT data::json ->> 2 FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "->>"},
				{SPACE, " "},
				{NUMBER, "2"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "extracts JSON object field with the given key, as text",
			input: `SELECT data::json ->> 'key' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "->>"},
				{SPACE, " "},
				{STRING, "'key'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "extracts JSON sub-object at the specified path",
			input: `SELECT data::json #> '{key1,key2}' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "#>"},
				{SPACE, " "},
				{STRING, "'{key1,key2}'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "extracts JSON sub-object at the specified path as text",
			input: `SELECT data::json #>> '{key1,key2}' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "json"},
				{SPACE, " "},
				{JSON_OP, "#>>"},
				{SPACE, " "},
				{STRING, "'{key1,key2}'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "JSON path return any item for the specified JSON value",
			input: `SELECT data::jsonb @? '$.a[*] ? (@ > 2)' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "jsonb"},
				{SPACE, " "},
				{JSON_OP, "@?"},
				{SPACE, " "},
				{STRING, "'$.a[*] ? (@ > 2)'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "JSON path predicate check for the specified JSON value",
			input: `SELECT data::jsonb @@ '$.a[*] > 2' FROM users`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "data"},
				{OPERATOR, "::"},
				{IDENT, "jsonb"},
				{SPACE, " "},
				{JSON_OP, "@@"},
				{SPACE, " "},
				{STRING, "'$.a[*] > 2'"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "users"},
			},
		},
		{
			name:  "create procedure",
			input: `CREATE PROCEDURE test_proc (IN param1 INT, OUT param2 VARCHAR(255))`,
			expected: []TokenSpec{
				{COMMAND, "CREATE"},
				{SPACE, " "},
				{PROC_INDICATOR, "PROCEDURE"},
				{SPACE, " "},
				{IDENT, "test_proc"},
				{SPACE, " "},
				{PUNCTUATION, "("},
				{KEYWORD, "IN"},
				{SPACE, " "},
				{IDENT, "param1"},
				{SPACE, " "},
				{IDENT, "INT"},
				{PUNCTUATION, ","},
				{SPACE, " "},
				{KEYWORD, "OUT"},
				{SPACE, " "},
				{IDENT, "param2"},
				{SPACE, " "},
				{FUNCTION, "VARCHAR"},
				{PUNCTUATION, "("},
				{NUMBER, "255"},
				{PUNCTUATION, ")"},
				{PUNCTUATION, ")"},
			},
		},
		{
			name:  "escape character",
			input: `SELECT E'\c'`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "E"},
				{STRING, `'\c'`},
			},
		},
		{
			name:  "unknown character",
			input: `\c`, // \c is a psql command but not a valid postgres sql
			expected: []TokenSpec{
				{UNKNOWN, `\`},
				{IDENT, "c"},
			},
		},
		{
			name:  "mysql comment",
			input: "#1",
			expected: []TokenSpec{
				{COMMENT, "#1"},
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSMySQL)},
		},
		{
			name:  "string with escaped characters",
			input: `SELECT 1 WHERE 'test_temp_test' LIKE '%\_temp\_%' ESCAPE '\'`,
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{NUMBER, "1"},
				{SPACE, " "},
				{KEYWORD, "WHERE"},
				{SPACE, " "},
				{STRING, "'test_temp_test'"},
				{SPACE, " "},
				{KEYWORD, "LIKE"},
				{SPACE, " "},
				{STRING, `'%\_temp\_%'`},
				{SPACE, " "},
				{KEYWORD, "ESCAPE"},
				{SPACE, " "},
				{STRING, `'\'`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			i := 0

			for {
				got := lexer.Scan()
				if got.Type == EOF {
					if i != len(tt.expected) {
						t.Errorf("got %d tokens, want %d", i, len(tt.expected))
					}
					break
				}

				if i >= len(tt.expected) {
					t.Errorf("got more tokens than expected at position %d", i)
					break
				}

				want := tt.expected[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Value != want.Value {
					t.Errorf("token[%d] got value %q, want %q", i, got.Value, want.Value)
				}

				i++
			}
		})
	}
}

func TestLexerIdentifierWithDigits(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []TokenSpec
		expectedDigits []bool
		lexerOpts      []lexerOption
	}{
		{
			input: `abc123`,
			expectedTokens: []TokenSpec{
				{IDENT, "abc123"},
			},
			expectedDigits: []bool{
				true,
			},
		},
		{
			input: `abc123def456`,
			expectedTokens: []TokenSpec{
				{IDENT, "abc123def456"},
			},
			expectedDigits: []bool{
				true,
			},
		},
		{
			input: `abc123 bc12d456, "c123ef"`,
			expectedTokens: []TokenSpec{
				{IDENT, "abc123"},
				{SPACE, " "},
				{IDENT, "bc12d456"},
				{PUNCTUATION, ","},
				{SPACE, " "},
				{QUOTED_IDENT, `"c123ef"`},
			},
			expectedDigits: []bool{
				true,
				false,
				true,
				false,
				false,
				true,
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			i := 0

			for {
				got := lexer.Scan()
				if got.Type == EOF {
					if i != len(tt.expectedTokens) {
						t.Errorf("got %d tokens, want %d", i, len(tt.expectedTokens))
					}
					break
				}

				if i >= len(tt.expectedTokens) {
					t.Errorf("got more tokens than expected at position %d", i)
					break
				}

				want := tt.expectedTokens[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Value != want.Value {
					t.Errorf("token[%d] got value %q, want %q", i, got.Value, want.Value)
				}

				if i < len(tt.expectedDigits) {
					if got.hasDigits != tt.expectedDigits[i] {
						t.Errorf("token[%d] got %v digits, want %v", i, got.hasDigits, tt.expectedDigits[i])
					}
				}

				i++
			}
		})
	}
}

func TestLexerIdentifierWithQuotes(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []TokenSpec
		expectedQuotes bool
		lexerOpts      []lexerOption
	}{
		{
			input: `"abc"`,
			expectedTokens: []TokenSpec{
				{QUOTED_IDENT, `"abc"`},
			},
			expectedQuotes: true,
		},
		{
			input: `"abc"."def"`,
			expectedTokens: []TokenSpec{
				{QUOTED_IDENT, `"abc"."def"`},
			},
			expectedQuotes: true,
		},
		{
			input: `"fóo"."bar"`,
			expectedTokens: []TokenSpec{
				{QUOTED_IDENT, `"fóo"."bar"`},
			},
			expectedQuotes: true,
		},
		{
			input: `SELECT "fóo"."`,
			expectedTokens: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{ERROR, `"fóo"."`},
			},
			expectedQuotes: false,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			i := 0

			for {
				got := lexer.Scan()
				if got.Type == EOF {
					if i != len(tt.expectedTokens) {
						t.Errorf("got %d tokens, want %d", i, len(tt.expectedTokens))
					}
					break
				}

				if i >= len(tt.expectedTokens) {
					t.Errorf("got more tokens than expected at position %d", i)
					break
				}

				want := tt.expectedTokens[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Value != want.Value {
					t.Errorf("token[%d] got value %q, want %q", i, got.Value, want.Value)
				}

				if got.hasQuotes != tt.expectedQuotes {
					t.Errorf("token[%d] got %v quotes, want %v", i, got.hasQuotes, tt.expectedQuotes)
				}

				i++
			}
		})
	}
}

func TestLexerUnicode(t *testing.T) {
	tests := []struct {
		input     string
		expected  []TokenSpec
		lexerOpts []lexerOption
	}{
		{
			input: `abc`,
			expected: []TokenSpec{
				{IDENT, "abc"},
			},
		},
		{
			input: `Descripció_CAT`,
			expected: []TokenSpec{
				{IDENT, "Descripció_CAT"},
			},
		},
		{
			input: `世界`,
			expected: []TokenSpec{
				{IDENT, "世界"},
			},
		},
		{
			input: `こんにちは`,
			expected: []TokenSpec{
				{IDENT, "こんにちは"},
			},
		},
		{
			input: `안녕하세요`,
			expected: []TokenSpec{
				{IDENT, "안녕하세요"},
			},
		},
		{
			input: `über`,
			expected: []TokenSpec{
				{IDENT, `über`},
			},
		},
		{
			input: `résumé`,
			expected: []TokenSpec{
				{IDENT, "résumé"},
			},
		},
		{
			input: `"über"`,
			expected: []TokenSpec{
				{QUOTED_IDENT, `"über"`},
			},
		},
		// Multi-byte UTF-8 characters that are not valid identifiers should be
		// tokenized as single UNKNOWN tokens, not split into individual bytes.
		// This is a regression test for scanUnknown() byte-splitting bug.
		{
			input: "，", // Full-width comma U+FF0C (3 bytes)
			expected: []TokenSpec{
				{UNKNOWN, "，"},
			},
		},
		{
			input: "🔥", // Emoji U+1F525 (4 bytes)
			expected: []TokenSpec{
				{UNKNOWN, "🔥"},
			},
		},
		{
			input: "SELECT a， b FROM t", // Full-width comma in query
			expected: []TokenSpec{
				{COMMAND, "SELECT"},
				{SPACE, " "},
				{IDENT, "a"},
				{UNKNOWN, "，"},
				{SPACE, " "},
				{IDENT, "b"},
				{SPACE, " "},
				{KEYWORD, "FROM"},
				{SPACE, " "},
				{IDENT, "t"},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			i := 0
			for {
				got := lexer.Scan()
				if got.Type == EOF {
					break
				}
				want := tt.expected[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Value != want.Value {
					t.Errorf("token[%d] got value %q, want %q", i, got.Value, want.Value)
				}
				i++
			}
		})
	}
}

func ExampleLexer() {
	query := "SELECT * FROM users WHERE id = 1"
	lexer := New(query)

	// Print tokens one by one
	for {
		token := lexer.Scan()
		if token.Type == EOF {
			break
		}
		fmt.Println(token)
	}
}
