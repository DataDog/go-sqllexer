package sqllexer

var PostgresFixtures = []DBMSTestFixture{
	{
		queryType: "select",
		name:      "basic_select_with_alias",
		expected:  "SELECT u.id, u.name FROM users u",
		statementMetadata: &StatementMetadata{
			Tables:     []string{"users"},
			Comments:   []string{},
			Commands:   []string{"SELECT"},
			Procedures: []string{},
			Size:       11,
		},
	},
	{
		queryType: "select",
		name:      "basic_select_with_alias",
		expected:  "SELECT u.id AS user_id, u.name AS username FROM users u",
		normalizerConfig: &normalizerConfig{
			KeepSQLAlias: true,
		},
	},
}

func init() {
	RegisterFixture(DBMSPostgres, PostgresFixtures)
}
