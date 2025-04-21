package sqllexer

import (
	"fmt"
	"strconv"
	"testing"
)

// Benchmark the Tokenizer using a SQL statement
func BenchmarkObfuscator(b *testing.B) {
	// LargeQuery is sourced from https://stackoverflow.com/questions/12607667/issues-with-a-very-large-sql-query/12711494
	var LargeQuery = `SELECT '%c%' as Chapter,
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status IN ('new','assigned') ) AS 'New',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='document_interface' ) AS 'Document\
 Interface',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='interface_development' ) AS 'Inter\
face Development',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='interface_check' ) AS 'Interface C\
heck',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='document_routine' ) AS 'Document R\
outine',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='full_development' ) AS 'Full Devel\
opment',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='peer_review_1' ) AS 'Peer Review O\
ne',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%'AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='peer_review_2' ) AS 'Peer Review Tw\
o',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='qa' ) AS 'QA',
(SELECT count(ticket.id) AS Matches FROM engine.ticket INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%'AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine' AND ticket.status='closed' ) AS 'Closed',
count(id) AS Total,
ticket.id AS _id
FROM engine.ticket
INNER JOIN engine.ticket_custom ON ticket.id = ticket_custom.ticket
WHERE ticket_custom.name='chapter' AND ticket_custom.value LIKE '%c%' AND type='New material' AND milestone='1.1.12' AND component NOT LIKE 'internal_engine'`

	// query3 is sourced from https://www.ibm.com/support/knowledgecenter/SSCRJT_6.0.0/com.ibm.swg.im.bigsql.doc/doc/tut_bsql_uc_complex_query.html
	var ComplexQuery = `WITH
 sales AS
 (SELECT sf.*
  FROM gosalesdw.sls_order_method_dim AS md,
       gosalesdw.sls_product_dim AS pd,
       gosalesdw.emp_employee_dim AS ed,
       gosalesdw.sls_sales_fact AS sf
  WHERE pd.product_key = sf.product_key
    AND pd.product_number > 10000
    AND pd.base_product_key > 30
    AND md.order_method_key = sf.order_method_key
    AND md.order_method_code > 5
    AND ed.employee_key = sf.employee_key
    AND ed.manager_code1 > 20),
 inventory AS
 (SELECT if.*
  FROM gosalesdw.go_branch_dim AS bd,
    gosalesdw.dist_inventory_fact AS if
  WHERE if.branch_key = bd.branch_key
    AND bd.branch_code > 20)
SELECT sales.product_key AS PROD_KEY,
 SUM(CAST (inventory.quantity_shipped AS BIGINT)) AS INV_SHIPPED,
 SUM(CAST (sales.quantity AS BIGINT)) AS PROD_QUANTITY,
 RANK() OVER ( ORDER BY SUM(CAST (sales.quantity AS BIGINT)) DESC) AS PROD_RANK
FROM sales, inventory
 WHERE sales.product_key = inventory.product_key
GROUP BY sales.product_key;
`

	var superLargeQuery = "select top ? percent IdTrebEmpresa, CodCli, NOMEMP, Baixa, CASE WHEN IdCentreTreball IS ? THEN ? ELSE CONVERT ( VARCHAR ( ? ) IdCentreTreball ) END, CASE WHEN NOMESTAB IS ? THEN ? ELSE NOMESTAB END, TIPUS, CASE WHEN IdLloc IS ? THEN ? ELSE CONVERT ( VARCHAR ( ? ) IdLloc ) END, CASE WHEN NomLlocComplert IS ? THEN ? ELSE NomLlocComplert END, CASE WHEN DesLloc IS ? THEN ? ELSE DesLloc END, IdLlocTreballUnic From ( SELECT ?, dbo.Treb_Empresa.IdTrebEmpresa, dbo.Treb_Empresa.IdTreballador, dbo.Treb_Empresa.CodCli, dbo.Clients.NOMEMP, dbo.Treb_Empresa.Baixa, dbo.Treb_Empresa.IdCentreTreball, dbo.Cli_Establiments.NOMESTAB, ?, ?, dbo.Treb_Empresa.DataInici, dbo.Treb_Empresa.DataFi, CASE WHEN dbo.Treb_Empresa.DesLloc IS ? THEN ? ELSE dbo.Treb_Empresa.DesLloc END DesLloc, dbo.Treb_Empresa.IdLlocTreballUnic FROM dbo.Clients WITH ( NOLOCK ) INNER JOIN dbo.Treb_Empresa WITH ( NOLOCK ) ON dbo.Clients.CODCLI = dbo.Treb_Empresa.CodCli LEFT OUTER JOIN dbo.Cli_Establiments WITH ( NOLOCK ) ON dbo.Cli_Establiments.Id_ESTAB_CLI = dbo.Treb_Empresa.IdCentreTreball AND dbo.Cli_Establiments.CODCLI = dbo.Treb_Empresa.CodCli WHERE dbo.Treb_Empresa.IdTreballador = ? AND Treb_Empresa.IdTecEIRLLlocTreball IS ? AND IdMedEIRLLlocTreball IS ? AND IdLlocTreballTemporal IS ? UNION ALL SELECT ?, dbo.Treb_Empresa.IdTrebEmpresa, dbo.Treb_Empresa.IdTreballador, dbo.Treb_Empresa.CodCli, dbo.Clients.NOMEMP, dbo.Treb_Empresa.Baixa, dbo.Treb_Empresa.IdCentreTreball, dbo.Cli_Establiments.NOMESTAB, dbo.Treb_Empresa.IdTecEIRLLlocTreball, dbo.fn_NomLlocComposat ( dbo.Treb_Empresa.IdTecEIRLLlocTreball ), dbo.Treb_Empresa.DataInici, dbo.Treb_Empresa.DataFi, CASE WHEN dbo.Treb_Empresa.DesLloc IS ? THEN ? ELSE dbo.Treb_Empresa.DesLloc END DesLloc, dbo.Treb_Empresa.IdLlocTreballUnic FROM dbo.Clients WITH ( NOLOCK ) INNER JOIN dbo.Treb_Empresa WITH ( NOLOCK ) ON dbo.Clients.CODCLI = dbo.Treb_Empresa.CodCli LEFT OUTER JOIN dbo.Cli_Establiments WITH ( NOLOCK ) ON dbo.Cli_Establiments.Id_ESTAB_CLI = dbo.Treb_Empresa.IdCentreTreball AND dbo.Cli_Establiments.CODCLI = dbo.Treb_Empresa.CodCli WHERE ( dbo.Treb_Empresa.IdTreballador = ? ) AND ( NOT ( dbo.Treb_Empresa.IdTecEIRLLlocTreball IS ? ) ) UNION ALL SELECT ?, dbo.Treb_Empresa.IdTrebEmpresa, dbo.Treb_Empresa.IdTreballador, dbo.Treb_Empresa.CodCli, dbo.Clients.NOMEMP, dbo.Treb_Empresa.Baixa, dbo.Treb_Empresa.IdCentreTreball, dbo.Cli_Establiments.NOMESTAB, dbo.Treb_Empresa.IdMedEIRLLlocTreball, dbo.fn_NomMedEIRLLlocComposat ( dbo.Treb_Empresa.IdMedEIRLLlocTreball ), dbo.Treb_Empresa.DataInici, dbo.Treb_Empresa.DataFi, CASE WHEN dbo.Treb_Empresa.DesLloc IS ? THEN ? ELSE dbo.Treb_Empresa.DesLloc END DesLloc, dbo.Treb_Empresa.IdLlocTreballUnic FROM dbo.Clients WITH ( NOLOCK ) INNER JOIN dbo.Treb_Empresa WITH ( NOLOCK ) ON dbo.Clients.CODCLI = dbo.Treb_Empresa.CodCli LEFT OUTER JOIN dbo.Cli_Establiments WITH ( NOLOCK ) ON dbo.Cli_Establiments.Id_ESTAB_CLI = dbo.Treb_Empresa.IdCentreTreball AND dbo.Cli_Establiments.CODCLI = dbo.Treb_Empresa.CodCli WHERE ( dbo.Treb_Empresa.IdTreballador = ? ) AND ( Treb_Empresa.IdTecEIRLLlocTreball IS ? ) AND ( NOT ( dbo.Treb_Empresa.IdMedEIRLLlocTreball IS ? ) ) UNION ALL SELECT ?, dbo.Treb_Empresa.IdTrebEmpresa, dbo.Treb_Empresa.IdTreballador, dbo.Treb_Empresa.CodCli, dbo.Clients.NOMEMP, dbo.Treb_Empresa.Baixa, dbo.Treb_Empresa.IdCentreTreball, dbo.Cli_Establiments.NOMESTAB, dbo.Treb_Empresa.IdLlocTreballTemporal, dbo.Lloc_Treball_Temporal.NomLlocTreball, dbo.Treb_Empresa.DataInici, dbo.Treb_Empresa.DataFi, CASE WHEN dbo.Treb_Empresa.DesLloc IS ? THEN ? ELSE dbo.Treb_Empresa.DesLloc END DesLloc, dbo.Treb_Empresa.IdLlocTreballUnic FROM dbo.Clients WITH ( NOLOCK ) INNER JOIN dbo.Treb_Empresa WITH ( NOLOCK ) ON dbo.Clients.CODCLI = dbo.Treb_Empresa.CodCli INNER JOIN dbo.Lloc_Treball_Temporal WITH ( NOLOCK ) ON dbo.Treb_Empresa.IdLlocTreballTemporal = dbo.Lloc_Treball_Temporal.IdLlocTreballTemporal LEFT OUTER JOIN dbo.Cli_Establiments WITH ( NOLOCK ) ON dbo.Cli_Establiments.Id_ESTAB_CLI = dbo.Treb_Empresa.IdCentreTreball AND dbo.Cli_Establiments.CODCLI = dbo.Treb_Empresa.CodCli WHERE dbo.Treb_Empresa.IdTreballador = ? AND Treb_Empresa.IdTecEIRLLlocTreball IS ? AND IdMedEIRLLlocTreball IS ? ) Where ? = %d"

	benchmarks := []struct {
		name  string
		query string
	}{
		{"Escaping", `INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`},
		{"Grouping", `INSERT INTO delayed_jobs (created_at, failed_at, handler) VALUES (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL)`},
		{"Large", LargeQuery},
		{"Complex", ComplexQuery},
		{"SuperLarge", fmt.Sprintf(superLargeQuery, 1)},
	}
	obfuscator := NewObfuscator(
		WithReplaceDigits(false),
	)

	for _, bm := range benchmarks {
		b.Run(bm.name+"/"+strconv.Itoa(len(bm.query)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				obfuscator.Obfuscate(bm.query)
			}
		})
	}
}

func BenchmarkObfuscatorMore(b *testing.B) {
	tests := []struct {
		input                      string
		expected                   string
		replaceDigits              bool
		replacePositionalParameter bool
		replaceBoolean             bool
		replaceNull                bool
		dollarQuotedFunc           bool
		keepJsonPath               bool
		replaceBindParameter       bool
		dbms                       DBMSType
	}{
		{
			input:         "SELECT * FROM users where id = 1",
			expected:      "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = 0x124af",
			expected:      "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = 0617",
			expected:      "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = '12'",
			expected:      "SELECT * FROM users where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id = 'j\\'s'",
			expected:      "SELECT * FROM users where id = ?",
			replaceDigits: false,
		},
		{
			input:         "SELECT * FROM \"users table\" where id = 1",
			expected:      "SELECT * FROM \"users table\" where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users1 where id = ?",
			expected:      "SELECT * FROM users1 where id = ?",
			replaceDigits: false,
		},
		{
			input:         "SELECT * FROM users1 where id = ?",
			expected:      "SELECT * FROM users? where id = ?",
			replaceDigits: true,
		},
		{
			input:          "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			expected:       "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			replaceBoolean: false,
			replaceNull:    false,
		},
		{
			input:          "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			expected:       "SELECT * FROM users where id is NULL and is_active = ? and is_admin = ?",
			replaceBoolean: true,
			replaceNull:    false,
		},
		{
			input:          "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			expected:       "SELECT * FROM users where id is ? and is_active = TRUE and is_admin = FALSE",
			replaceBoolean: false,
			replaceNull:    true,
		},
		{
			input:          "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			expected:       "SELECT * FROM users where id is ? and is_active = ? and is_admin = ?",
			replaceBoolean: true,
			replaceNull:    true,
		},
		{
			input:         "SELECT * FROM users where id = 1 -- this is a comment",
			expected:      "SELECT * FROM users where id = ? -- this is a comment",
			replaceDigits: true,
		},
		{
			input: `/* this is a comment
			with multiple lines
			*/
			SELECT * FROM users where id = 1`,
			expected: `/* this is a comment
			with multiple lines
			*/
			SELECT * FROM users where id = ?`,
			replaceDigits: true,
		},
		{
			input: `
			SELECT * FROM users where id = 1
			/* this is a comment
			with multiple lines */
			`,
			expected: `SELECT * FROM users where id = ?
			/* this is a comment
			with multiple lines */`,
		},
		{
			input:    "SELECT * FROM users where id = 'Joh",
			expected: "SELECT * FROM users where id = ?",
		},
		{
			input:            "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users where id = 1",
			expected:         "SELECT ? FROM users where id = ?",
			dollarQuotedFunc: false,
		},
		{
			input:            "SELECT $func$INSERT INTO table VALUES ('a', 1, 2)$func$ FROM users where id = 1",
			expected:         "SELECT $func$INSERT INTO table VALUES (?, ?, ?)$func$ FROM users where id = ?",
			dollarQuotedFunc: true,
		},
		{
			input:    "SELECT * FROM users where id = $tag$test$tag$",
			expected: "SELECT * FROM users where id = ?",
		},
		{
			input:    "SELECT * FROM users where id = $$test$$",
			expected: "SELECT * FROM users where id = ?",
		},
		{
			input:    "SELECT 1.2, 1.2e3, 1.2e-3, 1.2E3, 1.2E-3 FROM users where id = 1",
			expected: "SELECT ?, ?, ?, ?, ? FROM users where id = ?",
		},
		{
			input:    `SELECT * FROM "世界" where name = '🌊'`,
			expected: `SELECT * FROM "世界" where name = ?`,
		},
		{
			input:    "SELECT * FROM users where id in (SELECT id FROM users where id in (1, 2, 3))",
			expected: "SELECT * FROM users where id in (SELECT id FROM users where id in (?, ?, ?))",
		},
		{
			input:    "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d192729812918718919 SET (c1, c2, c3) = (c1 + 1, c2 + 1, c3 + 1))",
			expected: "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d192729812918718919 SET (c1, c2, c3) = (c1 + ?, c2 + ?, c3 + ?))",
		},
		{
			input:    "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d1 SET (c1, c2, c3) = (c1 + 1, c2 + 1, c3 + 1))",
			expected: "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d1 SET (c1, c2, c3) = (c1 + ?, c2 + ?, c3 + ?))",
		},
		{
			input:    "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d SET (c, c, c) = (c + 1, c + 1, c + 1))",
			expected: "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE (UPDATE d SET (c, c, c) = (c + ?, c + ?, c + ?))",
		},
		{
			input:    "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE UPDATE d1 SET c1 = c1 + 1",
			expected: "CREATE TRIGGER dogwatcher SELECT ON w1 BEFORE UPDATE d1 SET c1 = c1 + ?",
		},
		{
			input:    "UPDATE d1 SET c1 = c1 + 1",
			expected: "UPDATE d1 SET c1 = c1 + ?",
		},
		{
			input:    "UPDATE d SET c = c + 1",
			expected: "UPDATE d SET c = c + ?",
		},
		{
			input: `
			-- Testing explicit table SQL expression
			WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE  CITY = 'London'),
			T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, 2 * WEIGHT AS NEW_WEIGHT, 'Oslo' AS NEW_CITY FROM T1),
			T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2),
			T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1)
			TABLE T4 UNION CORRESPONDING TABLE T3`,
			expected: `-- Testing explicit table SQL expression
			WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE  CITY = ?),
			T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT AS NEW_WEIGHT, ? AS NEW_CITY FROM T1),
			T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2),
			T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1)
			TABLE T4 UNION CORRESPONDING TABLE T3`,
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
			expected: `-- Testing explicit table SQL expression
			WITH T? AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE  CITY = ?),
			T? AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT AS NEW_WEIGHT, ? AS NEW_CITY FROM T?),
			T? AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T?),
			T? AS ( TABLE P EXCEPT CORRESPONDING TABLE T?)
			TABLE T? UNION CORRESPONDING TABLE T?`,
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users123 where id = 1",
			expected:      "SELECT * FROM users? where id = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users123ab where id12 = 1",
			expected:      "SELECT * FROM users?ab where id? = ?",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id in (1, '2', 3, 1.5, '12')",
			expected:      "SELECT * FROM users where id in (?, ?, ?, ?, ?)",
			replaceDigits: true,
		},
		{
			input:         "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			expected:      "SELECT * FROM users where id is NULL and is_active = TRUE and is_admin = FALSE",
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname !~ '.*toIgnore.*'`,
			expected:      `SELECT nspname FROM pg_class where nspname !~ ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname !~* '.*toIgnoreInsensitive.*'`,
			expected:      `SELECT nspname FROM pg_class where nspname !~* ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname ~ '.*matching.*'`,
			expected:      `SELECT nspname FROM pg_class where nspname ~ ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT nspname FROM pg_class where nspname ~* '.*matchingInsensitive.*'`,
			expected:      `SELECT nspname FROM pg_class where nspname ~* ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT * FROM dbo.Items WHERE id = 1 or /*!obfuscation*/ 1 = 1`,
			expected:      `SELECT * FROM dbo.Items WHERE id = ? or /*!obfuscation*/ ? = ?`,
			replaceDigits: true,
		},
		{
			input:         `SELECT * FROM Items WHERE id = -1 OR id = +01 OR id = -108 OR id = -.018 OR id = -.08 OR id = -908129 OR id = 1e2 OR id = 1e-1`,
			expected:      `SELECT * FROM Items WHERE id = ? OR id = ? OR id = ? OR id = ? OR id = ? OR id = ? OR id = ? OR id = ?`,
			replaceDigits: true,
		},
		{
			input:         "USING $1 SELECT",
			expected:      `USING $1 SELECT`,
			replaceDigits: true,
		},
		{
			input:         "USING - SELECT",
			expected:      `USING - SELECT`,
			replaceDigits: true,
		},
		{
			input:                      "SELECT * FROM users where id = $1",
			expected:                   `SELECT * FROM users where id = $1`,
			replacePositionalParameter: false,
		},
		{
			input:                      "SELECT * FROM users where id = $1",
			expected:                   `SELECT * FROM users where id = ?`,
			replacePositionalParameter: true,
		},
		{
			input:    `SELECT * FROM "public"."users" where id = 2`,
			expected: `SELECT * FROM "public"."users" where id = ?`,
		},
		{
			input:    "SELECT * FROM \"世界\" where id = '🌊'",
			expected: "SELECT * FROM \"世界\" where id = ?",
		},
		{
			input:    "SELECT '🥒'",
			expected: "SELECT ?",
		},
		{
			// postgres json array
			input:    `SELECT * FROM users where id = '{"a": 1, "b": 2}'`,
			expected: `SELECT * FROM users where id = ?`,
		},
		{
			// postgres json
			input:    `SELECT * FROM users where id = '{"a": 1, "b": 2}'::jsonb`,
			expected: `SELECT * FROM users where id = ?::jsonb`,
		},
		{
			// postgres json <@ operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb <@ '{"a": 1, "b": 2}'::jsonb`,
			expected: `SELECT * FROM users where ?::jsonb <@ ?::jsonb`,
		},
		{
			// postgres json @> operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb @> '{"a": 1, "b": 2}'::jsonb`,
			expected: `SELECT * FROM users where ?::jsonb @> ?::jsonb`,
		},
		{
			// postgres -> operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb -> 'a'`,
			expected: `SELECT * FROM users where ?::jsonb -> ?`,
		},
		{
			// postgres ->> operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ->> 'a'`,
			expected: `SELECT * FROM users where ?::jsonb ->> ?`,
		},
		{
			// postgres #> operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #> '{a}'`,
			expected: `SELECT * FROM users where ?::jsonb #> ?`,
		},
		{
			// postgres #>> operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #>> '{a}'`,
			expected: `SELECT * FROM users where ?::jsonb #>> ?`,
		},
		{
			// postgres ? operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ? 'a'`,
			expected: `SELECT * FROM users where ?::jsonb ? ?`,
		},
		{
			// postgres ?| operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?| '{a}'`,
			expected: `SELECT * FROM users where ?::jsonb ?| ?`,
		},
		{
			// postgres ?& operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?& '{a}'`,
			expected: `SELECT * FROM users where ?::jsonb ?& ?`,
		},
		{
			// postgres json delete operator
			input:    `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb - 'a'`,
			expected: `SELECT * FROM users where ?::jsonb - ?`,
		},
		{
			input: `
			-- Testing explicit table SQL expression
			DECLARE @TableName NVARCHAR(50) = 'MyTableName'
			DECLARE @Query NVARCHAR(1000)
			/* Build the SQL string */

			SET @Query = 'SELECT * FROM ' + @TableName
			EXEC sp_executesql @Query
			`,
			expected: `-- Testing explicit table SQL expression
			DECLARE @TableName NVARCHAR(?) = ?
			DECLARE @Query NVARCHAR(?)
			/* Build the SQL string */

			SET @Query = ? + @TableName
			EXEC sp_executesql @Query`,
		},
		{
			input: `
			MERGE INTO Employees AS target
			USING EmployeeUpdates AS source
			ON (target.EmployeeID = source.EmployeeID)
			WHEN MATCHED THEN
				UPDATE SET
					target.Name = source.Name,
					target.Age = source.Age,
					target.Salary = source.Salary
			WHEN NOT MATCHED BY TARGET THEN
				INSERT (EmployeeID, Name, Age, Salary)
				VALUES (source.EmployeeID, source.Name, source.Age, source.Salary)
			WHEN NOT MATCHED BY SOURCE THEN
				DELETE
			OUTPUT $action, inserted.*, deleted.*;
			`,
			expected: `MERGE INTO Employees AS target
			USING EmployeeUpdates AS source
			ON (target.EmployeeID = source.EmployeeID)
			WHEN MATCHED THEN
				UPDATE SET
					target.Name = source.Name,
					target.Age = source.Age,
					target.Salary = source.Salary
			WHEN NOT MATCHED BY TARGET THEN
				INSERT (EmployeeID, Name, Age, Salary)
				VALUES (source.EmployeeID, source.Name, source.Age, source.Salary)
			WHEN NOT MATCHED BY SOURCE THEN
				DELETE
			OUTPUT $action, inserted.*, deleted.*;`,
			dbms: DBMSSQLServer,
		},
		{
			input:    "SELECT TRUNC(SYSDATE@!) from dual",
			expected: "SELECT TRUNC(SYSDATE@!) from dual",
			dbms:     DBMSOracle,
		},
		{
			input: `
			select sql_fulltext from v$sql where force_matching_signature = 1033183797897134935
			GROUP BY c.name, force_matching_signature, plan_hash_value
			HAVING MAX(last_active_time) > sysdate - :seconds/24/60/60
			FETCH FIRST :limit ROWS ONLY`,
			expected: `select sql_fulltext from v$sql where force_matching_signature = ?
			GROUP BY c.name, force_matching_signature, plan_hash_value
			HAVING MAX(last_active_time) > sysdate - :seconds/?/?/?
			FETCH FIRST :limit ROWS ONLY`,
			dbms: DBMSOracle,
		},
		{
			input:    "SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > 85",
			expected: `SELECT TABLESPACE_NAME, USED_SPACE, TABLESPACE_SIZE, USED_PERCENT FROM SYS.DBA_TABLESPACE_USAGE_METRICS K WHERE USED_PERCENT > ?`,
			dbms:     DBMSOracle,
		},
		{
			input:    "SELECT dbms_lob.substr(sql_fulltext, 4000, 1) sql_fulltext FROM sys.dd_session",
			expected: `SELECT dbms_lob.substr(sql_fulltext, ?, ?) sql_fulltext FROM sys.dd_session`,
			dbms:     DBMSOracle,
		},
		{
			input:    "begin execute immediate 'alter session set sql_trace=true'; end;",
			expected: "begin execute immediate ?; end;",
			dbms:     DBMSOracle,
		},
		{
			input:    "SELECT * FROM #users where id = @id and name = @1",
			expected: "SELECT * FROM #users where id = @id and name = @1",
			dbms:     DBMSSQLServer,
		},
		{
			input: `SELECT @@OPTIONS AS OriginalOptionsValue;
			SET CONCAT_NULL_YIELDS_NULL OFF;
			SELECT 'abc' + NULL AS ResultWhen_OFF, @@OPTIONS AS OptionsValueWhen_OFF;
			SET CONCAT_NULL_YIELDS_NULL ON;
			SELECT 'abc' + NULL AS ResultWhen_ON, @@OPTIONS AS OptionsValueWhen_ON;`,
			expected: `SELECT @@OPTIONS AS OriginalOptionsValue;
			SET CONCAT_NULL_YIELDS_NULL OFF;
			SELECT ? + NULL AS ResultWhen_OFF, @@OPTIONS AS OptionsValueWhen_OFF;
			SET CONCAT_NULL_YIELDS_NULL ON;
			SELECT ? + NULL AS ResultWhen_ON, @@OPTIONS AS OptionsValueWhen_ON;`,
			dbms: DBMSSQLServer,
		},
		{
			input:    "SELECT * FROM users where id = :id and name = :1",
			expected: "SELECT * FROM users where id = :id and name = :1",
			dbms:     DBMSOracle,
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
			expected: `WITH SILENCES AS (
				SELECT LOWER(BASE_TABLE_NAME), CREATED_DT, SILENCE_UNTIL_DT, REASON
					,ROW_NUMBER() OVER (PARTITION BY LOWER(BASE_TABLE_NAME) ORDER BY CREATED_DT DESC) AS ROW_NUMBER
				FROM REPORTING.GENERAL.SOME_TABLE
				WHERE CONTAINS(?, LOWER(DATACENTER_LABEL))
			  )
			  SELECT * FROM SILENCES WHERE ROW_NUMBER = ?;`,
			dbms: DBMSSnowflake,
		},
		{
			input:    `USE WAREHOUSE "SOME_WAREHOUSE";`,
			expected: `USE WAREHOUSE "SOME_WAREHOUSE";`, // double quoted identifier are not replaced
			dbms:     DBMSSnowflake,
		},
		{
			input: `SELECT 1 FROM REPORTING.GENERAL.SOME_RANDOM_TABLE
			WHERE BASE_TABLE_NAME='xxx_ttt_zzz_v1'
			AND DATACENTER_LABEL='us3'
			AND CENSUS_ELEMENT_ID='bef52c3f-788f-4fb3-b116-a05a1c4a9792';`,
			expected: `SELECT ? FROM REPORTING.GENERAL.SOME_RANDOM_TABLE
			WHERE BASE_TABLE_NAME=?
			AND DATACENTER_LABEL=?
			AND CENSUS_ELEMENT_ID=?;`,
			dbms: DBMSSnowflake,
		},
		{
			input: `COPY INTO  REPORTING.GENERAL.MY_TABLE
			(FEATURE,DESCRIPTION,COVERAGE,DATE_PARTITION)
			FROM (SELECT $1,$2,$3,TO_TIMESTAMP('2023-12-14 00:00:00') FROM @REPORTING.GENERAL.SOME_DESCRIPTIONS/external_data/2023/12/14/)
			file_format=(type=CSV SKIP_HEADER=1 FIELD_OPTIONALLY_ENCLOSED_BY='\"' ESCAPE_UNENCLOSED_FIELD='\\' FIELD_DELIMITER=',' )
			;`,
			expected: `COPY INTO  REPORTING.GENERAL.MY_TABLE
			(FEATURE,DESCRIPTION,COVERAGE,DATE_PARTITION)
			FROM (SELECT $1,$2,$3,TO_TIMESTAMP(?) FROM @REPORTING.GENERAL.SOME_DESCRIPTIONS/external_data/2023/12/14/)
			file_format=(type=CSV SKIP_HEADER=? FIELD_OPTIONALLY_ENCLOSED_BY=? ESCAPE_UNENCLOSED_FIELD=? FIELD_DELIMITER=? )
			;`,
			dbms: DBMSSnowflake,
		},
		{
			input: `SELECT EXISTS(
				SELECT * FROM REPORTING.INFORMATION_SCHEMA.TABLES
				WHERE table_schema='XXX_YYY'
				AND table_name='ABC'
				AND table_type='EXTERNAL TABLE'
			);`,
			expected: `SELECT EXISTS(
				SELECT * FROM REPORTING.INFORMATION_SCHEMA.TABLES
				WHERE table_schema=?
				AND table_name=?
				AND table_type=?
			);`,
			dbms: DBMSSnowflake,
		},
		{
			input:    `ALTER EXTERNAL TABLE REPORTING.TEST.MY_TABLE REFRESH '2024_01_15';`,
			expected: `ALTER EXTERNAL TABLE REPORTING.TEST.MY_TABLE REFRESH ?;`,
			dbms:     DBMSSnowflake,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb <@ '{"a": 1, "b": 2}'::jsonb`,
			expected:     `SELECT * FROM users where ?::jsonb <@ '{"a": 1, "b": 2}'::jsonb`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb @> '{"a": 1, "b": 2}'::jsonb`,
			expected:     `SELECT * FROM users where ?::jsonb @> '{"a": 1, "b": 2}'::jsonb`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb -> 'a'`,
			expected:     `SELECT * FROM users where ?::jsonb -> 'a'`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ->> 'a'`,
			expected:     `SELECT * FROM users where ?::jsonb ->> 'a'`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #> '{a}'`,
			expected:     `SELECT * FROM users where ?::jsonb #> '{a}'`,
			keepJsonPath: true,
		},
		{
			// postgres #>> operator
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb #>> '{a}'`,
			expected:     `SELECT * FROM users where ?::jsonb #>> '{a}'`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?| '{a}'`,
			expected:     `SELECT * FROM users where ?::jsonb ?| '{a}'`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where '{"a": 1, "b": 2}'::jsonb ?& '{a}'`,
			expected:     `SELECT * FROM users where ?::jsonb ?& '{a}'`,
			keepJsonPath: true,
		},
		{
			input:        `SELECT * FROM users where data::jsonb ->> 1`,
			expected:     `SELECT * FROM users where data::jsonb ->> 1`,
			keepJsonPath: true,
		},
		{
			input:                `SELECT * FROM users where id = @_My_id`,
			expected:             `SELECT * FROM users where id = @_My_id`,
			replaceBindParameter: false,
		},
		{
			input:                `SELECT * FROM users where id = @_My_id`,
			expected:             `SELECT * FROM users where id = ?`,
			replaceBindParameter: true,
		},
	}

	for _, bm := range tests {
		for _, hasDigits := range []bool{false, true} {
			b.Run(bm.input[0:5]+"/"+strconv.FormatBool(hasDigits)+"/"+strconv.Itoa(len(bm.input)), func(b *testing.B) {
				obfuscator := NewObfuscator(
					WithReplaceDigits(bm.replaceDigits),
					WithReplacePositionalParameter(bm.replacePositionalParameter),
					WithReplaceBoolean(bm.replaceBoolean),
					WithReplaceNull(bm.replaceNull),
					WithDollarQuotedFunc(bm.dollarQuotedFunc),
					WithKeepJsonPath(bm.keepJsonPath),
					WithReplaceBindParameter(bm.replaceBindParameter),
				)

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					obfuscator.Obfuscate(bm.input, WithUseHasDigits(hasDigits))
				}
			})
		}
	}

}
