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
		// {
		// 	// boolean and null
		// 	input:    `SELECT * FROM (SELECT customer_id, product_id, amount FROM order_details) AS SourceTable PIVOT (SUM(amount) FOR product_id IN ([1], [2], [3])) AS PivotTable;`,
		// 	expected: `SELECT * FROM ( SELECT customer_id, product_id, amount FROM order_details ) PIVOT ( SUM ( amount ) FOR product_id IN ( ? ) )`,
		// 	statementMetadata: StatementMetadata{
		// 		Tables:     []string{"users"},
		// 		Comments:   []string{},
		// 		Commands:   []string{"SELECT"},
		// 		Procedures: []string{},
		// 		Size:       11,
		// 	},
		// },
		{
			input:    "SELECT 1",
			expected: "SELECT ?",
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       6,
			},
		},
		{
			input: `
			/*dddbs='orders-mysql',dde='dbm-agent-integration',ddps='orders-app',ddpv='7825a16',traceparent='00-000000000000000068e229d784ee697c-569d1b940c1fb3ac-00'*/
			/* date='12%2F31',key='val' */
			SELECT * FROM users WHERE id = 1`,
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{"/*dddbs='orders-mysql',dde='dbm-agent-integration',ddps='orders-app',ddpv='7825a16',traceparent='00-000000000000000068e229d784ee697c-569d1b940c1fb3ac-00'*/", "/* date='12%2F31',key='val' */"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       196,
			},
		},
		{
			input:    "SELECT * FROM users WHERE id IN (1, 2) and name IN ARRAY[3, 4]",
			expected: "SELECT * FROM users WHERE id IN ( ? ) and name IN ARRAY [ ? ]",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
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
				Tables:     []string{"vs?.host", "vs?.host_alias"},
				Comments:   []string{},
				Commands:   []string{"SELECT", "JOIN"},
				Procedures: []string{},
				Size:       32,
			},
		},
		{
			input:    "/* this is a comment */ SELECT * FROM users WHERE id = '2'",
			expected: "SELECT * FROM users WHERE id = ?",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{"/* this is a comment */"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       34,
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
				Tables:     []string{"users"},
				Comments:   []string{"/* this is a \nmultiline comment */", "/* comment comment */", "-- this is another comment"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       92,
			},
		},
		{
			input:    "SELECT u.id as ID, u.name as Name FROM users as u WHERE u.id = 1",
			expected: "SELECT u.id, u.name FROM users WHERE u.id = ?",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			input:    "SELECT TRUNC(SYSDATE@!) from dual",
			expected: "SELECT TRUNC ( SYSDATE@! ) from dual",
			statementMetadata: StatementMetadata{
				Tables:     []string{"dual"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       10,
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
				Tables:     []string{"v$sql"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSOracle),
			},
		},
		{
			input:    "SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > 85",
			expected: `SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"SYS.DBA_TABLESPACE_USAGE_METRICS"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       38,
			},
		},
		{
			input:    "SELECT dbms_lob.substr(sql_fulltext, 4000, 1) sql_fulltext FROM sys.dd_session",
			expected: "SELECT dbms_lob.substr ( sql_fulltext, ?, ? ) sql_fulltext FROM sys.dd_session",
			statementMetadata: StatementMetadata{
				Tables:     []string{"sys.dd_session"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       20,
			},
		},
		{
			input:    "begin execute immediate 'alter session set sql_trace=true'; end;",
			expected: "begin execute immediate ?; end",
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"BEGIN", "EXECUTE"},
				Procedures: []string{},
				Size:       12,
			},
		},
		{
			// double quoted table name
			input:    `SELECT * FROM "public"."users" WHERE id = 1`,
			expected: `SELECT * FROM public.users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"public.users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       18,
			},
		},
		{
			// [] quoted table name
			input:    `SELECT * FROM [public].[users] WHERE id = 1`,
			expected: `SELECT * FROM public.users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"public.users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       18,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
			},
		},
		// test for .Net tracer
		{
			// [] quoted table name
			input:    `SELECT * FROM [public].[users] WHERE id = 1`,
			expected: `SELECT * FROM public.users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"public.users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       18,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServerAlias1),
			},
		},
		// test for Java tracer
		{
			// [] quoted table name
			input:    `SELECT * FROM [public].[users] WHERE id = 1`,
			expected: `SELECT * FROM public.users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"public.users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       18,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServerAlias2),
			},
		},
		{
			input:    `CREATE PROCEDURE TestProc AS SELECT * FROM users`,
			expected: `CREATE PROCEDURE TestProc AS SELECT * FROM users`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"CREATE", "SELECT"},
				Procedures: []string{"TestProc"},
				Size:       25,
			},
		},
		{
			input:    "SELECT $func$SELECT * FROM table WHERE ID in ('a', 1, 2)$func$ FROM users",
			expected: "SELECT $func$SELECT * FROM table WHERE ID in ( ? )$func$ FROM users",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			input:    "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users",
			expected: "SELECT $func$INSERT INTO table VALUES ( ? )$func$ FROM users",
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			input:    `select "user_id" from "public"."users"`,
			expected: `select user_id from public.users`,
			statementMetadata: StatementMetadata{
				Tables:     []string{`public.users`},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       18,
			},
		},
		{
			// boolean and null
			input:    `SELECT * FROM users where active = true and deleted is FALSE and age is not null and test is NULL`,
			expected: `SELECT * FROM users where active = ? and deleted is ? and age is not ? and test is ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			input:    "SELECT file#, name, bytes, status FROM V$DATAFILE",
			expected: "SELECT file#, name, bytes, status FROM V$DATAFILE",
			statementMetadata: StatementMetadata{
				Tables:     []string{"V$DATAFILE"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       16,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSOracle),
			},
		},
		{
			input:    `SELECT * FROM users WHERE id = 1 # this is a comment`,
			expected: `SELECT * FROM users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{"# this is a comment"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       30,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSMySQL),
			},
		},
		{
			input:    `SELECT * FROM [世界].[测试] WHERE id = 1`,
			expected: `SELECT * FROM 世界.测试 WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"世界.测试"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       19,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
			},
		},
		{
			input: `SET NOCOUNT ON
			IF @@OPTIONS & 512 > 0
			RAISERROR ('Current user has SET NOCOUNT turned on.', 1, 1)`,
			expected: `SET NOCOUNT ON IF @@OPTIONS & ? > ? RAISERROR ( ? )`,
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{},
				Procedures: []string{},
				Size:       0,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
			},
		},
		{
			input: `
			WITH SILENCES AS (
				SELECT LOWER(BASE_TABLE_NAME), CREATED_DT, SILENCE_UNTIL_DT, REASON
					,ROW_NUMBER() OVER (PARTITION BY LOWER(BASE_TABLE_NAME) ORDER BY CREATED_DT DESC) AS ROW_NUMBER
				FROM REPORTING.GENERAL.SOME_TABLE
				WHERE CONTAINS('us1', LOWER(DATACENTER_LABEL))
			  )
			  SELECT * FROM SILENCES WHERE ROW_NUMBER = 1;`,
			expected: `WITH SILENCES AS ( SELECT LOWER ( BASE_TABLE_NAME ), CREATED_DT, SILENCE_UNTIL_DT, REASON, ROW_NUMBER ( ) OVER ( PARTITION BY LOWER ( BASE_TABLE_NAME ) ORDER BY CREATED_DT DESC ) FROM REPORTING.GENERAL.SOME_TABLE WHERE CONTAINS ( ?, LOWER ( DATACENTER_LABEL ) ) ) SELECT * FROM SILENCES WHERE ROW_NUMBER = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"REPORTING.GENERAL.SOME_TABLE"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       34,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input:    `USE WAREHOUSE "SOME_WAREHOUSE";`,
			expected: `USE WAREHOUSE SOME_WAREHOUSE`, // double quoted identifier are not replaced
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"USE"},
				Procedures: []string{},
				Size:       3,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input: `SELECT 1 FROM REPORTING.GENERAL.SOME_RANDOM_TABLE
			WHERE BASE_TABLE_NAME='xxx_ttt_zzz_v1'
			AND DATACENTER_LABEL='us3'
			AND CENSUS_ELEMENT_ID='bef52c3f-788f-4fb3-b116-a05a1c4a9792';`,
			expected: `SELECT ? FROM REPORTING.GENERAL.SOME_RANDOM_TABLE WHERE BASE_TABLE_NAME = ? AND DATACENTER_LABEL = ? AND CENSUS_ELEMENT_ID = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"REPORTING.GENERAL.SOME_RANDOM_TABLE"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       41,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input: `COPY INTO  REPORTING.GENERAL.MY_TABLE
			(FEATURE,DESCRIPTION,COVERAGE,DATE_PARTITION)
			FROM (SELECT $1,$2,$3,TO_TIMESTAMP('2023-12-14 00:00:00') FROM @REPORTING.GENERAL.SOME_DESCRIPTIONS/external_data/)
			file_format=(type=CSV SKIP_HEADER=1 FIELD_OPTIONALLY_ENCLOSED_BY='\"' ESCAPE_UNENCLOSED_FIELD='\\' FIELD_DELIMITER=',' )
			;`,
			expected: `COPY INTO REPORTING.GENERAL.MY_TABLE ( FEATURE, DESCRIPTION, COVERAGE, DATE_PARTITION ) FROM ( SELECT $1, $2, $3, TO_TIMESTAMP ( ? ) FROM @REPORTING.GENERAL.SOME_DESCRIPTIONS/external_data/ ) file_format = ( type = CSV SKIP_HEADER = ? FIELD_OPTIONALLY_ENCLOSED_BY = ? ESCAPE_UNENCLOSED_FIELD = ? FIELD_DELIMITER = ? )`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"REPORTING.GENERAL.MY_TABLE", "@REPORTING.GENERAL.SOME_DESCRIPTIONS/external_data/"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       83,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input: `SELECT EXISTS(
				SELECT * FROM REPORTING.INFORMATION_SCHEMA.TABLES
				WHERE table_schema='XXX_YYY'
				AND table_name='ABC'
				AND table_type='EXTERNAL TABLE'
			);`,
			expected: `SELECT EXISTS ( SELECT * FROM REPORTING.INFORMATION_SCHEMA.TABLES WHERE table_schema = ? AND table_name = ? AND table_type = ? )`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"REPORTING.INFORMATION_SCHEMA.TABLES"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       41,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input:    `ALTER EXTERNAL TABLE REPORTING.TEST.MY_TABLE REFRESH '2024_01_15';`,
			expected: `ALTER EXTERNAL TABLE REPORTING.TEST.MY_TABLE REFRESH ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"REPORTING.TEST.MY_TABLE"},
				Comments:   []string{},
				Commands:   []string{"ALTER"},
				Procedures: []string{},
				Size:       28,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSnowflake),
			},
		},
		{
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb <@ '{"a": 1, "b": 2}'::jsonb`,
			expected: `SELECT * FROM users where ? :: jsonb <@ '{"a": 1, "b": 2}' :: jsonb`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			input:    `DELETE FROM [discount]  WHERE [description]=@1`,
			expected: `DELETE FROM discount WHERE description = @1`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"discount"},
				Comments:   []string{},
				Commands:   []string{"DELETE"},
				Procedures: []string{},
				Size:       14,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
			},
		},
		{
			input:    `/*dddbs='mydb',ddpv='1.2.3'*/ ( @p1 bigint ) SELECT * from dbm_user WHERE id = @p1`,
			expected: `SELECT * from dbm_user WHERE id = @p1`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"dbm_user"},
				Comments:   []string{"/*dddbs='mydb',ddpv='1.2.3'*/"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       43,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSSQLServer),
			},
		},
		{
			input:    `SELECT pk, updatedAt, createdAt, name, description, isAutoCreated, autoCreatedFeaturePk FROM FeatureStrategyGroup WHERE FeatureStrategyGroup.autoCreatedFeaturePk IN ( ? )`,
			expected: `SELECT pk, updatedAt, createdAt, name, description, isAutoCreated, autoCreatedFeaturePk FROM FeatureStrategyGroup WHERE FeatureStrategyGroup.autoCreatedFeaturePk IN ( ? )`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"FeatureStrategyGroup"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       26,
			},
			lexerOpts: []lexerOption{
				WithDBMS("postgres"),
			},
		},
	}

	obfuscator := NewObfuscator(
		WithReplaceDigits(true),
		WithReplaceBoolean(true),
		WithReplaceNull(true),
		WithDollarQuotedFunc(true),
		WithKeepJsonPath(true),
	)

	normalizer := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithCollectProcedures(true),
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
