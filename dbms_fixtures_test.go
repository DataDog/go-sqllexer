package sqllexer

type DBMSTestFixture struct {
	queryType         string
	name              string
	expected          string
	statementMetadata *StatementMetadata
	obfuscatorConfig  *obfuscatorConfig
	normalizerConfig  *normalizerConfig
}

var Fixtures = make(map[DBMSType][]DBMSTestFixture)

func RegisterFixture(dbmsType DBMSType, fixtures []DBMSTestFixture) {
	Fixtures[dbmsType] = fixtures
}
