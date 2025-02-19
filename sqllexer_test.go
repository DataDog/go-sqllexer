package sqllexer

import (
	"fmt"
	"testing"
)

// TokenSpec is a simplified token specification for testing
type TokenSpec struct {
	Type  TokenType
	Start int
	End   int
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
				{COMMAND, 0, 6},  // SELECT
				{WS, 6, 7},       // space
				{WILDCARD, 7, 8}, // *
				{WS, 8, 9},       // space
				{KEYWORD, 9, 13}, // FROM
				{WS, 13, 14},     // space
				{IDENT, 14, 19},  // users
			},
		},
		{
			name:  "select with number",
			input: "SELECT id FROM users WHERE id = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 9},      // id
				{WS, 9, 10},        // space
				{KEYWORD, 10, 14},  // FROM
				{WS, 14, 15},       // space
				{IDENT, 15, 20},    // users
				{WS, 20, 21},       // space
				{KEYWORD, 21, 26},  // WHERE
				{WS, 26, 27},       // space
				{IDENT, 27, 29},    // id
				{WS, 29, 30},       // space
				{OPERATOR, 30, 31}, // =
				{WS, 31, 32},       // space
				{NUMBER, 32, 33},   // 1
			},
		},
		{
			name:  "simple select with number",
			input: "SELECT * FROM users where id = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{NUMBER, 31, 32},   // 1
			},
		},
		{
			name:  "simple select with number in quotes",
			input: "SELECT * FROM users where id = '1'",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{STRING, 31, 34},   // '1'
			},
		},
		{
			name:  "simple select with negative number",
			input: "SELECT * FROM users where id = -1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{NUMBER, 31, 33},   // -1
			},
		},
		{
			name:  "simple select with string",
			input: "SELECT * FROM users where id = '12'",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{STRING, 31, 35},   // '12'
			},
		},
		{
			name:  "simple select with boolean",
			input: "SELECT * FROM users where id = true",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{BOOLEAN, 31, 35},  // true
			},
		},
		{
			name:  "simple select with null",
			input: "SELECT * FROM users where id = null",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{NULL, 31, 35},     // null
			},
		},
		{
			name:  "simple select with double quoted identifier",
			input: "SELECT * FROM \"users table\" where id = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},        // SELECT
				{WS, 6, 7},             // space
				{WILDCARD, 7, 8},       // *
				{WS, 8, 9},             // space
				{KEYWORD, 9, 13},       // FROM
				{WS, 13, 14},           // space
				{QUOTED_IDENT, 14, 27}, // "users table"
				{WS, 27, 28},           // space
				{KEYWORD, 28, 33},      // where
				{WS, 33, 34},           // space
				{IDENT, 34, 36},        // id
				{WS, 36, 37},           // space
				{OPERATOR, 37, 38},     // =
				{WS, 38, 39},           // space
				{NUMBER, 39, 40},       // 1
			},
		},
		{
			name:  "simple select with single line comment",
			input: "SELECT * FROM users where id = 1 -- comment here",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{NUMBER, 31, 32},   // 1
				{WS, 32, 33},       // space
				{COMMENT, 33, 48},  // -- comment here
			},
		},
		{
			name: "simple select with multi line comment",
			input: `SELECT * /* comment here */ FROM users where id = 1/* comment 
here */`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},             // SELECT
				{WS, 6, 7},                  // space
				{WILDCARD, 7, 8},            // *
				{WS, 8, 9},                  // space
				{MULTILINE_COMMENT, 9, 27},  // /* comment here */
				{WS, 27, 28},                // space
				{KEYWORD, 28, 32},           // FROM
				{WS, 32, 33},                // space
				{IDENT, 33, 38},             // users
				{WS, 38, 39},                // space
				{KEYWORD, 39, 44},           // where
				{WS, 44, 45},                // space
				{IDENT, 45, 47},             // id
				{WS, 47, 48},                // space
				{OPERATOR, 48, 49},          // =
				{WS, 49, 50},                // space
				{NUMBER, 50, 51},            // 1
				{MULTILINE_COMMENT, 51, 70}, // /* comment \nhere */
			},
		},
		{
			name:  "simple malformed select",
			input: "SELECT * FROM users where id = 1 and name = 'j",
			expected: []TokenSpec{
				{COMMAND, 0, 6},             // SELECT
				{WS, 6, 7},                  // space
				{WILDCARD, 7, 8},            // *
				{WS, 8, 9},                  // space
				{KEYWORD, 9, 13},            // FROM
				{WS, 13, 14},                // space
				{IDENT, 14, 19},             // users
				{WS, 19, 20},                // space
				{KEYWORD, 20, 25},           // where
				{WS, 25, 26},                // space
				{IDENT, 26, 28},             // id
				{WS, 28, 29},                // space
				{OPERATOR, 29, 30},          // =
				{WS, 30, 31},                // space
				{NUMBER, 31, 32},            // 1
				{WS, 32, 33},                // space
				{KEYWORD, 33, 36},           // and
				{WS, 36, 37},                // space
				{IDENT, 37, 41},             // name
				{WS, 41, 42},                // space
				{OPERATOR, 42, 43},          // =
				{WS, 43, 44},                // space
				{INCOMPLETE_STRING, 44, 46}, // 'j
			},
		},
		{
			name:  "truncated sql",
			input: "SELECT * FROM users where id = ",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
			},
		},
		{
			name:  "simple select with array of literals",
			input: "SELECT * FROM users where id in (1, '2')",
			expected: []TokenSpec{
				{COMMAND, 0, 6},       // SELECT
				{WS, 6, 7},            // space
				{WILDCARD, 7, 8},      // *
				{WS, 8, 9},            // space
				{KEYWORD, 9, 13},      // FROM
				{WS, 13, 14},          // space
				{IDENT, 14, 19},       // users
				{WS, 19, 20},          // space
				{KEYWORD, 20, 25},     // where
				{WS, 25, 26},          // space
				{IDENT, 26, 28},       // id
				{WS, 28, 29},          // space
				{KEYWORD, 29, 31},     // in
				{WS, 31, 32},          // space
				{PUNCTUATION, 32, 33}, // (
				{NUMBER, 33, 34},      // 1
				{PUNCTUATION, 34, 35}, // ,
				{WS, 35, 36},          // space
				{STRING, 36, 39},      // '2'
				{PUNCTUATION, 39, 40}, // )
			},
		},
		{
			name:  "dollar quoted function",
			input: "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users",
			expected: []TokenSpec{
				{COMMAND, 0, 6},                 // SELECT
				{WS, 6, 7},                      // space
				{DOLLAR_QUOTED_FUNCTION, 7, 55}, // $func$INSERT INTO table VALUES ('a', 1, 2)$func$
				{WS, 55, 56},                    // space
				{KEYWORD, 56, 60},               // FROM
				{WS, 60, 61},                    // space
				{IDENT, 61, 66},                 // users
			},
		},
		{
			name:  "dollar quoted string",
			input: "SELECT * FROM users where id = $tag$test$tag$",
			expected: []TokenSpec{
				{COMMAND, 0, 6},                // SELECT
				{WS, 6, 7},                     // space
				{WILDCARD, 7, 8},               // *
				{WS, 8, 9},                     // space
				{KEYWORD, 9, 13},               // FROM
				{WS, 13, 14},                   // space
				{IDENT, 14, 19},                // users
				{WS, 19, 20},                   // space
				{KEYWORD, 20, 25},              // where
				{WS, 25, 26},                   // space
				{IDENT, 26, 28},                // id
				{WS, 28, 29},                   // space
				{OPERATOR, 29, 30},             // =
				{WS, 30, 31},                   // space
				{DOLLAR_QUOTED_STRING, 31, 45}, // $tag$test$tag$
			},
		},
		{
			name:  "dollar quoted string",
			input: "SELECT * FROM users where id = $$test$$",
			expected: []TokenSpec{
				{COMMAND, 0, 6},                // SELECT
				{WS, 6, 7},                     // space
				{WILDCARD, 7, 8},               // *
				{WS, 8, 9},                     // space
				{KEYWORD, 9, 13},               // FROM
				{WS, 13, 14},                   // space
				{IDENT, 14, 19},                // users
				{WS, 19, 20},                   // space
				{KEYWORD, 20, 25},              // where
				{WS, 25, 26},                   // space
				{IDENT, 26, 28},                // id
				{WS, 28, 29},                   // space
				{OPERATOR, 29, 30},             // =
				{WS, 30, 31},                   // space
				{DOLLAR_QUOTED_STRING, 31, 39}, // $$test$$
			},
		},
		{
			name:  "numbered parameter",
			input: "SELECT * FROM users where id = $1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},                // SELECT
				{WS, 6, 7},                     // space
				{WILDCARD, 7, 8},               // *
				{WS, 8, 9},                     // space
				{KEYWORD, 9, 13},               // FROM
				{WS, 13, 14},                   // space
				{IDENT, 14, 19},                // users
				{WS, 19, 20},                   // space
				{KEYWORD, 20, 25},              // where
				{WS, 25, 26},                   // space
				{IDENT, 26, 28},                // id
				{WS, 28, 29},                   // space
				{OPERATOR, 29, 30},             // =
				{WS, 30, 31},                   // space
				{POSITIONAL_PARAMETER, 31, 33}, // $1
			},
		},
		{
			name:  "identifier with underscore and period",
			input: "SELECT * FROM users where user_id = 2 and users.name = 'j'",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 33},    // user_id
				{WS, 33, 34},       // space
				{OPERATOR, 34, 35}, // =
				{WS, 35, 36},       // space
				{NUMBER, 36, 37},   // 2
				{WS, 37, 38},       // space
				{KEYWORD, 38, 41},  // and
				{WS, 41, 42},       // space
				{IDENT, 42, 52},    // users.name
				{WS, 52, 53},       // space
				{OPERATOR, 53, 54}, // =
				{WS, 54, 55},       // space
				{STRING, 55, 58},   // 'j'
			},
		},
		{
			name:  "select with hex and octal numbers",
			input: "SELECT * FROM users where id = 0x123 and id = 0123",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{NUMBER, 31, 36},   // 0x123
				{WS, 36, 37},       // space
				{KEYWORD, 37, 40},  // and
				{WS, 40, 41},       // space
				{IDENT, 41, 43},    // id
				{WS, 43, 44},       // space
				{OPERATOR, 44, 45}, // =
				{WS, 45, 46},       // space
				{NUMBER, 46, 50},   // 0123
			},
		},
		{
			name:  "select with float numbers and scientific notation",
			input: "SELECT 1.2,1.2e3,1.2e-3,1.2E3,1.2E-3 FROM users",
			expected: []TokenSpec{
				{COMMAND, 0, 6},       // SELECT
				{WS, 6, 7},            // space
				{NUMBER, 7, 10},       // 1.2
				{PUNCTUATION, 10, 11}, // ,
				{NUMBER, 11, 16},      // 1.2e3
				{PUNCTUATION, 16, 17}, // ,
				{NUMBER, 17, 23},      // 1.2e-3
				{PUNCTUATION, 23, 24}, // ,
				{NUMBER, 24, 29},      // 1.2E3
				{PUNCTUATION, 29, 30}, // ,
				{NUMBER, 30, 36},      // 1.2E-3
				{WS, 36, 37},          // space
				{KEYWORD, 37, 41},     // FROM
				{WS, 41, 42},          // space
				{IDENT, 42, 47},       // users
			},
		},
		{
			name:  "select with double quoted identifier",
			input: `SELECT * FROM "users table"`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},        // SELECT
				{WS, 6, 7},             // space
				{WILDCARD, 7, 8},       // *
				{WS, 8, 9},             // space
				{KEYWORD, 9, 13},       // FROM
				{WS, 13, 14},           // space
				{QUOTED_IDENT, 14, 27}, // "users table"
			},
		},
		{
			name:  "select with double quoted identifier with period",
			input: `SELECT * FROM "public"."users table"`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},        // SELECT
				{WS, 6, 7},             // space
				{WILDCARD, 7, 8},       // *
				{WS, 8, 9},             // space
				{KEYWORD, 9, 13},       // FROM
				{WS, 13, 14},           // space
				{QUOTED_IDENT, 14, 36}, // "public"."users table"
			},
		},
		{
			name:  "select with escaped string",
			input: "SELECT * FROM users where id = 'j\\'s'",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{WS, 30, 31},       // space
				{STRING, 31, 37},   // 'j\\'s'
			},
		},
		{
			name:  "select with escaped string",
			input: "SELECT * FROM users where id =?",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{IDENT, 14, 19},    // users
				{WS, 19, 20},       // space
				{KEYWORD, 20, 25},  // where
				{WS, 25, 26},       // space
				{IDENT, 26, 28},    // id
				{WS, 28, 29},       // space
				{OPERATOR, 29, 30}, // =
				{OPERATOR, 30, 31}, // ?
			},
		},
		{
			name:  "select with bind parameter",
			input: "SELECT * FROM users where id = :id and name = :1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},          // SELECT
				{WS, 6, 7},               // space
				{WILDCARD, 7, 8},         // *
				{WS, 8, 9},               // space
				{KEYWORD, 9, 13},         // FROM
				{WS, 13, 14},             // space
				{IDENT, 14, 19},          // users
				{WS, 19, 20},             // space
				{KEYWORD, 20, 25},        // where
				{WS, 25, 26},             // space
				{IDENT, 26, 28},          // id
				{WS, 28, 29},             // space
				{OPERATOR, 29, 30},       // =
				{WS, 30, 31},             // space
				{BIND_PARAMETER, 31, 34}, // :id
				{WS, 34, 35},             // space
				{KEYWORD, 35, 38},        // and
				{WS, 38, 39},             // space
				{IDENT, 39, 43},          // name
				{WS, 43, 44},             // space
				{OPERATOR, 44, 45},       // =
				{WS, 45, 46},             // space
				{BIND_PARAMETER, 46, 48}, // :1
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSOracle)},
		},
		{
			name:  "select with bind parameter",
			input: "SELECT * FROM users where id = @id and name = @1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},          // SELECT
				{WS, 6, 7},               // space
				{WILDCARD, 7, 8},         // *
				{WS, 8, 9},               // space
				{KEYWORD, 9, 13},         // FROM
				{WS, 13, 14},             // space
				{IDENT, 14, 19},          // users
				{WS, 19, 20},             // space
				{KEYWORD, 20, 25},        // where
				{WS, 25, 26},             // space
				{IDENT, 26, 28},          // id
				{WS, 28, 29},             // space
				{OPERATOR, 29, 30},       // =
				{WS, 30, 31},             // space
				{BIND_PARAMETER, 31, 34}, // @id
				{WS, 34, 35},             // space
				{KEYWORD, 35, 38},        // and
				{WS, 38, 39},             // space
				{IDENT, 39, 43},          // name
				{WS, 43, 44},             // space
				{OPERATOR, 44, 45},       // =
				{WS, 45, 46},             // space
				{BIND_PARAMETER, 46, 48}, // @1
			},
		},
		{
			name:  "select with bind parameter using underscore",
			input: "SELECT * FROM users where id = @__my_id",
			expected: []TokenSpec{
				{COMMAND, 0, 6},          // SELECT
				{WS, 6, 7},               // space
				{WILDCARD, 7, 8},         // *
				{WS, 8, 9},               // space
				{KEYWORD, 9, 13},         // FROM
				{WS, 13, 14},             // space
				{IDENT, 14, 19},          // users
				{WS, 19, 20},             // space
				{KEYWORD, 20, 25},        // where
				{WS, 25, 26},             // space
				{IDENT, 26, 28},          // id
				{WS, 28, 29},             // space
				{OPERATOR, 29, 30},       // =
				{WS, 30, 31},             // space
				{BIND_PARAMETER, 31, 39}, // @__my_id
			},
		},
		{
			name:  "select with system variable",
			input: "SELECT @@VERSION AS SqlServerVersion",
			expected: []TokenSpec{
				{COMMAND, 0, 6},           // SELECT
				{WS, 6, 7},                // space
				{SYSTEM_VARIABLE, 7, 16},  // @@VERSION
				{WS, 16, 17},              // space
				{ALIAS_INDICATOR, 17, 19}, // AS
				{WS, 19, 20},              // space
				{IDENT, 20, 36},           // SqlServerVersion
			},
		},
		{
			name:  "SQL Server quoted identifier",
			input: "SELECT [user] FROM [test].[table] WHERE [id] = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},        // SELECT
				{WS, 6, 7},             // space
				{QUOTED_IDENT, 7, 13},  // [user]
				{WS, 13, 14},           // space
				{KEYWORD, 14, 18},      // FROM
				{WS, 18, 19},           // space
				{QUOTED_IDENT, 19, 33}, // [test].[table]
				{WS, 33, 34},           // space
				{KEYWORD, 34, 39},      // WHERE
				{WS, 39, 40},           // space
				{QUOTED_IDENT, 40, 44}, // [id]
				{WS, 44, 45},           // space
				{OPERATOR, 45, 46},     // =
				{WS, 46, 47},           // space
				{NUMBER, 47, 48},       // 1
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSSQLServer)},
		},
		{
			name:  "MySQL backtick quoted identifier",
			input: "SELECT `user` FROM `test`.`table` WHERE `id` = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},        // SELECT
				{WS, 6, 7},             // space
				{QUOTED_IDENT, 7, 13},  // `user`
				{WS, 13, 14},           // space
				{KEYWORD, 14, 18},      // FROM
				{WS, 18, 19},           // space
				{QUOTED_IDENT, 19, 33}, // `test`.`table`
				{WS, 33, 34},           // space
				{KEYWORD, 34, 39},      // WHERE
				{WS, 39, 40},           // space
				{QUOTED_IDENT, 40, 44}, // `id`
				{WS, 44, 45},           // space
				{OPERATOR, 45, 46},     // =
				{WS, 46, 47},           // space
				{NUMBER, 47, 48},       // 1
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSMySQL)},
		},
		{
			name:  "Tokenize function",
			input: "SELECT count(*) FROM users",
			expected: []TokenSpec{
				{COMMAND, 0, 6},       // SELECT
				{WS, 6, 7},            // space
				{FUNCTION, 7, 12},     // count
				{PUNCTUATION, 12, 13}, // (
				{WILDCARD, 13, 14},    // *
				{PUNCTUATION, 14, 15}, // )
				{WS, 15, 16},          // space
				{KEYWORD, 16, 20},     // FROM
				{WS, 20, 21},          // space
				{IDENT, 21, 26},       // users
			},
		},
		{
			name:  "Tokenize temp table",
			input: `SELECT * FROM #temp`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},  // SELECT
				{WS, 6, 7},       // space
				{WILDCARD, 7, 8}, // *
				{WS, 8, 9},       // space
				{KEYWORD, 9, 13}, // FROM
				{WS, 13, 14},     // space
				{IDENT, 14, 19},  // #temp
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSSQLServer)},
		},
		{
			name:  "MySQL comment",
			input: `SELECT * FROM users # comment`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},   // SELECT
				{WS, 6, 7},        // space
				{WILDCARD, 7, 8},  // *
				{WS, 8, 9},        // space
				{KEYWORD, 9, 13},  // FROM
				{WS, 13, 14},      // space
				{IDENT, 14, 19},   // users
				{WS, 19, 20},      // space
				{COMMENT, 20, 29}, // # comment
			},
			lexerOpts: []lexerOption{WithDBMS(DBMSMySQL)},
		},
		{
			name:  "drop table if exists",
			input: `DROP TABLE IF EXISTS users`,
			expected: []TokenSpec{
				{COMMAND, 0, 4},   // DROP
				{WS, 4, 5},        // space
				{KEYWORD, 5, 10},  // TABLE
				{WS, 10, 11},      // space
				{KEYWORD, 11, 13}, // IF
				{WS, 13, 14},      // space
				{KEYWORD, 14, 20}, // EXISTS
				{WS, 20, 21},      // space
				{IDENT, 21, 26},   // users
			},
		},
		{
			name:  "select only",
			input: "SELECT * FROM ONLY tab1 where id = 1",
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{WILDCARD, 7, 8},   // *
				{WS, 8, 9},         // space
				{KEYWORD, 9, 13},   // FROM
				{WS, 13, 14},       // space
				{KEYWORD, 14, 18},  // ONLY
				{WS, 18, 19},       // space
				{IDENT, 19, 23},    // tab1
				{WS, 23, 24},       // space
				{KEYWORD, 24, 29},  // where
				{WS, 29, 30},       // space
				{IDENT, 30, 32},    // id
				{WS, 32, 33},       // space
				{OPERATOR, 33, 34}, // =
				{WS, 34, 35},       // space
				{NUMBER, 35, 36},   // 1
			},
		},
		{
			name:  "extracts n'th element of JSON array",
			input: `SELECT data::json -> 2 FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 20},  // ->
				{WS, 20, 21},       // space
				{NUMBER, 21, 22},   // 2
				{WS, 22, 23},       // space
				{KEYWORD, 23, 27},  // FROM
				{WS, 27, 28},       // space
				{IDENT, 28, 33},    // users
			},
		},
		{
			name:  "extracts JSON object field with the given key",
			input: `SELECT data::json -> 'key' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 20},  // ->
				{WS, 20, 21},       // space
				{STRING, 21, 26},   // 'key'
				{WS, 26, 27},       // space
				{KEYWORD, 27, 31},  // FROM
				{WS, 31, 32},       // space
				{IDENT, 32, 37},    // users
			},
		},
		{
			name:  "extracts n'th element of JSON array, as text",
			input: `SELECT data::json ->> 2 FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 21},  // ->>
				{WS, 21, 22},       // space
				{NUMBER, 22, 23},   // 2
				{WS, 23, 24},       // space
				{KEYWORD, 24, 28},  // FROM
				{WS, 28, 29},       // space
				{IDENT, 29, 34},    // users
			},
		},
		{
			name:  "extracts JSON object field with the given key, as text",
			input: `SELECT data::json ->> 'key' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 21},  // ->>
				{WS, 21, 22},       // space
				{STRING, 22, 27},   // 'key'
				{WS, 27, 28},       // space
				{KEYWORD, 28, 32},  // FROM
				{WS, 32, 33},       // space
				{IDENT, 33, 38},    // users
			},
		},
		{
			name:  "extracts JSON sub-object at the specified path",
			input: `SELECT data::json #> '{key1,key2}' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 20},  // #>
				{WS, 20, 21},       // space
				{STRING, 21, 34},   // '{key1,key2}'
				{WS, 34, 35},       // space
				{KEYWORD, 35, 39},  // FROM
				{WS, 39, 40},       // space
				{IDENT, 40, 45},    // users
			},
		},
		{
			name:  "extracts JSON sub-object at the specified path as text",
			input: `SELECT data::json #>> '{key1,key2}' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 17},    // json
				{WS, 17, 18},       // space
				{JSON_OP, 18, 21},  // #>>
				{WS, 21, 22},       // space
				{STRING, 22, 35},   // '{key1,key2}'
				{WS, 35, 36},       // space
				{KEYWORD, 36, 40},  // FROM
				{WS, 40, 41},       // space
				{IDENT, 41, 46},    // users
			},
		},
		{
			name:  "JSON path return any item for the specified JSON value",
			input: `SELECT data::jsonb @? '$.a[*] ? (@ > 2)' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 18},    // jsonb
				{WS, 18, 19},       // space
				{JSON_OP, 19, 21},  // @?
				{WS, 21, 22},       // space
				{STRING, 22, 40},   // '$.a[*] ? (@ > 2)'
				{WS, 40, 41},       // space
				{KEYWORD, 41, 45},  // FROM
				{WS, 45, 46},       // space
				{IDENT, 46, 51},    // users
			},
		},
		{
			name:  "JSON path predicate check for the specified JSON value",
			input: `SELECT data::jsonb @@ '$.a[*] > 2' FROM users`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},    // SELECT
				{WS, 6, 7},         // space
				{IDENT, 7, 11},     // data
				{OPERATOR, 11, 13}, // ::
				{IDENT, 13, 18},    // jsonb
				{WS, 18, 19},       // space
				{JSON_OP, 19, 21},  // @@
				{WS, 21, 22},       // space
				{STRING, 22, 34},   // '$.a[*] > 2'
				{WS, 34, 35},       // space
				{KEYWORD, 35, 39},  // FROM
				{WS, 39, 40},       // space
				{IDENT, 40, 45},    // users
			},
		},
		{
			name:  "create procedure",
			input: `CREATE PROCEDURE test_proc (IN param1 INT, OUT param2 VARCHAR(255))`,
			expected: []TokenSpec{
				{COMMAND, 0, 6},         // CREATE
				{WS, 6, 7},              // space
				{PROC_INDICATOR, 7, 16}, // PROCEDURE
				{WS, 16, 17},            // space
				{IDENT, 17, 26},         // test_proc
				{WS, 26, 27},            // space
				{PUNCTUATION, 27, 28},   // (
				{KEYWORD, 28, 30},       // IN
				{WS, 30, 31},            // space
				{IDENT, 31, 37},         // param1
				{WS, 37, 38},            // space
				{IDENT, 38, 41},         // INT
				{PUNCTUATION, 41, 42},   // ,
				{WS, 42, 43},            // space
				{KEYWORD, 43, 46},       // OUT
				{WS, 46, 47},            // space
				{IDENT, 47, 53},         // param2
				{WS, 53, 54},            // space
				{FUNCTION, 54, 61},      // VARCHAR
				{PUNCTUATION, 61, 62},   // (
				{NUMBER, 62, 65},        // 255
				{PUNCTUATION, 65, 66},   // )
				{PUNCTUATION, 66, 67},   // )
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			tokens := lexer.ScanAll()

			if len(tokens) != len(tt.expected) {
				t.Errorf("got %d tokens, want %d", len(tokens), len(tt.expected))
				return
			}

			for i, want := range tt.expected {
				got := tokens[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Start != want.Start {
					t.Errorf("token[%d] got start %v, want %v", i, got.Start, want.Start)
				}
				if got.End != want.End {
					t.Errorf("token[%d] got end %v, want %v", i, got.End, want.End)
				}
			}
		})
	}
}

func TestLexerIdentifierWithDigits(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []TokenSpec
		expectedDigits [][]int
		lexerOpts      []lexerOption
	}{
		{
			input: `abc123`,
			expectedTokens: []TokenSpec{
				{IDENT, 0, 6},
			},
			expectedDigits: [][]int{
				{3, 4, 5},
			},
		},
		{
			input: `abc123def456`,
			expectedTokens: []TokenSpec{
				{IDENT, 0, 12},
			},
			expectedDigits: [][]int{
				{3, 4, 5, 9, 10, 11},
			},
		},
		{
			input: `abc123 bc12d456, "c123ef"`,
			expectedTokens: []TokenSpec{
				{IDENT, 0, 6},
				{WS, 6, 7},
				{IDENT, 7, 15},
				{PUNCTUATION, 15, 16},
				{WS, 16, 17},
				{QUOTED_IDENT, 17, 25},
			},
			expectedDigits: [][]int{
				{3, 4, 5},
				nil,
				{9, 10, 12, 13, 14},
				nil,
				nil,
				{19, 20, 21},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			tokens := lexer.ScanAll()

			if len(tokens) != len(tt.expectedTokens) {
				t.Errorf("got %d tokens, want %d", len(tokens), len(tt.expectedTokens))
				return
			}

			for i, want := range tt.expectedTokens {
				got := tokens[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Start != want.Start {
					t.Errorf("token[%d] got start %v, want %v", i, got.Start, want.Start)
				}
				if got.End != want.End {
					t.Errorf("token[%d] got end %v, want %v", i, got.End, want.End)
				}
			}

			for i, digits := range tt.expectedDigits {
				if digits == nil {
					if tokens[i].ExtraInfo != nil && tokens[i].ExtraInfo.Digits != nil {
						t.Errorf("token[%d] got digits, want nil", i)
					}
					continue
				}
				got := tokens[i].ExtraInfo.Digits
				if len(got) != len(digits) {
					t.Errorf("token[%d] got %d digits, want %d", i, len(got), len(digits))
					continue
				}
				for j, digit := range digits {
					if got[j] != digit {
						t.Errorf("token[%d] got digit[%d] %d, want %d", i, j, got[j], digit)
					}
				}
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
				{IDENT, 0, 3},
			},
		},
		{
			input: `Descripció_CAT`,
			expected: []TokenSpec{
				{IDENT, 0, 15},
			},
		},
		{
			input: `世界`,
			expected: []TokenSpec{
				{IDENT, 0, 6},
			},
		},
		{
			input: `こんにちは`,
			expected: []TokenSpec{
				{IDENT, 0, 15},
			},
		},
		{
			input: `안녕하세요`,
			expected: []TokenSpec{
				{IDENT, 0, 15},
			},
		},
		{
			input: `über`,
			expected: []TokenSpec{
				{IDENT, 0, 5},
			},
		},
		{
			input: `résumé`,
			expected: []TokenSpec{
				{IDENT, 0, 8},
			},
		},
		{
			input: `"über"`,
			expected: []TokenSpec{
				{QUOTED_IDENT, 0, 7},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lexer := New(tt.input, tt.lexerOpts...)
			tokens := lexer.ScanAll()

			if len(tokens) != len(tt.expected) {
				t.Errorf("got %d tokens, want %d", len(tokens), len(tt.expected))
				return
			}

			for i, want := range tt.expected {
				got := tokens[i]
				if got.Type != want.Type {
					t.Errorf("token[%d] got type %v, want %v", i, got.Type, want.Type)
				}
				if got.Start != want.Start {
					t.Errorf("token[%d] got start %v, want %v", i, got.Start, want.Start)
				}
				if got.End != want.End {
					t.Errorf("token[%d] got end %v, want %v", i, got.End, want.End)
				}
			}
		})
	}
}

func ExampleLexer() {
	query := "SELECT * FROM users WHERE id = 1"
	lexer := New(query)
	tokens := lexer.ScanAll()
	fmt.Println(tokens)
}
