package sqllexer

import (
	"testing"
)

func TestObfuscator(t *testing.T) {
	tests := []struct {
		input         string
		want          string
		replaceDigits bool
	}{
		{
			input:         "SELECT * FROM users where id = 1",
			want:          "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = '12'",
			want:          "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM \"users table\" where id = 1",
			want:          "SELECT * FROM \"users table\" where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users1 where id = ?",
			want:          "SELECT * FROM users1 where id = ?",
			replaceDigits: false,
		},
		{
			input:         "SELECT * FROM users1 where id = ?",
			want:          "SELECT * FROM users? where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = 1 -- this is a comment",
			want:          "SELECT * FROM users where id = ? /* this is a comment */",
			replaceDigits: true,
		},
		{
			input: `
			/* this is a comment 
			with multiple lines
			*/
			SELECT * FROM users where id = 1
			`,
			want:          "/* this is a comment with multiple lines */ SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input: `
			SELECT * FROM users where id = 1
			/* this is a comment with multiple lines
			`,
			want: "SELECT * FROM users where id = ? /* this is a comment with multiple lines",
		},
		{
			input: "SELECT * FROM users where id = 'Joh",
			want:  "SELECT * FROM users where id = ?",
		},
		{
			input: "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users where id = 1",
			want:  "SELECT $func$INSERT INTO table VALUES (?, ?, ?)$func$ FROM users where id = ?",
		},
		{
			input: "SELECT * FROM users where id = $tag$test$tag$",
			want:  "SELECT * FROM users where id = ?",
		},
		{
			input: "SELECT * FROM users where id = $$test$$",
			want:  "SELECT * FROM users where id = ?",
		},
		{
			input: "SELECT 1.2, 1.2e3, 1.2e-3, 1.2E3, 1.2E-3 FROM users where id = 1",
			want:  "SELECT ?, ?, ?, ?, ? FROM users where id = ?",
		},
		{
			input: `SELECT * FROM "世界" where name = '🌊'`,
			want:  `SELECT * FROM "世界" where name = ?`,
		},
		{
			input: "SELECT * FROM users where id in (SELECT id FROM users where id in (1, 2, 3))",
			want:  "SELECT * FROM users where id in (SELECT id FROM users where id in (?, ?, ?))",
		},
		{
			input: "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d1 SET (c1, c2, c3) = (c1 + 1, c2 + 1, c3 + 1))",
			want:  "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d1 SET (c1, c2, c3) = (c1 + ?, c2 + ?, c3 + ?))",
		},
		{
			input: `
			-- Testing explicit table SQL expression
			WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE  CITY = 'London'),
			T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, 2 * WEIGHT AS NEW_WEIGHT, 'Oslo' AS NEW_CITY FROM T1),
			T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2),
			T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1)
			TABLE T4 UNION CORRESPONDING TABLE T3`,
			want:          "/* Testing explicit table SQL expression */ WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE CITY = ?), T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT AS NEW_WEIGHT, ? AS NEW_CITY FROM T1), T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2), T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1) TABLE T4 UNION CORRESPONDING TABLE T3",
			replaceDigits: false,
		},
		{
			input: `
			-- Testing explicit table SQL expression
			WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE  CITY = 'London'),
			T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, 2 * WEIGHT AS NEW_WEIGHT, 'Oslo' AS NEW_CITY FROM T1),
			T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2),
			T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1)
			TABLE T4 UNION CORRESPONDING TABLE T3`,
			want:          "/* Testing explicit table SQL expression */ WITH T? AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE CITY = ?), T? AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT AS NEW_WEIGHT, ? AS NEW_CITY FROM T?), T? AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T?), T? AS ( TABLE P EXCEPT CORRESPONDING TABLE T?) TABLE T? UNION CORRESPONDING TABLE T?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users123 where id = 1",
			want:          "SELECT * FROM users? where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id in (1, '2', NULL, true, false)",
			want:          "SELECT * FROM users where id in (?, ?, ?, ?, ?)",
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname !~ '.*toIgnore.*'`,
			want:          `SELECT nspname FROM pg_class where nspname !~ ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname !~* '.*toIgnoreInsensitive.*'`,
			want:          `SELECT nspname FROM pg_class where nspname !~* ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname ~ '.*matching.*'`,
			want:          `SELECT nspname FROM pg_class where nspname ~ ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname ~* '.*matchingInsensitive.*'`,
			want:          `SELECT nspname FROM pg_class where nspname ~* ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT * FROM dbo.Items WHERE id = 1 or /*!obfuscation*/ 1 = 1`,
			want:          `SELECT * FROM dbo.Items WHERE id = ? or /*!obfuscation*/ ? = ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT * FROM Items WHERE id = -1 OR id = -01 OR id = -108 OR id = -.018 OR id = -.08 OR id = -908129`,
			want:          `SELECT * FROM Items WHERE id = ? OR id = ? OR id = ? OR id = ? OR id = ? OR id = ?`,
			replaceDigits: true,
		},
		{
			input:         "USING $1 SELECT",
			want:          `USING $1 SELECT`,
			replaceDigits: true,
		},
		{
			input:         "USING - SELECT",
			want:          `USING - SELECT`,
			replaceDigits: true,
		},
		{
			input: "SELECT * FROM \"世界\" where id = '🌊'",
			want:  "SELECT * FROM \"世界\" where id = ?",
		},
		{
			input: "SELECT '🥒'",
			want:  "SELECT ?",
		},
		{
			// postgres json array
			input: `SELECT * FROM users where id = '{"a": 1, "b": 2}'`,
			want:  `SELECT * FROM users where id = ?`,
		},
		{
			// postgres json
			input: `SELECT * FROM users where id = '{"a": 1, "b": 2}'::jsonb`,
			want:  `SELECT * FROM users where id = ?::jsonb`,
		},
		{
			// postgres json <@ operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb <@ '{"a": 1, "b": 2}'::jsonb`,
			want:  `SELECT * FROM users where ?::jsonb <@ ?::jsonb`,
		},
		{
			// postgres json @> operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb @> '{"a": 1, "b": 2}'::jsonb`,
			want:  `SELECT * FROM users where ?::jsonb @> ?::jsonb`,
		},
		{
			// postgres -> operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb -> 'a'`,
			want:  `SELECT * FROM users where ?::jsonb -> ?`,
		},
		{
			// postgres ->> operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ->> 'a'`,
			want:  `SELECT * FROM users where ?::jsonb ->> ?`,
		},
		{
			// postgres #> operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #> '{a}'`,
			want:  `SELECT * FROM users where ?::jsonb #> ?`,
		},
		{
			// postgres #>> operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #>> '{a}'`,
			want:  `SELECT * FROM users where ?::jsonb #>> ?`,
		},
		{
			// postgres ? operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ? 'a'`,
			want:  `SELECT * FROM users where ?::jsonb ? ?`,
		},
		{
			// postgres ?| operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?| '{a}'`,
			want:  `SELECT * FROM users where ?::jsonb ?| ?`,
		},
		{
			// postgres ?& operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?& '{a}'`,
			want:  `SELECT * FROM users where ?::jsonb ?& ?`,
		},
		{
			// postgres json delete operator
			input: `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb - 'a'`,
			want:  `SELECT * FROM users where ?::jsonb - ?`,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			obfuscatorConfig := &SQLObfuscatorConfig{
				ReplaceDigits: tt.replaceDigits,
			}
			obfuscator := NewSQLObfuscator(obfuscatorConfig)
			got := obfuscator.Obfuscate(tt.input)
			if got != tt.want {
				t.Errorf("Obfuscate() = %v, want %v", got, tt.want)
			}
		})
	}
}
