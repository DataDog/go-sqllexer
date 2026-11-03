package sqllexer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Db2 ordinary strings use doubled quotes and treat backslashes literally.
// Check boundaries so a closing quote cannot swallow a comma or expose the next value.
func TestDb2StringBoundaries(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []TokenSpec
	}{
		{`'path\','probe_alpha'`, []TokenSpec{{STRING, `'path\'`}, {PUNCTUATION, ","}, {STRING, `'probe_alpha'`}}},
		{`'probe_alpha''suffix'`, []TokenSpec{{STRING, `'probe_alpha''suffix'`}}},
		{`'path\''秘密',17`, []TokenSpec{{STRING, `'path\''秘密'`}, {PUNCTUATION, ","}, {NUMBER, "17"}}},
		{`''||''''`, []TokenSpec{{STRING, `''`}, {OPERATOR, "||"}, {STRING, `''''`}}},
		{`'path\\',?`, []TokenSpec{{STRING, `'path\\'`}, {PUNCTUATION, ","}, {OPERATOR, "?"}}},
		{`'unfinished\`, []TokenSpec{{INCOMPLETE_STRING, `'unfinished\`}}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			lexer := New(tc.input, WithDBMS(DBMSType("ibm_db2")))
			for _, want := range tc.want {
				got := lexer.Scan()
				require.Equal(t, want.Type, got.Type)
				require.Equal(t, want.Value, got.Value)
			}
			require.Equal(t, EOF, lexer.Scan().Type)
		})
	}
}

// Literal contents must neither survive obfuscation nor change the normalized
// text from which the Agent's callers compute query signatures.
func TestDb2LiteralNormalization(t *testing.T) {
	for _, tc := range []struct {
		first, second, want string
	}{
		{`VALUES 'path\', 'probe_alpha'`, `VALUES 'other', 'probe_beta'`, `VALUES ?, ?`},
		{`VALUES 'probe_alpha''suffix'`, `VALUES 'probe_beta'`, `VALUES ?`},
		{`VALUES 'path\''秘密', 'probe_alpha'`, `VALUES 'plain', 'probe_beta'`, `VALUES ?, ?`},
		{`VALUES ''`, `VALUES ''''`, `VALUES ?`},
		{`VALUES 'path\\'`, `VALUES 'plain'`, `VALUES ?`},
		{`VALUES 'a' || 'b'`, `VALUES 'c' || 'd'`, `VALUES ? || ?`},
	} {
		t.Run(tc.first, func(t *testing.T) {
			for _, query := range []string{tc.first, tc.second} {
				got, _, err := ObfuscateAndNormalize(query, NewObfuscator(), NewNormalizer(), WithDBMS(DBMSType("ibm_db2")))
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
