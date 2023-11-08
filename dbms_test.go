package sqllexer

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/*
var testdata embed.FS

// TestQueriesPerDBMS tests a preset of queries and expected output per DBMS
// Test folder structure:
// -- testdata
//
//	-- dbms_type
//	  -- query_type
//	    -- query_name.sql
//	    -- query_name.expected
func TestQueriesPerDBMS(t *testing.T) {
	dbmsTypes := []DBMSType{
		DBMSPostgres,
	}

	for _, dbms := range dbmsTypes {
		// Get all subdirectories of the testdata folder
		baseDir := filepath.Join("testdata", string(dbms))

		for _, fixture := range Fixtures[dbms] {
			queryPath := filepath.Join(baseDir, fixture.queryType, fixture.name+".sql")
			t.Run(fixture.name, func(t *testing.T) {
				input, err := testdata.ReadFile(queryPath)
				if err != nil {
					t.Fatal(err)
				}

				var defaultObfuscatorConfig *obfuscatorConfig
				var defaultNormalizerConfig *normalizerConfig

				// If the test case has a custom obfuscator or normalizer config
				// use it, otherwise use the default config
				if fixture.obfuscatorConfig != nil {
					defaultObfuscatorConfig = fixture.obfuscatorConfig
				} else {
					defaultObfuscatorConfig = &obfuscatorConfig{
						DollarQuotedFunc:           true,
						ReplaceDigits:              true,
						ReplacePositionalParameter: true,
						ReplaceBoolean:             true,
						ReplaceNull:                true,
					}
				}

				if fixture.normalizerConfig != nil {
					defaultNormalizerConfig = fixture.normalizerConfig
				} else {
					defaultNormalizerConfig = &normalizerConfig{
						CollectComments:               true,
						CollectCommands:               true,
						CollectTables:                 true,
						CollectProcedure:              true,
						KeepSQLAlias:                  false,
						UppercaseKeywords:             false,
						RemoveSpaceBetweenParentheses: false,
						KeepTrailingSemicolon:         false,
					}
				}

				obfuscator := NewObfuscator(
					WithDollarQuotedFunc(defaultObfuscatorConfig.DollarQuotedFunc),
					WithReplaceDigits(defaultObfuscatorConfig.ReplaceDigits),
					WithReplacePositionalParameter(defaultObfuscatorConfig.ReplacePositionalParameter),
					WithReplaceBoolean(defaultObfuscatorConfig.ReplaceBoolean),
					WithReplaceNull(defaultObfuscatorConfig.ReplaceNull),
				)

				normalizer := NewNormalizer(
					WithCollectComments(defaultNormalizerConfig.CollectComments),
					WithCollectCommands(defaultNormalizerConfig.CollectCommands),
					WithCollectTables(defaultNormalizerConfig.CollectTables),
					WithCollectProcedures(defaultNormalizerConfig.CollectProcedure),
					WithKeepSQLAlias(defaultNormalizerConfig.KeepSQLAlias),
					WithUppercaseKeywords(defaultNormalizerConfig.UppercaseKeywords),
					WithRemoveSpaceBetweenParentheses(defaultNormalizerConfig.RemoveSpaceBetweenParentheses),
					WithKeepTrailingSemicolon(defaultNormalizerConfig.KeepTrailingSemicolon),
				)

				got, statementMetadata, err := ObfuscateAndNormalize(string(input), obfuscator, normalizer, WithDBMS(dbms))

				if err != nil {
					t.Fatal(err)
				}

				// Compare the expected output with the actual output
				assert.Equal(t, fixture.expected, got)

				// Compare the expected statement metadata with the actual statement metadata
				if fixture.statementMetadata != nil {
					assert.Equal(t, fixture.statementMetadata, statementMetadata)
				}
			})
		}
	}
}
