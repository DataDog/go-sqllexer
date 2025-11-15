package sqllexer

import (
	"testing"
)

func FuzzNormalizer(f *testing.F) {
	// Add complex SQL patterns for different DBMS
	addComplexTestCases(f)
	addObfuscationTestCases(f)

	normalizer := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithCollectProcedures(true),
	)

	f.Fuzz(func(t *testing.T, input string, dbmsType string) {
		// Try different DBMS types with each input
		opts := []lexerOption{WithDBMS(DBMSType(dbmsType))}
		_, _, err := normalizer.Normalize(input, opts...)
		if err != nil {
			t.Errorf("error normalizing input: %v", err)
		}
	})
}

func FuzzObfuscatorAndNormalizer(f *testing.F) {
	// Test the combined obfuscation and normalization
	addComplexTestCases(f)
	addObfuscationTestCases(f)

	normalizer := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithCollectProcedures(true),
	)
	obfuscator := NewObfuscator(
		WithReplaceDigits(true),
	)

	f.Fuzz(func(t *testing.T, input string, dbmsType string) {
		opts := []lexerOption{WithDBMS(DBMSType(dbmsType))}
		_, _, err := ObfuscateAndNormalize(input, obfuscator, normalizer, opts...)
		if err != nil {
			t.Errorf("error obfuscating and normalizing input: %v", err)
		}
	})
}

func addComplexTestCases(f *testing.F) {
	// PostgreSQL specific patterns
	postgresPatterns := []string{
		// Schema qualified objects with quotes
		`SELECT * FROM "public"."users" u CROSS JOIN LATERAL (SELECT * FROM "schema"."table") sub`,

		// Custom operators
		`SELECT * FROM users WHERE name <-> 'pattern' < 0.7`,

		// Array and JSON operations
		`SELECT array[1,2,3] @> array[1,2]`,
		`SELECT data->>'name' FROM users WHERE data @? '$.age > 20'`,

		// WITH ORDINALITY
		`SELECT * FROM unnest(ARRAY['a','b','c']) WITH ORDINALITY`,

		// Complex type casts
		`SELECT CAST(CAST(col AS text) AS integer[])`,

		// Dollar quoted strings
		`SELECT $func$BEGIN RETURN 1; END$func$`,
		`SELECT $tag$string with " and ' quotes$tag$`,
	}

	// SQL Server specific patterns
	sqlServerPatterns := []string{
		// Bracketed identifiers
		`SELECT * FROM [server].[database].[schema].[table]`,

		// CROSS/OUTER APPLY
		`SELECT * FROM users CROSS APPLY (SELECT * FROM table(value)) t`,

		// Table hints
		`SELECT * FROM users WITH (NOLOCK, INDEX(idx))`,

		// TOP with ties
		`SELECT TOP 1 WITH TIES * FROM users ORDER BY score`,

		// OUTPUT clause
		`DELETE users OUTPUT deleted.* INTO audit_table`,

		// PIVOT/UNPIVOT
		`SELECT * FROM users PIVOT (SUM(val) FOR col IN ([A],[B],[C])) p`,
	}

	// MySQL specific patterns
	mysqlPatterns := []string{
		// Backtick identifiers
		"SELECT * FROM `database`.`table`",

		// STRAIGHT_JOIN
		"SELECT STRAIGHT_JOIN * FROM t1 INNER JOIN t2",

		// Complex index hints
		"SELECT /*+ BKA(t1) NO_BKA(t2) */ * FROM t1 USE INDEX (idx1, idx2)",

		// Group concat with order by
		"SELECT GROUP_CONCAT(name ORDER BY id SEPARATOR ';')",

		// MySQL comment
		"#1",
	}

	// Oracle specific patterns
	oraclePatterns := []string{
		// Connect by
		`SELECT * FROM users START WITH id = 1 CONNECT BY PRIOR parent_id = id`,

		// Hierarchical queries
		`SELECT * FROM users CONNECT BY NOCYCLE PRIOR id = parent_id`,

		// MINUS operator
		`SELECT * FROM t1 MINUS SELECT * FROM t2`,

		// Row limiting with offset
		`SELECT * FROM users OFFSET 5 ROWS FETCH FIRST 10 ROWS ONLY`,

		// Flashback queries
		`SELECT * FROM users AS OF TIMESTAMP SYSTIMESTAMP - INTERVAL '1' DAY`,
	}

	// Snowflake specific patterns
	snowflakePatterns := []string{
		// Semi-structured data
		`SELECT PARSE_JSON('{"a":1}'):a::string`,

		// Pattern matching
		`SELECT REGEXP_SUBSTR(col, '[A-Z]+', 1, 1, 'i')`,

		// Time travel
		`SELECT * FROM users AT(TIMESTAMP => 'yesterday'::timestamp)`,
		`SELECT * FROM users BEFORE(STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')`,

		// External tables
		`SELECT $1, $2 FROM @mystage/file.csv`,
	}

	// SQLite specific patterns
	sqlitePatterns := []string{
		`SELECT * FROM pragma_table_info('users')`,
		`INSERT OR REPLACE INTO kv_store(key, value) VALUES(:key, json_extract($payload, '$.value'))`,
		`INSERT INTO logs VALUES($ns::var, $env(config), $ns::name(sub))`,
		"CREATE TABLE IF NOT EXISTS logs (id INTEGER PRIMARY KEY, payload TEXT) WITHOUT ROWID",
		"WITH ranked AS (SELECT *, row_number() OVER (PARTITION BY type ORDER BY created_at DESC) AS rn FROM events) SELECT * FROM ranked WHERE rn = 1",
		"SELECT [user] FROM [main].[table] WHERE [id] = 1",
		"ATTACH DATABASE 'archive.db' AS archive; DETACH DATABASE archive",
	}

	// Common edge cases across all DBMS
	commonEdgeCases := []string{
		// Nested subqueries
		`SELECT * FROM (SELECT * FROM (SELECT * FROM users) t1) t2`,

		// Multiple CTEs
		`WITH
	     cte1 AS (SELECT 1),
	     cte2 AS (SELECT * FROM cte1),
	     cte3 AS (SELECT * FROM cte2)
	     SELECT * FROM cte3`,

		// Mixed quoted identifiers
		`SELECT "col1", [col2], 'string "with" quotes' FROM users`,

		// Comments in various positions
		`SELECT /*comment1*/ * --comment2
	     /*comment3*/ FROM /*comment4*/ users`,

		// Unicode identifiers
		`SELECT * FROM "表" WHERE "列" = '値'`,

		// Empty strings and special characters
		`SELECT '', '\'\'\'\'\'''', '\\\\', '\n\r\t'`,
	}

	// Add all patterns for testing
	patterns := []string{}
	patterns = append(patterns, postgresPatterns...)
	patterns = append(patterns, sqlServerPatterns...)
	patterns = append(patterns, mysqlPatterns...)
	patterns = append(patterns, oraclePatterns...)
	patterns = append(patterns, snowflakePatterns...)
	patterns = append(patterns, commonEdgeCases...)
	patterns = append(patterns, sqlitePatterns...)

	// Add each pattern with different DBMS types
	dbmsTypes := []string{
		string(DBMSPostgres),
		string(DBMSSQLServer),
		string(DBMSMySQL),
		string(DBMSOracle),
		string(DBMSSnowflake),
		string(DBMSSQLite),
	}

	for _, pattern := range patterns {
		for _, dbms := range dbmsTypes {
			f.Add(pattern, dbms)
		}
	}
}

func addObfuscationTestCases(f *testing.F) {
	// PostgreSQL specific obfuscation patterns
	postgresPatterns := []string{
		// Dollar quoted strings
		`SELECT $tag$string with 'quotes" and $dollars$tag$`,
		// Array literals
		`SELECT ARRAY[1, 2, 3], '{1,2,3}'::int[]`,
		// Postgres specific time/date
		`SELECT TIMESTAMP WITH TIME ZONE '2023-01-01 12:00:00+00'`,
		// Custom types
		`SELECT '127.0.0.1'::inet, '12:34:56:78:90:ab'::macaddr`,
	}

	// SQL Server specific obfuscation patterns
	sqlServerPatterns := []string{
		// Money and smallmoney
		`SELECT $123.45, £123.45`,
		// Unicode strings
		`SELECT N'unicode string'`,
		// Binary strings
		`SELECT 0x1234ABCD`,
		// DateTime2
		`SELECT CONVERT(datetime2, '2023-01-01 12:00:00.1234567')`,
	}

	// MySQL specific obfuscation patterns
	mysqlPatterns := []string{
		// Hex literals
		`SELECT X'1234', 0x1234`,
		// Bit values
		`SELECT b'1010', 0b1010`,
		// MySQL date formats
		`SELECT DATE '2023-01-01' + INTERVAL 1 DAY`,
		// Backtick strings
		"SELECT `col1`, `table`.`col2`",
	}

	// Oracle specific obfuscation patterns
	oraclePatterns := []string{
		// Oracle date format
		`SELECT DATE '2023-01-01', TIMESTAMP '2023-01-01 12:00:00'`,
		// INTERVAL literals
		`SELECT INTERVAL '1' DAY, INTERVAL '2' YEAR`,
		// Q quoted strings
		`SELECT Q'[string's with 'quotes']'`,
		// ROWID
		`SELECT CHARTOROWID('AAAB12AADAAAAwPAAA')`,
	}

	// Snowflake specific obfuscation patterns
	snowflakePatterns := []string{
		// Semi-structured data
		`SELECT PARSE_JSON('{"a": 1}'):a::number`,
		// Variant
		`SELECT TO_VARIANT(1234)`,
		// Geographic data
		`SELECT TO_GEOGRAPHY('POINT(-122.35 37.55)')`,
		// Stage references
		`SELECT $1, $2, $3 FROM @mystage`,
	}

	// SQLite specific obfuscation patterns
	sqlitePatterns := []string{
		`SELECT * FROM logs WHERE id = ?5 AND tag = @tag`,
		`SELECT * FROM users WHERE email = :email OR email = $email`,
		`SELECT $ns::var, $env(config), $ns::name(sub)`,
		`SELECT [user] FROM [main].[table] WHERE [id] = 1`,
		`PRAGMA table_info('users')`,
	}

	// Common obfuscation patterns for all DBMS
	commonPatterns := []string{
		// Basic numbers
		`SELECT 123, -456, 3.14159, -0.123, 1e10, -1e-10`,
		`SELECT * FROM t1 WHERE id IN (1, 2, 3, 4, 5)`,

		// Basic strings
		`SELECT 'string', 'str''ing'`,
		`SELECT * FROM t1 WHERE name IN ('a', 'b', 'c')`,

		// Mixed literals
		`SELECT * FROM t1 WHERE id = 123 AND name = 'abc'`,

		// Complex expressions
		`SELECT CASE WHEN id > 100 THEN 'high' ELSE 'low' END`,

		// Special characters
		`SELECT '\n\r\t\b\f'`,

		// Unicode
		`SELECT '🙂', '漢字', 'ñ', 'é'`,

		// Numbers in identifiers
		`SELECT col1 AS alias123`,

		// Complex number formats
		`SELECT .123, 123., -123.456e-789`,
	}

	// Quote edge cases for all DBMS
	quoteEdgeCases := []string{
		// Unmatched quotes (these should be handled gracefully)
		`SELECT '`, `SELECT "`, `SELECT [`,

		// Multiple types of quotes
		`SELECT '"'`, `SELECT "''"`,
		`SELECT '''`, `SELECT """`,

		// Escaped quotes
		`SELECT '\''`, `SELECT "\""`,

		// Quotes in comments
		`SELECT /* ' */ col1`,
		`SELECT -- "`,
		`SELECT /* [ */ col1 /* ] */`,
	}

	// Add Postgres patterns with Postgres DBMS
	for _, pattern := range postgresPatterns {
		f.Add(pattern, string(DBMSPostgres))
	}

	// Add SQL Server patterns with SQL Server DBMS
	for _, pattern := range sqlServerPatterns {
		f.Add(pattern, string(DBMSSQLServer))
	}

	// Add MySQL patterns with MySQL DBMS
	for _, pattern := range mysqlPatterns {
		f.Add(pattern, string(DBMSMySQL))
	}

	// Add Oracle patterns with Oracle DBMS
	for _, pattern := range oraclePatterns {
		f.Add(pattern, string(DBMSOracle))
	}

	// Add Snowflake patterns with Snowflake DBMS
	for _, pattern := range snowflakePatterns {
		f.Add(pattern, string(DBMSSnowflake))
	}

	// Add SQLite patterns with SQLite DBMS
	for _, pattern := range sqlitePatterns {
		f.Add(pattern, string(DBMSSQLite))
	}

	// Add common patterns and quote edge cases with all DBMS types
	dbmsTypes := []string{
		string(DBMSPostgres),
		string(DBMSSQLServer),
		string(DBMSMySQL),
		string(DBMSOracle),
		string(DBMSSnowflake),
		string(DBMSSQLite),
	}

	for _, pattern := range append(commonPatterns, quoteEdgeCases...) {
		for _, dbms := range dbmsTypes {
			f.Add(pattern, dbms)
		}
	}
}
