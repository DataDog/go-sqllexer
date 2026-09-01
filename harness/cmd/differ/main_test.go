package main

import (
	"testing"

	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

func metadata() *protocol.Metadata {
	return &protocol.Metadata{
		Size:     11,
		Tables:   []protocol.Text{"users", "orders"},
		Commands: []protocol.Text{"SELECT"},
	}
}

// The differ is the gate that decides whether the Rust implementation is
// compatible, so each dimension of the contract needs a case proving a difference
// there is actually caught.
func TestDiffResponse(t *testing.T) {
	base := protocol.Response{ID: "x", Output: "SELECT * FROM users", Metadata: metadata()}

	tests := []struct {
		name      string
		mutate    func(*protocol.Response)
		wantField string
		want      verdict
	}{
		{"identical", func(*protocol.Response) {}, "", verdictEqual},
		{"output", func(r *protocol.Response) { r.Output = "SELECT * FROM user" }, "output", verdictMismatch},
		{"error appears", func(r *protocol.Response) { r.Error = "boom" }, "error", verdictMismatch},
		{"metadata missing", func(r *protocol.Response) { r.Metadata = nil }, "metadata", verdictMismatch},
		{"metadata size", func(r *protocol.Response) { r.Metadata.Size = 12 }, "metadata.size", verdictMismatch},
		// Same deduplicated values in a different order: reported, not gating.
		{"table order", func(r *protocol.Response) {
			r.Metadata.Tables = []protocol.Text{"orders", "users"}
		}, "metadata.tables", verdictOrder},
		{"table dropped", func(r *protocol.Response) {
			r.Metadata.Tables = []protocol.Text{"users"}
		}, "metadata.tables", verdictMismatch},
		{"duplicate table", func(r *protocol.Response) {
			r.Metadata.Tables = []protocol.Text{"users", "users"}
		}, "metadata.tables", verdictMismatch},
		{"command differs", func(r *protocol.Response) {
			r.Metadata.Commands = []protocol.Text{"INSERT"}
		}, "metadata.commands", verdictMismatch},
		{"comment added", func(r *protocol.Response) {
			r.Metadata.Comments = []protocol.Text{"/* c */"}
		}, "metadata.comments", verdictMismatch},
		{"procedure added", func(r *protocol.Response) {
			r.Metadata.Procedures = []protocol.Text{"p"}
		}, "metadata.procedures", verdictMismatch},
		{"token count", func(r *protocol.Response) {
			r.Tokens = []protocol.Token{{Type: 1, Value: "a"}}
		}, "tokens", verdictMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := base
			candidate.Metadata = metadata()
			tt.mutate(&candidate)

			field, got := diffResponse(base, candidate)
			if got != tt.want {
				t.Fatalf("verdict = %v (field %q), want %v", got, field, tt.want)
			}
			if field != tt.wantField {
				t.Errorf("field = %q, want %q", field, tt.wantField)
			}
		})
	}
}

// An output difference must win over a metadata ordering difference: the run has to
// fail, not be downgraded to a warning.
func TestDiffResponseMismatchOutranksOrder(t *testing.T) {
	ref := protocol.Response{Output: "a", Metadata: metadata()}
	cand := protocol.Response{Output: "b", Metadata: &protocol.Metadata{
		Size: 11, Tables: []protocol.Text{"orders", "users"}, Commands: []protocol.Text{"SELECT"},
	}}

	if field, got := diffResponse(ref, cand); got != verdictMismatch || field != "output" {
		t.Fatalf("field = %q verdict = %v, want output mismatch", field, got)
	}
}

func TestDiffResponseTokenValue(t *testing.T) {
	ref := protocol.Response{Tokens: []protocol.Token{{Type: 6, Value: "users"}, {Type: 2, Value: " "}}}
	cand := protocol.Response{Tokens: []protocol.Token{{Type: 6, Value: "users"}, {Type: 3, Value: " "}}}

	field, got := diffResponse(ref, cand)
	if got != verdictMismatch || field != "tokens[1]" {
		t.Fatalf("field = %q verdict = %v, want tokens[1] mismatch", field, got)
	}
}

func TestRunMatchesResponsesByID(t *testing.T) {
	// A runner that emits responses out of order must still be compared correctly;
	// missing responses must fail loudly rather than silently comparing zero values.
	requests := []protocol.Request{{ID: "a", SQL: "SELECT 1"}, {ID: "b", SQL: "SELECT 2"}}

	got, err := run(`printf '{"id":"b","output":"two"}\n{"id":"a","output":"one"}\n'`, requests)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Output != "one" || got[1].Output != "two" {
		t.Fatalf("responses not realigned to request order: %+v", got)
	}

	if _, err := run(`printf '{"id":"a","output":"one"}\n'`, requests); err == nil {
		t.Fatal("expected an error when a response is missing")
	}
}
