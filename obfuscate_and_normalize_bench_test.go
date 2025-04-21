package sqllexer

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Benchmark the Tokenizer using a SQL statement
func BenchmarkObfuscationAndNormalization(b *testing.B) {
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

	var bracketQuotedQuery = `
	SELECT 
    [orders].[OrderID],
    [customers].[CustomerName],
    [products].[ProductName],
    [order_details].[Quantity],
    [order_details].[UnitPrice],
    ([order_details].[Quantity] * [order_details].[UnitPrice]) AS [TotalPrice],
    [orders].[OrderDate],
    [orders].[ShippedDate],
    CASE 
        WHEN [orders].[ShippedDate] IS NULL THEN 'Pending'
        ELSE 'Shipped'
    END AS [OrderStatus]
FROM 
    [orders]
INNER JOIN 
    [customers] ON [orders].[CustomerID] = [customers].[CustomerID]
INNER JOIN 
    [order_details] ON [orders].[OrderID] = [order_details].[OrderID]
INNER JOIN 
    [products] ON [order_details].[ProductID] = [products].[ProductID]
WHERE 
    [orders].[OrderDate] >= '2024-01-01' 
    AND [orders].[OrderDate] <= '2024-12-31'
    AND [customers].[Region] = 'North America'
GROUP BY 
    [orders].[OrderID],
    [customers].[CustomerName],
    [products].[ProductName],
    [order_details].[Quantity],
    [order_details].[UnitPrice],
    [orders].[OrderDate],
    [orders].[ShippedDate]
HAVING 
    SUM([order_details].[Quantity]) > 10
ORDER BY 
    [orders].[OrderDate] DESC;
`

	var backtickQuotedQuery = "SELECT `orders`.`OrderID`, `customers`.`CustomerName`, `products`.`ProductName`, `order_details`.`Quantity`, `order_details`.`UnitPrice`, (`order_details`.`Quantity` * `order_details`.`UnitPrice`) AS `TotalPrice`, `orders`.`OrderDate`, `orders`.`ShippedDate`, CASE WHEN `orders`.`ShippedDate` IS NULL THEN 'Pending' ELSE 'Shipped' END AS `OrderStatus` FROM `orders` INNER JOIN `customers` ON `orders`.`CustomerID` = `customers`.`CustomerID` INNER JOIN `order_details` ON `orders`.`OrderID` = `order_details`.`OrderID` INNER JOIN `products` ON `order_details`.`ProductID` = `products`.`ProductID` WHERE `orders`.`OrderDate` >= '2024-01-01' AND `orders`.`OrderDate` <= '2024-12-31' AND `customers`.`Region` = 'North America' GROUP BY `orders`.`OrderID`, `customers`.`CustomerName`, `products`.`ProductName`, `order_details`.`Quantity`, `order_details`.`UnitPrice`, `orders`.`OrderDate`, `orders`.`ShippedDate` HAVING SUM(`order_details`.`Quantity`) > 10 ORDER BY `orders`.`OrderDate` DESC;"

	// Generate a query with 12000+ parameters like in test.csv
	generateLargeParamQuery := func() string {
		// Build the IN clause parameters
		var params []string
		for i := 1; i <= 15000; i++ {
			params = append(params, fmt.Sprintf("$%d", i))
		}
		return fmt.Sprintf("SELECT service_instance_id, CASE WHEN last_seen::timestamptz < NOW() - interval $11111 THEN $22222 ELSE $33333 END FROM apm_telemetry.service_instance WHERE service_instance_id IN (%s)",
			strings.Join(params, ", ")) // The IN clause parameters
	}

	benchmarks := []struct {
		name  string
		query string
	}{
		{"Escaping", `INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`},
		{"Grouping", `INSERT INTO delayed_jobs (created_at, failed_at, handler) VALUES (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL)`},
		{"Large", LargeQuery},
		{"Complex", ComplexQuery},
		{"SuperLarge", fmt.Sprintf(superLargeQuery, 1)},
		{"BracketQuoted", bracketQuotedQuery},
		{"BacktickQuoted", backtickQuotedQuery},
		{"ManyParams", generateLargeParamQuery()},
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
		WithUppercaseKeywords(true),
		WithRemoveSpaceBetweenParentheses(true),
	)

	for _, bm := range benchmarks {
		b.Run(bm.name+"/"+strconv.Itoa(len(bm.query)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := ObfuscateAndNormalize(bm.query, obfuscator, normalizer)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkObfuscationAndNormalizationMore(b *testing.B) {
	tests := []struct {
		input             string
		expected          string
		statementMetadata StatementMetadata
		lexerOpts         []lexerOption
	}{
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
			input:    "SELECT * FROM `database`.`table`",
			expected: "SELECT * FROM database.table",
			statementMetadata: StatementMetadata{
				Tables:     []string{"database.table"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       20,
			},
			lexerOpts: []lexerOption{
				WithDBMS(DBMSMySQL),
			},
		},
		{
			// double quoted table name with non-ascii characters
			input:    `SELECT * FROM "fóo"."users" WHERE id = 1`,
			expected: `SELECT * FROM fóo.users WHERE id = ?`,
			statementMetadata: StatementMetadata{
				Tables:     []string{"fóo.users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       16,
			},
		},
		{
			// double quoted table name with truncated quotes
			input:    `SELECT * FROM "fóo"."us`,
			expected: `SELECT * FROM "fóo"."us`,
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       6,
			},
		},
		{
			// double quoted table name with truncated quotes
			input:    `SELECT "fun" FROM "`,
			expected: `SELECT fun FROM "`,
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       6,
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

	for _, bm := range tests {
		b.Run(bm.input[0:5]+"/"+strconv.Itoa(len(bm.input)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := ObfuscateAndNormalize(bm.input, obfuscator, normalizer, bm.lexerOpts...)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
