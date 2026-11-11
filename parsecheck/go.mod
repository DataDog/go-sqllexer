module github.com/DataDog/go-sqllexer/parsecheck

go 1.25

require (
	github.com/DataDog/go-sqllexer v0.0.0
	github.com/pganalyze/pg_query_go/v6 v6.2.2
)

require google.golang.org/protobuf v1.31.0 // indirect

replace github.com/DataDog/go-sqllexer => ../
