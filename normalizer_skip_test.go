package sqllexer

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizerSkipNormalization(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		statementMetadata StatementMetadata
	}{
		{
			name:  "keeps spacing, casing, quotes and literals",
			input: "select   *\nFROM  \"Users\"  u   where u.id = 42 and name = 'bob'",
			statementMetadata: StatementMetadata{
				Tables:     []string{"Users"},
				Comments:   []string{},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       11,
			},
		},
		{
			name:  "collects comments from the original text",
			input: "/* trace=1 */ SELECT a FROM t -- tail",
			statementMetadata: StatementMetadata{
				Tables:     []string{"t"},
				Comments:   []string{"/* trace=1 */", "-- tail"},
				Commands:   []string{"SELECT"},
				Procedures: []string{},
				Size:       27,
			},
		},
		{
			name:  "skips CTE names and keeps the table list",
			input: "WITH recent AS (SELECT * FROM orders) SELECT * FROM recent JOIN users ON 1 = 1",
			statementMetadata: StatementMetadata{
				Tables:     []string{"orders", "users"},
				Comments:   []string{},
				Commands:   []string{"SELECT", "JOIN"},
				Procedures: []string{},
				Size:       21,
			},
		},
		{
			name:  "collects procedures",
			input: "CREATE PROCEDURE  sp_clean  AS SELECT 1",
			statementMetadata: StatementMetadata{
				Tables:     []string{},
				Comments:   []string{},
				Commands:   []string{"CREATE", "SELECT"},
				Procedures: []string{"sp_clean"},
				Size:       20,
			},
		},
	}

	normalizer := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithCollectProcedures(true),
		WithSkipNormalization(true),
	)

	full := NewNormalizer(
		WithCollectComments(true),
		WithCollectCommands(true),
		WithCollectTables(true),
		WithCollectProcedures(true),
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, statementMetadata, err := normalizer.Normalize(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.input, got)
			assertStatementMetadataEqual(t, &test.statementMetadata, statementMetadata)

			_, normalizedMetadata, err := full.Normalize(test.input)
			require.NoError(t, err)
			assert.Equal(t, normalizedMetadata, statementMetadata)
		})
	}
}

func TestNormalizerSkipNormalizationWithoutCollecting(t *testing.T) {
	normalizer := NewNormalizer(WithSkipNormalization(true))
	input := "SELECT  *  FROM t WHERE id = 1"
	got, statementMetadata, err := normalizer.Normalize(input)
	require.NoError(t, err)
	assert.Equal(t, input, got)
	assertStatementMetadataEqual(t, &StatementMetadata{
		Tables:     []string{},
		Comments:   []string{},
		Commands:   []string{},
		Procedures: []string{},
	}, statementMetadata)
}

// ObfuscateAndNormalize must keep producing obfuscated text, so the option
// cannot turn it into a pass that returns the raw query.
func TestObfuscateAndNormalizeIgnoresSkipNormalization(t *testing.T) {
	obfuscator := NewObfuscator(WithReplaceDigits(true))
	normalizer := NewNormalizer(WithCollectTables(true), WithSkipNormalization(true))

	got, statementMetadata, err := ObfuscateAndNormalize("SELECT  *  FROM users WHERE ssn = 123456", obfuscator, normalizer)
	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE ssn = ?", got)
	assert.Equal(t, []string{"users"}, statementMetadata.Tables)
}

// TestSkipNormalizationMatchesNormalizedMetadata checks that every query in the
// per DBMS fixtures yields the same metadata with and without normalization.
func TestSkipNormalizationMatchesNormalizedMetadata(t *testing.T) {
	for _, dbms := range []DBMSType{DBMSPostgres, DBMSOracle, DBMSSQLServer, DBMSMySQL, DBMSSnowflake} {
		baseDir := filepath.Join("testdata", string(dbms))
		queryTypes, err := testdata.ReadDir(baseDir)
		require.NoError(t, err)

		for _, qt := range queryTypes {
			dirPath := filepath.Join(baseDir, qt.Name())
			files, err := testdata.ReadDir(dirPath)
			require.NoError(t, err)

			for _, file := range files {
				raw, err := testdata.ReadFile(filepath.Join(dirPath, file.Name()))
				require.NoError(t, err)
				var tt testcase
				require.NoError(t, json.Unmarshal(raw, &tt))

				for _, keepQuotes := range []bool{false, true} {
					opts := []normalizerOption{
						WithCollectComments(true),
						WithCollectCommands(true),
						WithCollectTables(true),
						WithCollectProcedures(true),
						WithKeepIdentifierQuotation(keepQuotes),
					}
					full := NewNormalizer(opts...)
					skip := NewNormalizer(append(opts, WithSkipNormalization(true))...)

					_, want, err := full.Normalize(tt.Input, WithDBMS(dbms))
					require.NoError(t, err, file.Name())
					got, have, err := skip.Normalize(tt.Input, WithDBMS(dbms))
					require.NoError(t, err, file.Name())

					assert.Equal(t, tt.Input, got, "%s/%s", dbms, file.Name())
					assert.Equal(t, want, have, "%s/%s", dbms, file.Name())
				}
			}
		}
	}
}
