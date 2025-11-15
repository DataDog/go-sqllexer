package sqllexer

import (
	"embed"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/*
var testdata embed.FS

type output struct {
	Expected          string             `json:"expected"`
	ObfuscatorConfig  *obfuscatorConfig  `json:"obfuscator_config,omitempty"`
	NormalizerConfig  *normalizerConfig  `json:"normalizer_config,omitempty"`
	StatementMetadata *StatementMetadata `json:"statement_metadata,omitempty"`
}

type testcase struct {
	Input   string   `json:"input"`
	Outputs []output `json:"outputs"`
}

// TestQueriesPerDBMS tests a preset of queries and expected output per DBMS
// Test folder structure:
// -- testdata
//
//	-- dbms_type
//	  -- query_type
//	    -- query_name.json
func TestQueriesPerDBMS(t *testing.T) {
	dbmsTypes := []DBMSType{
		DBMSPostgres,
		DBMSOracle,
		DBMSSQLServer,
		DBMSMySQL,
		DBMSSnowflake,
		DBMSSQLite,
	}

	for _, dbms := range dbmsTypes {
		// Get all subdirectories of the testdata folder
		baseDir := filepath.Join("testdata", string(dbms))
		// Get all subdirectories of the testdata folder
		queryTypes, err := testdata.ReadDir(baseDir)
		if err != nil {
			t.Fatal(err)
		}

		for _, qt := range queryTypes {
			dirPath := filepath.Join(baseDir, qt.Name())
			files, err := testdata.ReadDir(dirPath)
			if err != nil {
				t.Fatal(err)
			}

			for _, file := range files {
				testName := strings.TrimSuffix(file.Name(), ".json")
				t.Run(testName, func(t *testing.T) {
					queryPath := filepath.Join(dirPath, file.Name())

					testfile, err := testdata.ReadFile(queryPath)
					if err != nil {
						t.Fatal(err)
					}

					var tt testcase

					if err := json.Unmarshal(testfile, &tt); err != nil {
						t.Fatal(err)
					}

					var defaultObfuscatorConfig *obfuscatorConfig
					var defaultNormalizerConfig *normalizerConfig

					for _, output := range tt.Outputs {
						// If the test case has a custom obfuscator or normalizer config
						// use it, otherwise use the default config
						if output.ObfuscatorConfig != nil {
							defaultObfuscatorConfig = output.ObfuscatorConfig
						} else {
							defaultObfuscatorConfig = &obfuscatorConfig{
								DollarQuotedFunc:           true,
								ReplaceDigits:              true,
								ReplacePositionalParameter: true,
								ReplaceBoolean:             true,
								ReplaceNull:                true,
								KeepJsonPath:               false,
							}
						}

						if output.NormalizerConfig != nil {
							defaultNormalizerConfig = output.NormalizerConfig
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
								KeepIdentifierQuotation:       false,
							}
						}

						obfuscator := NewObfuscator(
							WithDollarQuotedFunc(defaultObfuscatorConfig.DollarQuotedFunc),
							WithReplaceDigits(defaultObfuscatorConfig.ReplaceDigits),
							WithReplacePositionalParameter(defaultObfuscatorConfig.ReplacePositionalParameter),
							WithReplaceBoolean(defaultObfuscatorConfig.ReplaceBoolean),
							WithReplaceNull(defaultObfuscatorConfig.ReplaceNull),
							WithKeepJsonPath(defaultObfuscatorConfig.KeepJsonPath),
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
							WithKeepIdentifierQuotation(defaultNormalizerConfig.KeepIdentifierQuotation),
						)

						got, statementMetadata, err := ObfuscateAndNormalize(string(tt.Input), obfuscator, normalizer, WithDBMS(dbms))

						if err != nil {
							t.Fatal(err)
						}

						// Compare the expected output with the actual output
						assert.Equal(t, output.Expected, got)

						// Compare the expected statement metadata with the actual statement metadata
						if output.StatementMetadata != nil {
							assertStatementMetadataEqual(t, output.StatementMetadata, statementMetadata)
						}
					}
				})
			}
		}
	}
}
