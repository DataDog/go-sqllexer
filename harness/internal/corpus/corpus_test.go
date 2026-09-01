package corpus

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.jsonl")
	want := []protocol.Request{
		{ID: "a", SQL: protocol.Text("SELECT 1"), Mode: protocol.ModeObfuscateAndNormalize},
		{ID: "b", SQL: protocol.Text("SELECT\n'multi\tline'"), DBMS: "postgresql", Mode: protocol.ModeNormalize,
			Normalizer: &protocol.NormalizerConfig{CollectTables: true}},
		{ID: "c", SQL: protocol.Text(strings.Repeat("SELECT * FROM t; ", 100_000)), Mode: protocol.ModeObfuscate},
	}

	w, err := NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range want {
		if err := w.Write(req); err != nil {
			t.Fatal(err)
		}
	}
	if w.Count() != len(want) {
		t.Fatalf("Count() = %d, want %d", w.Count(), len(want))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d requests, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].SQL != want[i].SQL || got[i].DBMS != want[i].DBMS || got[i].Mode != want[i].Mode {
			t.Errorf("request %d round-tripped as %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[1].Normalizer == nil || !got[1].Normalizer.CollectTables {
		t.Errorf("normalizer config lost in round trip: %+v", got[1].Normalizer)
	}
}

func TestReadFromSkipsBlankLines(t *testing.T) {
	input := "\n{\"id\":\"a\",\"sql\":\"SELECT 1\"}\n\n{\"id\":\"b\",\"sql\":\"SELECT 2\"}\n"
	got, err := ReadFrom(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d requests, want 2", len(got))
	}
}

func TestReadFromReportsLineNumber(t *testing.T) {
	input := "{\"id\":\"a\"}\n{not json}\n"
	_, err := ReadFrom(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not identify the offending line", err)
	}
}
