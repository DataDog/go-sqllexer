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
	Expected         string            `json:"expected"`
	ObfuscatorConfig *obfuscatorConfig `json:"obfuscator_config,omitempty"`
	NormalizerConfig *normalizerConfig `json:"normalizer_config,omitempty"`
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

func TestQueriesPerDBMS(t *testing.T) {
	tests := []struct {
		dbms DBMSType
	}{
		{
			dbms: DBMSPostgres,
		},
	}

	for _, tt := range tests {
		baseDir := filepath.Join("testdata", string(tt.dbms))
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
				if strings.HasSuffix(file.Name(), ".sql") {
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

							got, _, err := ObfuscateAndNormalize(string(input), obfuscator, normalizer, WithDBMS(tt.dbms))

							if err != nil {
								t.Fatal(err)
							}

							assert.Equal(t, output.Expected, got)
						}
					})
				}
			}
		}
	}

}
