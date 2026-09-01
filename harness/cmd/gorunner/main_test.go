package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// TestHandleMatchesTestdata proves the runner is a faithful oracle: replaying the
// repository's own fixtures through the protocol must reproduce their expected
// output exactly. If this passes and the differ reports no mismatches against a
// candidate implementation, the candidate satisfies the same fixtures — without
// those fixtures or the existing tests being touched.
func TestHandleMatchesTestdata(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata")

	type testcase struct {
		Input   string `json:"input"`
		Outputs []struct {
			Expected          protocol.Text              `json:"expected"`
			ObfuscatorConfig  *protocol.ObfuscatorConfig `json:"obfuscator_config"`
			NormalizerConfig  *protocol.NormalizerConfig `json:"normalizer_config"`
			StatementMetadata *protocol.Metadata         `json:"statement_metadata"`
		} `json:"outputs"`
	}

	cases := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tc testcase
		if err := json.Unmarshal(raw, &tc); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		dbms := strings.Split(filepath.ToSlash(rel), "/")[0]

		for i, out := range tc.Outputs {
			cases++
			obfuscator := protocol.FixtureObfuscatorConfig()
			if out.ObfuscatorConfig != nil {
				obfuscator = *out.ObfuscatorConfig
			}
			normalizer := protocol.FixtureNormalizerConfig()
			if out.NormalizerConfig != nil {
				normalizer = *out.NormalizerConfig
			}

			resp := handle(protocol.Request{
				ID:         filepath.ToSlash(rel),
				SQL:        protocol.Text(tc.Input),
				DBMS:       dbms,
				Mode:       protocol.ModeObfuscateAndNormalize,
				Obfuscator: &obfuscator,
				Normalizer: &normalizer,
			})
			if resp.Error != "" {
				t.Errorf("%s#%d: unexpected error %q", rel, i, resp.Error)
				continue
			}
			if resp.Output != out.Expected {
				t.Errorf("%s#%d:\n got: %q\nwant: %q", rel, i, resp.Output, out.Expected)
			}
			if out.StatementMetadata != nil {
				assertMetadata(t, rel, i, out.StatementMetadata, resp.Metadata)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if cases == 0 {
		t.Fatal("no testdata cases were exercised")
	}
	t.Logf("verified %d testdata outputs", cases)
}

// assertMetadata compares element by element: ordering and deduplication are part
// of the compatibility contract the fixtures encode.
func assertMetadata(t *testing.T, file string, i int, want, got *protocol.Metadata) {
	t.Helper()
	if got == nil {
		t.Errorf("%s#%d: no metadata returned", file, i)
		return
	}
	if want.Size != got.Size {
		t.Errorf("%s#%d: metadata size = %d, want %d", file, i, got.Size, want.Size)
	}
	for _, f := range []struct {
		name string
		a, b []protocol.Text
	}{
		{"tables", want.Tables, got.Tables},
		{"comments", want.Comments, got.Comments},
		{"commands", want.Commands, got.Commands},
		{"procedures", want.Procedures, got.Procedures},
	} {
		if !slices.Equal(f.a, f.b) {
			t.Errorf("%s#%d: metadata %s = %v, want %v", file, i, f.name, f.b, f.a)
		}
	}
}

func TestHandleModes(t *testing.T) {
	const sql = "SELECT * FROM users WHERE id = 42 /* comment */"

	tokenized := handle(protocol.Request{ID: "t", SQL: protocol.Text(sql), Mode: protocol.ModeTokenize})
	if len(tokenized.Tokens) == 0 {
		t.Error("tokenize produced no tokens")
	}
	if tokenized.Output != "" || tokenized.Metadata != nil {
		t.Error("tokenize should not produce output or metadata")
	}

	obfuscated := handle(protocol.Request{ID: "o", SQL: protocol.Text(sql), Mode: protocol.ModeObfuscate})
	if strings.Contains(string(obfuscated.Output), "42") {
		t.Errorf("obfuscate left a literal in place: %q", obfuscated.Output)
	}

	normalized := handle(protocol.Request{ID: "n", SQL: protocol.Text(sql), Mode: protocol.ModeNormalize})
	if normalized.Metadata == nil || len(normalized.Metadata.Tables) != 1 || normalized.Metadata.Tables[0] != "users" {
		t.Errorf("normalize metadata = %+v, want tables [users]", normalized.Metadata)
	}

	both := handle(protocol.Request{ID: "b", SQL: protocol.Text(sql), Mode: protocol.ModeObfuscateAndNormalize})
	if both.Metadata == nil || strings.Contains(string(both.Output), "42") {
		t.Errorf("obfuscate_and_normalize = %+v", both)
	}

	unknown := handle(protocol.Request{ID: "u", SQL: protocol.Text(sql), Mode: "nope"})
	if unknown.Error == "" {
		t.Error("unknown mode should report an error")
	}
}

// Defaults are part of the protocol: a request that omits a config must behave
// identically on every implementation.
func TestHandleAppliesDefaultConfigs(t *testing.T) {
	explicit := protocol.DefaultObfuscatorConfig()
	normalizer := protocol.DefaultNormalizerConfig()
	const sql = "SELECT id1 FROM users WHERE active = true"

	withDefaults := handle(protocol.Request{ID: "a", SQL: protocol.Text(sql), Mode: protocol.ModeObfuscateAndNormalize})
	withExplicit := handle(protocol.Request{ID: "b", SQL: protocol.Text(sql), Mode: protocol.ModeObfuscateAndNormalize,
		Obfuscator: &explicit, Normalizer: &normalizer})

	if withDefaults.Output != withExplicit.Output {
		t.Errorf("default config differs from explicit default:\n %q\n %q", withDefaults.Output, withExplicit.Output)
	}
}
