package sqllexer

import (
	"fmt"
	"strconv"
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

	benchmarks := []struct {
		name         string
		query        string
		lexerOptions []lexerOption
	}{
		{"Escaping", `INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`, nil},
		{"Grouping", `INSERT INTO delayed_jobs (created_at, failed_at, handler) VALUES (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL)`, nil},
		{"Large", LargeQuery, nil},
		{"Complex", ComplexQuery, nil},
		{"SuperLarge", fmt.Sprintf(superLargeQuery, 1), nil},
		{"BracketQuoted", bracketQuotedQuery, []lexerOption{WithDBMS(DBMSSQLServer)}},
		{"BacktickQuoted", backtickQuotedQuery, []lexerOption{WithDBMS(DBMSMySQL)}},
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
		WithKeepIdentifierQuotation(true),
	)

	for _, bm := range benchmarks {
		b.Run(bm.name+"/"+strconv.Itoa(len(bm.query)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := ObfuscateAndNormalize(bm.query, obfuscator, normalizer, bm.lexerOptions...)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
