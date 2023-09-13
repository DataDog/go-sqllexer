package sqllexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscationAndNormalization(t *testing.T) {
	tests := []struct {
		input             string
		expected          string
		statementMetadata StatementMetadata
	}{
		{
			input:    "SELECT 1",
			expected: "SELECT ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			/*dddbs='orders-mysql',dde='dbm-agent-integration',ddps='orders-app',ddpv='7825a16',traceparent='00-000000000000000068e229d784ee697c-569d1b940c1fb3ac-00'*/
			/* date='12%2F31',key='val' */
			SELECT * FROM users WHERE id = 1`,
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{"/*dddbs='orders-mysql',dde='dbm-agent-integration',ddps='orders-app',ddpv='7825a16',traceparent='00-000000000000000068e229d784ee697c-569d1b940c1fb3ac-00'*/", "/* date='12%2F31',key='val' */"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input:    "SELECT * FROM users WHERE id IN (1, 2) and name IN ARRAY[3, 4]",
			expected: "SELECT * FROM users WHERE id IN ( ? ) and name IN ARRAY [ ? ]",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			SELECT h.id, h.org_id, h.name, ha.name as alias, h.created 
			FROM vs?.host h 
				JOIN vs?.host_alias ha on ha.host_id = h.id 
			WHERE ha.org_id = 1 AND ha.name = ANY ('3', '4')
			`,
			expected: "SELECT h.id, h.org_id, h.name, ha.name, h.created FROM vs?.host h JOIN vs?.host_alias ha on ha.host_id = h.id WHERE ha.org_id = ? AND ha.name = ANY ( ? )",
			statementMetadata: StatementMetadata{
				Tables:   []string{"vs?.host", "vs?.host_alias"},
				Comments: []string{},
				Commands: []string{"SELECT", "JOIN"},
			},
		},
		{
			input:    "/* this is a comment */ SELECT * FROM users WHERE id = '2'",
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{"/* this is a comment */"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			/* this is a 
multiline comment */
			SELECT * FROM users /* comment comment */ WHERE id = 'XXX'
			-- this is another comment
			`,
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{"/* this is a \nmultiline comment */", "/* comment comment */", "-- this is another comment"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input:    "SELECT u.id as ID, u.name as Name FROM users as u WHERE u.id = 1",
			expected: "SELECT u.id, u.name FROM users WHERE u.id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
	}

	obfuscator := NewObfuscator(
		WithReplaceDigits(true),
		WithDollarQuotedFunc(true),
	)

	normalizer := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithKeepSQLAlias(false),
	)

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			got, statementMetadata, err := ObfuscateAndNormalize(test.input, obfuscator, normalizer)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, got)
			assert.Equal(t, &test.statementMetadata, statementMetadata)
		})
	}
}
