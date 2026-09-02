// Command corpusgen builds the shared harness corpora from sources that already
// exist in the repository, without modifying any of them.
//
// Sources:
//   - testdata/**.json   the DBMS suite, including each case's per-output configs
//   - *_bench_test.go    benchmark query literals, extracted via the Go parser
//   - a Go fuzz corpus directory (optional), e.g.
//     $(go env GOCACHE)/fuzz/github.com/DataDog/go-sqllexer/FuzzObfuscatorAndNormalizer
//
// Every emitted request is a protocol.Request, so the same corpus drives the
// differential runs and the throughput harness.
//
//	go run ./harness/cmd/corpusgen -out harness/corpus
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/DataDog/go-sqllexer/harness/internal/corpus"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// testcaseFile mirrors the shape of testdata/**.json (see testdata/README.md).
type testcaseFile struct {
	Input   string `json:"input"`
	Outputs []struct {
		Expected         string                     `json:"expected"`
		ObfuscatorConfig *protocol.ObfuscatorConfig `json:"obfuscator_config"`
		NormalizerConfig *protocol.NormalizerConfig `json:"normalizer_config"`
	} `json:"outputs"`
}

const (
	// repoRoot is where testdata and the benchmark sources are read from; the
	// generator is always run from the repository root.
	repoRoot = "."
	// benchMinLen is the shortest benchmark string literal still treated as a query.
	benchMinLen = 24
	// matrixSize and matrixSeed keep the configuration matrix reproducible.
	matrixSize = 20000
	matrixSeed = 1
)

func main() {
	var (
		outDir  = flag.String("out", "harness/corpus", "directory to write corpora into")
		fuzzDir = flag.String("fuzz-corpus", "", "optional Go fuzz corpus directory to import as seeds")
	)
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("create out dir: %v", err)
	}

	if n, err := writeCorpus(filepath.Join(*outDir, "testdata.jsonl"), func(w *corpus.Writer) error {
		return genTestdata(repoRoot, w)
	}); err != nil {
		log.Fatalf("testdata corpus: %v", err)
	} else {
		fmt.Printf("testdata.jsonl        %6d requests\n", n)
	}

	if n, err := writeCorpus(filepath.Join(*outDir, "workloads.jsonl"), func(w *corpus.Writer) error {
		return genBenchQueries(repoRoot, benchMinLen, w)
	}); err != nil {
		log.Fatalf("workload corpus: %v", err)
	} else {
		fmt.Printf("workloads.jsonl       %6d requests\n", n)
	}

	if n, err := writeCorpus(filepath.Join(*outDir, "pathological.jsonl"), genPathological); err != nil {
		log.Fatalf("pathological corpus: %v", err)
	} else {
		fmt.Printf("pathological.jsonl    %6d requests\n", n)
	}

	if n, err := writeCorpus(filepath.Join(*outDir, "matrix.jsonl"), func(w *corpus.Writer) error {
		return genMatrix(repoRoot, matrixSize, matrixSeed, w)
	}); err != nil {
		log.Fatalf("matrix corpus: %v", err)
	} else {
		fmt.Printf("matrix.jsonl          %6d requests\n", n)
	}

	if *fuzzDir != "" {
		if n, err := writeCorpus(filepath.Join(*outDir, "fuzzseeds.jsonl"), func(w *corpus.Writer) error {
			return genFuzzSeeds(*fuzzDir, w)
		}); err != nil {
			log.Fatalf("fuzz corpus: %v", err)
		} else {
			fmt.Printf("fuzzseeds.jsonl       %6d requests\n", n)
		}
	}
}

func writeCorpus(path string, fn func(*corpus.Writer) error) (int, error) {
	w, err := corpus.NewWriter(path)
	if err != nil {
		return 0, err
	}
	if err := fn(w); err != nil {
		w.Close()
		return 0, err
	}
	return w.Count(), w.Close()
}

// genTestdata emits one request per (test case, output) pair, carrying the exact
// configuration that the Go suite applies to that pair. The expected values are
// deliberately not copied: the corpus feeds differential comparison, and the
// authoritative expectations stay in testdata/, which this project never edits.
func genTestdata(repoRoot string, w *corpus.Writer) error {
	base := filepath.Join(repoRoot, "testdata")
	return filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tc testcaseFile
		if err := json.Unmarshal(raw, &tc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		dbms := strings.Split(filepath.ToSlash(rel), "/")[0]
		id := strings.TrimSuffix(filepath.ToSlash(rel), ".json")

		for i, out := range tc.Outputs {
			// Configs are always materialized: a fixture that omits one is run by the
			// Go suite with the fixture defaults, which are not the library defaults.
			obfuscator := protocol.FixtureObfuscatorConfig()
			if out.ObfuscatorConfig != nil {
				obfuscator = *out.ObfuscatorConfig
			}
			normalizer := protocol.FixtureNormalizerConfig()
			if out.NormalizerConfig != nil {
				normalizer = *out.NormalizerConfig
			}
			req := protocol.Request{
				ID:         fmt.Sprintf("testdata/%s#%d", id, i),
				SQL:        protocol.Text(tc.Input),
				DBMS:       dbms,
				Mode:       protocol.ModeObfuscateAndNormalize,
				Obfuscator: &obfuscator,
				Normalizer: &normalizer,
			}
			if err := w.Write(req); err != nil {
				return err
			}
		}
		return nil
	})
}

// genBenchQueries extracts the SQL literals used by the existing benchmarks so both
// implementations can be benchmarked on byte-identical inputs. Parsing the AST keeps
// the benchmark files themselves untouched.
func genBenchQueries(repoRoot string, minLen int, w *corpus.Writer) error {
	matches, err := filepath.Glob(filepath.Join(repoRoot, "*_bench_test.go"))
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	fset := token.NewFileSet()

	for _, path := range matches {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		source := strings.TrimSuffix(filepath.Base(path), "_bench_test.go")

		var walkErr error
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || walkErr != nil {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil || len(value) < minLen || !looksLikeSQL(value) {
				return true
			}
			sum := sha256.Sum256([]byte(value))
			key := hex.EncodeToString(sum[:8])
			if seen[key] {
				return true
			}
			seen[key] = true
			walkErr = w.Write(protocol.Request{
				ID:   fmt.Sprintf("bench/%s/%s/%d", source, key, len(value)),
				Mode: protocol.ModeObfuscateAndNormalize,
				SQL:  protocol.Text(value),
			})
			return true
		})
		if walkErr != nil {
			return walkErr
		}
	}
	return nil
}

// sqlPrefixes are the statement heads used by the benchmark corpus; matching on them
// keeps helper strings (format verbs, benchmark names) out of the workload corpus.
var sqlPrefixes = []string{
	"select", "insert", "update", "delete", "with", "create", "alter", "drop",
	"copy", "grant", "revoke", "begin", "declare", "call", "explain", "merge",
	"truncate", "set", "show", "use", "/*", "--",
}

func looksLikeSQL(s string) bool {
	trimmed := strings.ToLower(strings.TrimLeft(s, " \t\r\n"))
	for _, p := range sqlPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}

// genPathological emits inputs that stress the parts of the implementation most
// likely to differ or to degrade non-linearly: very long parameter lists, deep
// nesting, unterminated constructs, and non-UTF-8 bytes. The benchmark suite builds
// its largest inputs programmatically, so they cannot be recovered from source
// literals and are reconstructed here instead.
func genPathological(w *corpus.Writer) error {
	params := make([]string, 10000)
	for i := range params {
		params[i] = fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2)
	}

	inputs := map[string]string{
		"many-params":      "INSERT INTO events (id, payload) VALUES " + strings.Join(params, ", "),
		"deep-nesting":     strings.Repeat("SELECT * FROM (", 500) + "SELECT 1" + strings.Repeat(")", 500),
		"long-in-list":     "SELECT * FROM t WHERE id IN (" + strings.Repeat("1, ", 20000) + "1)",
		"long-dollar":      "SELECT $tag$" + strings.Repeat("body ", 20000) + "$tag$",
		"unterminated-str": "SELECT * FROM t WHERE name = 'never closed",
		"unterminated-cmt": "SELECT 1 /* never closed",
		"truncated":        "SELECT id, name FROM users WHERE id IN (1, 2,",
		"invalid-utf8":     "SELECT * FROM t WHERE name = '\xff\xfe\x00'",
		"multibyte":        "SELECT * FROM \"таблица\" WHERE \"колонка\" = '日本語テキスト' -- コメント",
		"only-comments":    strings.Repeat("-- comment line\n", 5000),
		"whitespace":       strings.Repeat(" \t\n", 10000) + "SELECT 1",
		"empty":            "",
	}

	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, dbms := range dbmsTypes {
			for _, mode := range protocol.Modes {
				obfuscator := protocol.FixtureObfuscatorConfig()
				normalizer := protocol.FixtureNormalizerConfig()
				if err := w.Write(protocol.Request{
					ID:         fmt.Sprintf("pathological/%s/%s/%s", name, dbmsLabel(dbms), mode),
					SQL:        protocol.Text(inputs[name]),
					DBMS:       dbms,
					Mode:       mode,
					Obfuscator: &obfuscator,
					Normalizer: &normalizer,
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func dbmsLabel(dbms string) string {
	if dbms == "" {
		return "default"
	}
	return dbms
}

// dbmsTypes are every DBMS the library supports, including the unset default.
var dbmsTypes = []string{"", "postgresql", "mysql", "mssql", "oracle", "snowflake"}

// genMatrix crosses statements with randomly sampled option combinations. The
// option space (2^7 obfuscator x 2^9 normalizer x 6 DBMS x 4 modes) is far too
// large to enumerate against every statement, so coverage comes from seeded
// sampling: the same seed always yields the same corpus, which keeps a mismatch
// reproducible, while nightly runs widen coverage with a different seed.
func genMatrix(repoRoot string, count int, seed int64, w *corpus.Writer) error {
	statements, err := collectStatements(repoRoot)
	if err != nil {
		return err
	}
	if len(statements) == 0 {
		return fmt.Errorf("no statements found under %s", filepath.Join(repoRoot, "testdata"))
	}

	rng := rand.New(rand.NewSource(seed))
	flip := func() bool { return rng.Intn(2) == 1 }

	for i := 0; i < count; i++ {
		obfuscator := protocol.ObfuscatorConfig{
			DollarQuotedFunc:           flip(),
			ReplaceDigits:              flip(),
			ReplacePositionalParameter: flip(),
			ReplaceBoolean:             flip(),
			ReplaceNull:                flip(),
			KeepJsonPath:               flip(),
			ReplaceBindParameter:       flip(),
		}
		normalizer := protocol.NormalizerConfig{
			CollectTables:                 flip(),
			CollectCommands:               flip(),
			CollectComments:               flip(),
			CollectProcedure:              flip(),
			KeepSQLAlias:                  flip(),
			UppercaseKeywords:             flip(),
			RemoveSpaceBetweenParentheses: flip(),
			KeepTrailingSemicolon:         flip(),
			KeepIdentifierQuotation:       flip(),
		}
		req := protocol.Request{
			ID:         fmt.Sprintf("matrix/%d/%d", seed, i),
			SQL:        protocol.Text(statements[rng.Intn(len(statements))]),
			DBMS:       dbmsTypes[rng.Intn(len(dbmsTypes))],
			Mode:       protocol.Modes[rng.Intn(len(protocol.Modes))],
			Obfuscator: &obfuscator,
			Normalizer: &normalizer,
		}
		if err := w.Write(req); err != nil {
			return err
		}
	}
	return nil
}

func collectStatements(repoRoot string) ([]string, error) {
	var statements []string
	seen := map[string]bool{}
	err := filepath.Walk(filepath.Join(repoRoot, "testdata"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tc testcaseFile
		if err := json.Unmarshal(raw, &tc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !seen[tc.Input] {
			seen[tc.Input] = true
			statements = append(statements, tc.Input)
		}
		return nil
	})
	return statements, err
}

// genFuzzSeeds imports a Go fuzz corpus directory. Each file holds one testdata
// entry in the `go test fuzz v1` format; the SQL argument is the last string value.
func genFuzzSeeds(dir string, w *corpus.Writer) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		sql, ok := parseFuzzSeed(string(raw))
		if !ok {
			continue
		}
		if err := w.Write(protocol.Request{
			ID:   "fuzz/" + e.Name(),
			Mode: protocol.ModeObfuscateAndNormalize,
			SQL:  protocol.Text(sql),
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseFuzzSeed(content string) (string, bool) {
	var last string
	found := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "string(") || !strings.HasSuffix(line, ")") {
			continue
		}
		value, err := strconv.Unquote(strings.TrimSuffix(strings.TrimPrefix(line, "string("), ")"))
		if err != nil {
			continue
		}
		last, found = value, true
	}
	return last, found
}
