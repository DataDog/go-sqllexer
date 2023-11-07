package sqllexer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type output struct {
	Expected          string             `json:"expected"`
	ObfuscatorConfig  *obfuscatorConfig  `json:"obfuscator_config,omitempty"`
	NormalizerConfig  *normalizerConfig  `json:"normalizer_config,omitempty"`
	StatementMetadata *StatementMetadata `json:"statement_metadata,omitempty"`
}

type outputList []*output

func getSubdirectories(directory string) ([]string, error) {
	var subdirs []string
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, entry.Name())
		}
	}
	return subdirs, nil
}

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
		// Get all subdirectories of the testdata folder
		queryTypes, err := getSubdirectories(baseDir)
		if err != nil {
			t.Fatal(err)
		}

		for _, qt := range queryTypes {
			dirPath := filepath.Join(baseDir, qt)
			files, err := os.ReadDir(dirPath)
			if err != nil {
				t.Fatal(err)
			}

			for _, file := range files {
				// the testdata folder contains only .sql and .expected files
				// so we can safely ignore the other files
				if strings.HasSuffix(file.Name(), ".sql") {
					// Remove the .sql extension to get the test name
					testName := strings.TrimSuffix(file.Name(), ".sql")
					t.Run(testName, func(t *testing.T) {
						queryPath := filepath.Join(dirPath, file.Name())
						expectedPath := filepath.Join(dirPath, testName+".expected")

						input, err := os.ReadFile(queryPath)
						if err != nil {
							t.Fatal(err)
						}

						expectedJson, err := os.ReadFile(expectedPath)
						if err != nil {
							t.Fatal(err)
						}

						var expectedOutputs outputList

						if err := json.Unmarshal(expectedJson, &expectedOutputs); err != nil {
							t.Fatal(err)
						}

						var defaultObfuscatorConfig *obfuscatorConfig
						var defaultNormalizerConfig *normalizerConfig

						for _, output := range expectedOutputs {
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
							assert.Equal(t, output.Expected, got)

							// Compare the expected statement metadata with the actual statement metadata
							if output.StatementMetadata != nil {
								assert.Equal(t, output.StatementMetadata, statementMetadata)
							}
						}
					})
				}
			}
		}
	}

}
