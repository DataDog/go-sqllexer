package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DataDog/go-sqllexer"
)

func main() {
	var (
		mode            = flag.String("mode", "obfuscate_and_normalize", "Operation mode: obfuscate, normalize, tokenize, obfuscate_and_normalize")
		inputFile       = flag.String("input", "", "Input file (default: stdin)")
		outputFile      = flag.String("output", "", "Output file (default: stdout)")
		dbms            = flag.String("dbms", "", "Database type: mssql, postgresql, mysql, oracle, snowflake")
		replaceDigits   = flag.Bool("replace-digits", true, "Replace digits with placeholders")
		replaceBoolean  = flag.Bool("replace-boolean", true, "Replace boolean values with placeholders")
		replaceNull     = flag.Bool("replace-null", true, "Replace null values with placeholders")
		keepJsonPath    = flag.Bool("keep-json-path", false, "Keep JSON path expressions")
		collectComments = flag.Bool("collect-comments", true, "Collect comments during normalization")
		collectCommands = flag.Bool("collect-commands", true, "Collect commands during normalization")
		collectTables   = flag.Bool("collect-tables", true, "Collect table names during normalization")
		keepSQLAlias    = flag.Bool("keep-sql-alias", false, "Keep SQL aliases during normalization")
		help            = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Read input
	input, err := readInput(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Process based on mode
	var result string
	switch *mode {
	case "obfuscate":
		result, err = obfuscateSQL(input, *dbms, *replaceDigits, *replaceBoolean, *replaceNull, *keepJsonPath)
	case "normalize":
		result, err = normalizeSQL(input, *dbms, *collectComments, *collectCommands, *collectTables, *keepSQLAlias)
	case "tokenize":
		result, err = tokenizeSQL(input, *dbms)
	case "obfuscate_and_normalize":
		result, err = obfuscateAndNormalizeSQL(input, *dbms, *replaceDigits, *replaceBoolean, *replaceNull, *keepJsonPath, *collectComments, *collectCommands, *collectTables, *keepSQLAlias)
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Use -help for usage information.\n", *mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing SQL: %v\n", err)
		os.Exit(1)
	}

	// Write output
	err = writeOutput(result, *outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
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
		fmt.Print(result)
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

func obfuscateSQL(input, dbms string, replaceDigits, replaceBoolean, replaceNull, keepJsonPath bool) (string, error) {
	obfuscator := sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(replaceDigits),
		sqllexer.WithReplaceBoolean(replaceBoolean),
		sqllexer.WithReplaceNull(replaceNull),
		sqllexer.WithKeepJsonPath(keepJsonPath),
	)

	if dbms != "" {
		result := obfuscator.Obfuscate(input, sqllexer.WithDBMS(sqllexer.DBMSType(dbms)))
		return result, nil
	}

	result := obfuscator.Obfuscate(input)
	return result, nil
}

func normalizeSQL(input, dbms string, collectComments, collectCommands, collectTables, keepSQLAlias bool) (string, error) {
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectComments(collectComments),
		sqllexer.WithCollectCommands(collectCommands),
		sqllexer.WithCollectTables(collectTables),
		sqllexer.WithKeepSQLAlias(keepSQLAlias),
	)

	if dbms != "" {
		result, _, err := normalizer.Normalize(input, sqllexer.WithDBMS(sqllexer.DBMSType(dbms)))
		return result, err
	}

	result, _, err := normalizer.Normalize(input)
	return result, err
}

func obfuscateAndNormalizeSQL(input, dbms string, replaceDigits, replaceBoolean, replaceNull, keepJsonPath bool, collectComments, collectCommands, collectTables, keepSQLAlias bool) (string, error) {
	obfuscator := sqllexer.NewObfuscator(
		sqllexer.WithReplaceDigits(replaceDigits),
		sqllexer.WithReplaceBoolean(replaceBoolean),
		sqllexer.WithReplaceNull(replaceNull),
		sqllexer.WithKeepJsonPath(keepJsonPath),
	)

	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectComments(collectComments),
		sqllexer.WithCollectCommands(collectCommands),
		sqllexer.WithCollectTables(collectTables),
		sqllexer.WithKeepSQLAlias(keepSQLAlias),

		// TODO: Make config param
		sqllexer.WithKeepIdentifierQuotation(true),
	)

	result, _, err := sqllexer.ObfuscateAndNormalize(input, obfuscator, normalizer, sqllexer.WithDBMS(sqllexer.DBMSType(dbms)))
	return result, err
}

func tokenizeSQL(input, dbms string) (string, error) {
	var lexer *sqllexer.Lexer
	if dbms != "" {
		lexer = sqllexer.New(input, sqllexer.WithDBMS(sqllexer.DBMSType(dbms)))
	} else {
		lexer = sqllexer.New(input)
	}

	var result strings.Builder
	for {
		token := lexer.Scan()
		if token.Type == sqllexer.EOF {
			break
		}
		result.WriteString(fmt.Sprintf("%s\n", token.Value))
	}

	return result.String(), nil
}

func printHelp() {
	fmt.Println(`SQL Lexer CLI Tool

Usage: sqllexer [flags]

Flags:
  -mode string
        Operation mode: obfuscate, normalize, tokenize, obfuscate_and_normalize (default "obfuscate_and_normalize")
  -input string
        Input file (default: stdin)
  -output string
        Output file (default: stdout)
  -dbms string
        Database type: mssql, postgresql, mysql, oracle, snowflake
  -replace-digits
        Replace digits with placeholders (default true)
  -replace-boolean
        Replace boolean values with placeholders (default true)
  -replace-null
        Replace null values with placeholders (default true)
  -keep-json-path
        Keep JSON path expressions (default false)
  -collect-comments
        Collect comments during normalization (default true)
  -collect-commands
        Collect commands during normalization (default true)
  -collect-tables
        Collect table names during normalization (default true)
  -keep-sql-alias
        Keep SQL aliases during normalization (default false)
  -help
        Show this help message

Examples:
  # Obfuscate SQL from stdin
  echo "SELECT * FROM users WHERE id = 1" | sqllexer

  # Obfuscate SQL from file
  sqllexer -input query.sql -output obfuscated.sql

  # Normalize SQL for PostgreSQL
  sqllexer -mode normalize -dbms postgresql -input query.sql

  # Tokenize SQL
  sqllexer -mode tokenize -input query.sql

  # Obfuscate with custom options
  sqllexer -replace-digits=false -keep-json-path=true -input query.sql`)
}
