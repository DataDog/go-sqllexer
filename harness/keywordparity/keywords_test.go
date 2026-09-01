// Package keywordparity guards the one part of the Rust port that the differential
// corpora cannot reach exhaustively: the keyword tables are transcribed by hand, so
// a word added to sqllexer_utils.go would silently be missing on the Rust side for
// every input that does not happen to contain it. This test compares the two
// sources directly.
package keywordparity

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

var (
	goListPattern   = `\b%s = \[\]string\{(?s:(.*?))\n\t*\}`
	rustListPattern = `static %s: \[&str; \d+\] = \[(?s:(.*?))\];`
	stringPattern   = regexp.MustCompile(`"([^"]*)"`)
)

func extract(t *testing.T, source, pattern, name string) []string {
	t.Helper()
	re := regexp.MustCompile(fmt.Sprintf(pattern, name))
	match := re.FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("no list named %q found", name)
	}
	var values []string
	for _, hit := range stringPattern.FindAllStringSubmatch(match[1], -1) {
		values = append(values, hit[1])
	}
	sort.Strings(values)
	return values
}

func TestKeywordTablesMatch(t *testing.T) {
	goSource, err := os.ReadFile(filepath.Join("..", "..", "sqllexer_utils.go"))
	if err != nil {
		t.Fatal(err)
	}
	rustSource, err := os.ReadFile(filepath.Join("..", "..", "rust", "sqllexer", "src", "keywords.rs"))
	if err != nil {
		t.Fatal(err)
	}

	for _, pair := range []struct{ goName, rustName string }{
		{"commands", "COMMANDS"},
		{"tableIndicatorCommands", "TABLE_INDICATOR_COMMANDS"},
		{"tableIndicatorKeywords", "TABLE_INDICATOR_KEYWORDS"},
		{"keywords", "KEYWORDS"},
		{"booleanValues", "BOOLEAN_VALUES"},
		{"nullValues", "NULL_VALUES"},
		{"procedureNames", "PROCEDURE_NAMES"},
		{"ctes", "CTES"},
		{"alias", "ALIAS"},
	} {
		want := extract(t, string(goSource), goListPattern, pair.goName)
		got := extract(t, string(rustSource), rustListPattern, pair.rustName)
		if len(want) == 0 {
			t.Fatalf("%s: parsed an empty list from the Go source", pair.goName)
		}
		if len(want) != len(got) {
			t.Fatalf("%s: go has %d entries, rust %s has %d", pair.goName, len(want), pair.rustName, len(got))
		}
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("%s vs %s differ: go %q, rust %q", pair.goName, pair.rustName, want, got)
			}
		}
	}
}
