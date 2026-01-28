package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DataDog/go-sqllexer"
)

// ObfuscatorConfig holds all obfuscator-related CLI flags
type ObfuscatorConfig struct {
	ReplaceDigits              bool
	ReplaceBoolean             bool
	ReplaceNull                bool
	ReplaceBindParameter       bool
	ReplacePositionalParameter bool
	DollarQuotedFunc           bool
	KeepJsonPath               bool
}

// NormalizerConfig holds all normalizer-related CLI flags
type NormalizerConfig struct {
	CollectComments               bool
	CollectCommands               bool
	CollectTables                 bool
	CollectProcedures             bool
	KeepSQLAlias                  bool
	UppercaseKeywords             bool
	RemoveSpaceBetweenParentheses bool
	KeepTrailingSemicolon         bool
	KeepIdentifierQuotation       bool
}

// CLIConfig holds all CLI configuration
type CLIConfig struct {
	Mode         string
	InputFile    string
	OutputFile   string
	DBMS         string
	WithMetadata bool
	Obfuscator   ObfuscatorConfig
	Normalizer   NormalizerConfig
}

// NewObfuscator creates a sqllexer.Obfuscator from the config
func (c *ObfuscatorConfig) NewObfuscator() *sqllexer.Obfuscator {
	return sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(c.ReplaceDigits),
		sqllexer.WithReplaceBoolean(c.ReplaceBoolean),
		sqllexer.WithReplaceNull(c.ReplaceNull),
		sqllexer.WithReplaceBindParameter(c.ReplaceBindParameter),
		sqllexer.WithReplacePositionalParameter(c.ReplacePositionalParameter),
		sqllexer.WithDollarQuotedFunc(c.DollarQuotedFunc),
		sqllexer.WithKeepJsonPath(c.KeepJsonPath),
	)
}

// NewNormalizer creates a sqllexer.Normalizer from the config
func (c *NormalizerConfig) NewNormalizer() *sqllexer.Normalizer {
	return sqllexer.NewNormalizer(
		sqllexer.WithCollectComments(c.CollectComments),
		sqllexer.WithCollectCommands(c.CollectCommands),
		sqllexer.WithCollectTables(c.CollectTables),
		sqllexer.WithCollectProcedures(c.CollectProcedures),
		sqllexer.WithKeepSQLAlias(c.KeepSQLAlias),
		sqllexer.WithUppercaseKeywords(c.UppercaseKeywords),
		sqllexer.WithRemoveSpaceBetweenParentheses(c.RemoveSpaceBetweenParentheses),
		sqllexer.WithKeepTrailingSemicolon(c.KeepTrailingSemicolon),
		sqllexer.WithKeepIdentifierQuotation(c.KeepIdentifierQuotation),
	)
}

// DBMSType returns the DBMS type for the lexer
func (c *CLIConfig) DBMSType() sqllexer.DBMSType {
	return sqllexer.DBMSType(c.DBMS)
}

func parseFlags() *CLIConfig {
	cfg := &CLIConfig{}

	// General options
	flag.StringVar(&cfg.Mode, "mode", "obfuscate_and_normalize", "Operation mode: obfuscate, normalize, tokenize, obfuscate_and_normalize")
	flag.StringVar(&cfg.InputFile, "input", "", "Input file (default: stdin)")
	flag.StringVar(&cfg.OutputFile, "output", "", "Output file (default: stdout)")
	flag.StringVar(&cfg.DBMS, "dbms", "", "Database type: mssql, postgresql, mysql, oracle, snowflake")
	flag.BoolVar(&cfg.WithMetadata, "with-metadata", false, "Output result with metadata as JSON (normalize and obfuscate_and_normalize modes)")

	// Obfuscator options
	flag.BoolVar(&cfg.Obfuscator.ReplaceDigits, "replace-digits", true, "Replace digits in identifiers with placeholders")
	flag.BoolVar(&cfg.Obfuscator.ReplaceBoolean, "replace-boolean", true, "Replace boolean values with placeholders")
	flag.BoolVar(&cfg.Obfuscator.ReplaceNull, "replace-null", true, "Replace NULL values with placeholders")
	flag.BoolVar(&cfg.Obfuscator.ReplaceBindParameter, "replace-bind-parameter", false, "Replace bind parameters with placeholders")
	flag.BoolVar(&cfg.Obfuscator.ReplacePositionalParameter, "replace-positional-parameter", false, "Replace positional parameters ($1, $2, etc.) with placeholders")
	flag.BoolVar(&cfg.Obfuscator.DollarQuotedFunc, "dollar-quoted-func", false, "Obfuscate content inside $func$...$func$ blocks instead of replacing entirely")
	flag.BoolVar(&cfg.Obfuscator.KeepJsonPath, "keep-json-path", false, "Keep JSON path expressions unobfuscated")

	// Normalizer options
	flag.BoolVar(&cfg.Normalizer.CollectComments, "collect-comments", true, "Collect comments as metadata")
	flag.BoolVar(&cfg.Normalizer.CollectCommands, "collect-commands", true, "Collect SQL commands as metadata")
	flag.BoolVar(&cfg.Normalizer.CollectTables, "collect-tables", true, "Collect table names as metadata")
	flag.BoolVar(&cfg.Normalizer.CollectProcedures, "collect-procedures", false, "Collect procedure names as metadata")
	flag.BoolVar(&cfg.Normalizer.KeepSQLAlias, "keep-sql-alias", false, "Keep SQL aliases (AS clauses)")
	flag.BoolVar(&cfg.Normalizer.UppercaseKeywords, "uppercase-keywords", false, "Uppercase SQL keywords")
	flag.BoolVar(&cfg.Normalizer.RemoveSpaceBetweenParentheses, "remove-space-between-parentheses", false, "Remove spaces inside parentheses")
	flag.BoolVar(&cfg.Normalizer.KeepTrailingSemicolon, "keep-trailing-semicolon", false, "Keep trailing semicolon (useful for PL/SQL)")
	flag.BoolVar(&cfg.Normalizer.KeepIdentifierQuotation, "keep-identifier-quotation", false, "Keep identifier quotes (backticks, double quotes, brackets)")

	flag.Usage = printUsage
	flag.Parse()

	return cfg
}

func main() {
	cfg := parseFlags()

	// Read input
	input, err := readInput(cfg.InputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Process based on mode
	var result string
	switch cfg.Mode {
	case "obfuscate":
		result, err = obfuscate(cfg, input)
	case "normalize":
		result, err = normalize(cfg, input)
	case "tokenize":
		result, err = tokenize(cfg, input)
	case "obfuscate_and_normalize":
		result, err = obfuscateAndNormalize(cfg, input)
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Use -help for usage information.\n", cfg.Mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing SQL: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := writeOutput(result, cfg.OutputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
}

func obfuscate(cfg *CLIConfig, input string) (string, error) {
	obfuscator := cfg.Obfuscator.NewObfuscator()
	return obfuscator.Obfuscate(input, sqllexer.WithDBMS(cfg.DBMSType())), nil
}

func normalize(cfg *CLIConfig, input string) (string, error) {
	normalizer := cfg.Normalizer.NewNormalizer()
	result, metadata, err := normalizer.Normalize(input, sqllexer.WithDBMS(cfg.DBMSType()))
	if err != nil {
		return "", err
	}
	if cfg.WithMetadata {
		return formatWithMetadata(result, metadata)
	}
	return result, nil
}

func obfuscateAndNormalize(cfg *CLIConfig, input string) (string, error) {
	obfuscator := cfg.Obfuscator.NewObfuscator()
	normalizer := cfg.Normalizer.NewNormalizer()

	result, metadata, err := sqllexer.ObfuscateAndNormalize(input, obfuscator, normalizer, sqllexer.WithDBMS(cfg.DBMSType()))
	if err != nil {
		return "", err
	}
	if cfg.WithMetadata {
		return formatWithMetadata(result, metadata)
	}
	return result, nil
}

func tokenize(cfg *CLIConfig, input string) (string, error) {
	lexer := sqllexer.New(input, sqllexer.WithDBMS(cfg.DBMSType()))

	var result strings.Builder
	for {
		token := lexer.Scan()
		if token.Type == sqllexer.EOF {
			break
		}
		result.WriteString(token.Value)
		result.WriteByte('\n')
	}
	return result.String(), nil
}

func readInput(inputFile string) (string, error) {
	var reader io.Reader
	if inputFile == "" {
		reader = os.Stdin
	} else {
		file, err := os.Open(inputFile)
		if err != nil {
			return "", err
		}
		defer file.Close()
		reader = file
	}

	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func writeOutput(result, outputFile string) error {
	if outputFile == "" {
		fmt.Println(result)
		return nil
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(result)
	return err
}

type OutputWithMetadata struct {
	SQL      string                      `json:"sql"`
	Metadata *sqllexer.StatementMetadata `json:"metadata"`
}

func formatWithMetadata(sql string, metadata *sqllexer.StatementMetadata) (string, error) {
	output := OutputWithMetadata{
		SQL:      sql,
		Metadata: metadata,
	}

	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func printUsage() {
	fmt.Println(`SQL Lexer CLI Tool

Usage: sqllexer [flags]

General Flags:
  -mode string
        Operation mode: obfuscate, normalize, tokenize, obfuscate_and_normalize (default "obfuscate_and_normalize")
  -input string
        Input file (default: stdin)
  -output string
        Output file (default: stdout)
  -dbms string
        Database type: mssql, postgresql, mysql, oracle, snowflake
  -with-metadata
        Output result with metadata as JSON (default false)

Obfuscator Flags:
  -replace-digits
        Replace digits in identifiers with placeholders (default true)
  -replace-boolean
        Replace boolean values with placeholders (default true)
  -replace-null
        Replace NULL values with placeholders (default true)
  -replace-bind-parameter
        Replace bind parameters with placeholders (default false)
  -replace-positional-parameter
        Replace positional parameters ($1, $2, etc.) with placeholders (default false)
  -dollar-quoted-func
        Obfuscate content inside $func$...$func$ blocks instead of replacing entirely (default false)
  -keep-json-path
        Keep JSON path expressions unobfuscated (default false)

Normalizer Flags:
  -collect-comments
        Collect comments as metadata (default true)
  -collect-commands
        Collect SQL commands as metadata (default true)
  -collect-tables
        Collect table names as metadata (default true)
  -collect-procedures
        Collect procedure names as metadata (default false)
  -keep-sql-alias
        Keep SQL aliases (AS clauses) (default false)
  -uppercase-keywords
        Uppercase SQL keywords (default false)
  -remove-space-between-parentheses
        Remove spaces inside parentheses (default false)
  -keep-trailing-semicolon
        Keep trailing semicolon (useful for PL/SQL) (default false)
  -keep-identifier-quotation
        Keep identifier quotes (backticks, double quotes, brackets) (default false)

Examples:
  # Obfuscate SQL from stdin
  echo "SELECT * FROM users WHERE id = 1" | sqllexer

  # Obfuscate SQL from file
  sqllexer -input query.sql -output obfuscated.sql

  # Normalize SQL for PostgreSQL
  sqllexer -mode normalize -dbms postgresql -input query.sql

  # Obfuscate and normalize with metadata as JSON
  sqllexer -with-metadata -dbms postgresql -input query.sql

  # Tokenize SQL
  sqllexer -mode tokenize -input query.sql

  # Obfuscate with custom options
  sqllexer -replace-digits=false -keep-json-path=true -input query.sql

  # Strip MySQL backticks and uppercase keywords
  sqllexer -dbms mysql -uppercase-keywords -keep-identifier-quotation=false`)
}
