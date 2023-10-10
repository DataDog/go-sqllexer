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
		lexerOpts         []lexerOption
	}{
		{
			input:    "SELECT 1",
			expected: "SELECT ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     6,
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
				Size:     196,
			},
		},
		{
			input:    "SELECT * FROM users WHERE id IN (1, 2) and name IN ARRAY[3, 4]",
			expected: "SELECT * FROM users WHERE id IN ( ? ) and name IN ARRAY [ ? ]",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     11,
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
				Size:     32,
			},
		},
		{
			input:    "/* this is a comment */ SELECT * FROM users WHERE id = '2'",
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{"/* this is a comment */"},
				Commands: []string{"SELECT"},
				Size:     34,
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
				Size:     92,
			},
		},
		{
			input:    "SELECT u.id as ID, u.name as Name FROM users as u WHERE u.id = 1",
			expected: "SELECT u.id, u.name FROM users WHERE u.id = ?",
			statementMetadata: StatementMetadata{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     11,
			},
		},
		{
			input:    "SELECT TRUNC(SYSDATE@!) from dual",
			expected: "SELECT TRUNC ( SYSDATE @! ) from dual",
			statementMetadata: StatementMetadata{
				Tables:   []string{"dual"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     10,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSOracle),
			},
		},
		{
			input: `
			select sql_fulltext from v$sql where force_matching_signature = 1033183797897134935
			GROUP BY c.name, force_matching_signature, plan_hash_value
			HAVING MAX(last_active_time) > sysdate - :seconds/24/60/60
			FETCH FIRST :limit ROWS ONLY`,
			expected: "select sql_fulltext from v$sql where force_matching_signature = ? GROUP BY c.name, force_matching_signature, plan_hash_value HAVING MAX ( last_active_time ) > sysdate - :seconds / ? / ? / ? FETCH FIRST :limit ROWS ONLY",
			statementMetadata: StatementMetadata{
				Tables:   []string{"v$sql"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     11,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSOracle),
			},
		},
		{
			input:    "SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > 85",
			expected: `SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > ?`,
			statementMetadata: StatementMetadata{
				Tables:   []string{"SYS.DBA_TABLESPACE_USAGE_METRICS"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     38,
			},
		},
		{
			input:    "SELECT dbms_lob.substr(sql_fulltext, 4000, 1) sql_fulltext FROM sys.dd_session",
			expected: "SELECT dbms_lob.substr ( sql_fulltext, ?, ? ) sql_fulltext FROM sys.dd_session",
			statementMetadata: StatementMetadata{
				Tables:   []string{"sys.dd_session"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     20,
			},
		},
		{
			input:    "begin execute immediate 'alter session set sql_trace=true'; end;",
			expected: "begin execute immediate ? ; end ;",
			statementMetadata: StatementMetadata{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{"BEGIN", "EXECUTE"},
				Size:     12,
			},
		},
		{
			// double quoted table name
			input:    `SELECT * FROM "public"."users" WHERE id = 1`,
			expected: `SELECT * FROM "public"."users" WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:   []string{`"public"."users"`},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     22,
			},
		},
		{
			// [] quoted table name
			input:    `SELECT * FROM [public].[users] WHERE id = 1`,
			expected: `SELECT * FROM [public].[users] WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:   []string{"[public].[users]"},
				Comments: []string{},
				Commands: []string{"SELECT"},
				Size:     22,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
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
			got, statementMetadata, err := ObfuscateAndNormalize(test.input, obfuscator, normalizer, test.lexerOpts...)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, got)
			assert.Equal(t, &test.statementMetadata, statementMetadata)
		})
	}
}
