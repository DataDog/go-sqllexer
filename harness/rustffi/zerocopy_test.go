//go:build rustffi

package rustffi_test

// Tests for the low-allocation binding: the packed decode used by the owning
// entry points, and the lifetime rules of the borrowing ones. The point of most
// of them is the memory a result points at, not its bytes, so they assert on the
// address of the string data rather than only on equality.

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/DataDog/go-sqllexer"
	"github.com/DataDog/go-sqllexer/harness/rustffi"
)

func dataPointer(s string) uintptr {
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}

const withMetadata = "/* c */ SELECT a FROM first_table JOIN second_table ON 1 = 1 WHERE x = 42"

// The owning API must not alias anything the handle can write to again: two
// calls that produce identical bytes must still produce distinct memory.
func TestOwnedResultsDoNotAliasHandleMemory(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	first, firstMetadata, err := processor.ObfuscateAndNormalize(withMetadata, "")
	if err != nil {
		t.Fatal(err)
	}
	second, secondMetadata, err := processor.ObfuscateAndNormalize(withMetadata, "")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("same input produced different output: %q then %q", first, second)
	}
	if dataPointer(first) == dataPointer(second) {
		t.Fatal("two owned results share memory; one of them will be corrupted")
	}
	if len(firstMetadata.Tables) == 0 {
		t.Fatal("test statement produced no tables")
	}
	if dataPointer(firstMetadata.Tables[0]) == dataPointer(secondMetadata.Tables[0]) {
		t.Fatal("two owned metadata values share memory")
	}
}

// The packed decode hands out several strings that are windows onto one byte
// slice. Every one of them has to survive the handle being reused and closed.
func TestOwnedResultsSurviveReuseAndClose(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)

	sql, metadata, err := processor.ObfuscateAndNormalize(withMetadata, "")
	if err != nil {
		t.Fatal(err)
	}
	normalized, normalizedMetadata, err := processor.Normalize(withMetadata, "")
	if err != nil {
		t.Fatal(err)
	}
	wantSQL, wantTables := sql, append([]string(nil), metadata.Tables...)
	wantNormalized := normalized

	// Churn the handle's buffers: different shapes, different lengths, other
	// entry points, and finally a much larger statement so every buffer is
	// reallocated rather than overwritten in place.
	for _, other := range []string{
		"SELECT 1",
		"/* x */ DELETE FROM third_table WHERE id IN (1, 2, 3)",
		strings.Repeat("SELECT * FROM padding_table WHERE v = 'x'; ", 500),
	} {
		if _, _, err := processor.ObfuscateAndNormalize(other, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := processor.Obfuscate(other, ""); err != nil {
			t.Fatal(err)
		}
		if _, _, err := processor.Normalize(other, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := processor.Tokenize(other, ""); err != nil {
			t.Fatal(err)
		}
	}
	processor.Close()

	if sql != wantSQL || normalized != wantNormalized {
		t.Fatalf("results were corrupted: %q / %q", sql, normalized)
	}
	if strings.Join(metadata.Tables, ",") != strings.Join(wantTables, ",") {
		t.Fatalf("metadata was corrupted: %q", metadata.Tables)
	}
	if len(normalizedMetadata.Tables) != len(wantTables) {
		t.Fatalf("normalize metadata was corrupted: %q", normalizedMetadata.Tables)
	}
}

// Metadata lists are sub-slices of one backing array. A caller appending to one
// must not be able to reach into the next, so each is capped at its length.
func TestMetadataListsCannotOverwriteEachOther(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	_, metadata, err := processor.ObfuscateAndNormalize(withMetadata, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Commands) == 0 || len(metadata.Tables) == 0 {
		t.Fatalf("test statement produced no commands or tables: %+v", metadata)
	}
	commands := append([]string(nil), metadata.Commands...)
	metadata.Tables = append(metadata.Tables, "appended")
	if strings.Join(metadata.Commands, ",") != strings.Join(commands, ",") {
		t.Fatalf("appending to Tables overwrote Commands: %q", metadata.Commands)
	}
}

// Empty lists must stay non-nil, because the Go implementation's are, and the
// differ compares the two.
func TestEmptyMetadataListsAreNotNil(t *testing.T) {
	processor := rustffi.NewProcessor(0, 0)
	defer processor.Close()

	_, metadata, err := processor.ObfuscateAndNormalize("", "")
	if err != nil {
		t.Fatal(err)
	}
	for name, list := range map[string][]string{
		"tables": metadata.Tables, "comments": metadata.Comments,
		"commands": metadata.Commands, "procedures": metadata.Procedures,
	} {
		if list == nil {
			t.Fatalf("%s is nil", name)
		}
	}
}

// The borrowing entry point trades the copy for a lifetime rule. This is the
// test that pins the rule: its strings point into the handle's buffers, which is
// exactly why they must not be read after the next call.
func TestBorrowedResultsAliasTheHandleAndMatchTheOwningAPI(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	var borrowed rustffi.Borrowed
	if err := processor.ObfuscateAndNormalizeInto(withMetadata, "", &borrowed); err != nil {
		t.Fatal(err)
	}
	firstSQL := dataPointer(borrowed.SQL)
	firstTable := dataPointer(borrowed.Tables[0])

	if err := processor.ObfuscateAndNormalizeInto(withMetadata, "", &borrowed); err != nil {
		t.Fatal(err)
	}
	if dataPointer(borrowed.SQL) != firstSQL || dataPointer(borrowed.Tables[0]) != firstTable {
		t.Fatal("borrowed results were copied; the API's whole point is that they are not")
	}

	// Same values as the owning API, for every corpus-shaped statement.
	for _, sql := range []string{
		"", "   ", "SELECT 1", withMetadata, "/* only a comment */", "SELECT 'unterminated",
		"SELECT \xff\xfe\x00 FROM t", strings.Repeat("SELECT * FROM t WHERE id IN (1,2,3); ", 50),
	} {
		for _, dbms := range []sqllexer.DBMSType{"", sqllexer.DBMSPostgres, sqllexer.DBMSMySQL, sqllexer.DBMSOracle} {
			wantSQL, wantMetadata, err := processor.ObfuscateAndNormalize(sql, dbms)
			if err != nil {
				t.Fatal(err)
			}
			if err := processor.ObfuscateAndNormalizeInto(sql, dbms, &borrowed); err != nil {
				t.Fatal(err)
			}
			if borrowed.SQL != wantSQL || borrowed.Size != wantMetadata.Size {
				t.Fatalf("borrowed mismatch for %q: %q/%d vs %q/%d", sql, borrowed.SQL, borrowed.Size, wantSQL, wantMetadata.Size)
			}
			for _, field := range []struct {
				name      string
				got, want []string
			}{
				{"tables", borrowed.Tables, wantMetadata.Tables},
				{"comments", borrowed.Comments, wantMetadata.Comments},
				{"commands", borrowed.Commands, wantMetadata.Commands},
				{"procedures", borrowed.Procedures, wantMetadata.Procedures},
			} {
				if strings.Join(field.got, "\x00") != strings.Join(field.want, "\x00") {
					t.Fatalf("borrowed %s mismatch for %q: %q vs %q", field.name, sql, field.got, field.want)
				}
			}
		}
	}
}

// A reused Borrowed must not leak the previous call's values into a call that
// produced fewer of them.
func TestBorrowedIsFullyResetBetweenCalls(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	var borrowed rustffi.Borrowed
	if err := processor.ObfuscateAndNormalizeInto(withMetadata, "", &borrowed); err != nil {
		t.Fatal(err)
	}
	if len(borrowed.Tables) != 2 || len(borrowed.Comments) != 1 {
		t.Fatalf("unexpected metadata for the seed statement: %+v", borrowed)
	}
	if err := processor.ObfuscateAndNormalizeInto("SELECT 1", "", &borrowed); err != nil {
		t.Fatal(err)
	}
	if len(borrowed.Tables) != 0 || len(borrowed.Comments) != 0 || len(borrowed.Procedures) != 0 {
		t.Fatalf("stale metadata survived: %+v", borrowed)
	}
	if borrowed.SQL != "SELECT ?" {
		t.Fatalf("stale sql survived: %q", borrowed.SQL)
	}
}

func TestBorrowedRejectsNilAndClosedHandles(t *testing.T) {
	processor := rustffi.NewProcessor(0, 0)
	if err := processor.ObfuscateAndNormalizeInto("SELECT 1", "", nil); err == nil {
		t.Fatal("expected an error for a nil destination")
	}
	processor.Close()
	if err := processor.ObfuscateAndNormalizeInto("SELECT 1", "", &rustffi.Borrowed{}); err == nil {
		t.Fatal("expected an error from a closed processor")
	}
	if _, _, err := processor.ObfuscateAndNormalizeSize("SELECT 1", ""); err == nil {
		t.Fatal("expected an error from a closed processor")
	}
}

func TestSizeOnlyMatchesTheFullResult(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	for _, sql := range []string{"", "SELECT 1", withMetadata, "/* c */ CALL my_proc(1)"} {
		wantSQL, wantMetadata, err := processor.ObfuscateAndNormalize(sql, "")
		if err != nil {
			t.Fatal(err)
		}
		gotSQL, gotSize, err := processor.ObfuscateAndNormalizeSize(sql, "")
		if err != nil {
			t.Fatal(err)
		}
		if gotSQL != wantSQL || gotSize != wantMetadata.Size {
			t.Fatalf("size-only mismatch for %q: %q/%d vs %q/%d", sql, gotSQL, gotSize, wantSQL, wantMetadata.Size)
		}
	}
}

// Token values are substrings of the caller's input wherever the lexer did not
// have to rewrite them, so they own their memory as much as the input does; a
// value the lexer materialized would be copied instead. Neither may be
// disturbed by later calls.
func TestTokenValuesSurviveSubsequentCalls(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	sql := "SELECT 'a''b', c FROM t"
	tokens, err := processor.Tokenize(sql, "")
	if err != nil {
		t.Fatal(err)
	}
	before := make([]rustffi.Token, len(tokens))
	copy(before, tokens)

	for i := 0; i < 8; i++ {
		if _, err := processor.Tokenize(strings.Repeat("SELECT 'x''y' FROM other; ", 200), ""); err != nil {
			t.Fatal(err)
		}
		if _, _, err := processor.ObfuscateAndNormalize("SELECT * FROM churn", ""); err != nil {
			t.Fatal(err)
		}
	}
	for i := range tokens {
		if tokens[i] != before[i] {
			t.Fatalf("token %d was corrupted: %+v vs %+v", i, tokens[i], before[i])
		}
	}

	// Borrowed-from-input values must be windows onto the caller's string, not
	// copies: that is what keeps tokenize allocation-free in the common case.
	base := dataPointer(sql)
	aliased := 0
	for _, token := range tokens {
		if at := dataPointer(token.Value); at >= base && at < base+uintptr(len(sql)) {
			aliased++
		}
	}
	// The raw lexer never rewrites a value — only the obfuscator and normalizer
	// do — so on this path every value is a span of the input.
	if aliased != len(tokens) {
		t.Fatalf("expected every token to alias the input; %d of %d did", aliased, len(tokens))
	}
}

// Allocation counts are the deliverable, so they are asserted rather than only
// measured. The numbers are the ones ../reports/ZERO-COPY.md explains.
func TestAllocationsPerCall(t *testing.T) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()

	var borrowed rustffi.Borrowed
	for _, tc := range []struct {
		name string
		max  float64
		call func()
	}{
		// The bytes, the string headers, and the StatementMetadata.
		{"ObfuscateAndNormalize", 3, func() {
			if _, _, err := processor.ObfuscateAndNormalize(withMetadata, ""); err != nil {
				t.Fatal(err)
			}
		}},
		// Just the copy of the normalized SQL.
		{"ObfuscateAndNormalizeSize", 1, func() {
			if _, _, err := processor.ObfuscateAndNormalizeSize(withMetadata, ""); err != nil {
				t.Fatal(err)
			}
		}},
		// Nothing at all, once the scratch slice has been grown.
		{"ObfuscateAndNormalizeInto", 0, func() {
			if err := processor.ObfuscateAndNormalizeInto(withMetadata, "", &borrowed); err != nil {
				t.Fatal(err)
			}
		}},
		// Only the []Token; the values are substrings of the input.
		{"Tokenize", 1, func() {
			if _, err := processor.Tokenize(withMetadata, ""); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		tc.call() // warm the scratch buffers on both sides
		if got := testing.AllocsPerRun(200, tc.call); got > tc.max {
			t.Errorf("%s allocated %.2f times per call, want at most %.0f", tc.name, got, tc.max)
		}
	}
}

func BenchmarkObfuscateAndNormalize(b *testing.B) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := processor.ObfuscateAndNormalize(withMetadata, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObfuscateAndNormalizeInto(b *testing.B) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()
	var borrowed rustffi.Borrowed
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := processor.ObfuscateAndNormalizeInto(withMetadata, "", &borrowed); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObfuscateAndNormalizeSize(b *testing.B) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := processor.ObfuscateAndNormalizeSize(withMetadata, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTokenize(b *testing.B) {
	processor := rustffi.NewProcessor(fixtureObfuscatorFlags, fixtureNormalizerFlags)
	defer processor.Close()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := processor.Tokenize(withMetadata, ""); err != nil {
			b.Fatal(err)
		}
	}
}
