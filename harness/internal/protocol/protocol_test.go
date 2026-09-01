package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

// The wire format has to carry arbitrary bytes intact: a corpus entry that is
// silently rewritten to U+FFFD is an input neither implementation ever sees, which
// would quietly hide exactly the differences fuzzing is meant to find.
func TestTextRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantPlain bool
	}{
		{"ascii", "SELECT 1", true},
		{"empty", "", true},
		{"multibyte", "SELECT * FROM \"таблица\" -- 日本語", true},
		{"control chars", "SELECT\t1\n-- x\r\n", true},
		{"invalid utf8", "SELECT '\xff\xfe\x00'", false},
		{"lone surrogate bytes", "\xed\xa0\x80", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(Text(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			if isPlain := strings.HasPrefix(string(encoded), `"`); isPlain != tt.wantPlain {
				t.Errorf("encoded as %s, wantPlain=%v", encoded, tt.wantPlain)
			}

			var got Text
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.value {
				t.Errorf("round trip changed the bytes: %q -> %q", tt.value, string(got))
			}
		})
	}
}

func TestRequestRoundTripPreservesInvalidUTF8(t *testing.T) {
	want := Request{
		ID:   "x",
		SQL:  Text("SELECT '\xff\xfe'"),
		DBMS: "postgresql",
		Mode: ModeObfuscateAndNormalize,
	}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Request
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got.SQL != want.SQL {
		t.Errorf("sql = %q, want %q", string(got.SQL), string(want.SQL))
	}
}

// The fixture defaults are not the library defaults; conflating them silently
// changes what a corpus means.
func TestFixtureDefaultsDifferFromLibraryDefaults(t *testing.T) {
	if FixtureObfuscatorConfig() == DefaultObfuscatorConfig() {
		t.Error("fixture and library obfuscator defaults must stay distinct")
	}
	if FixtureNormalizerConfig() == DefaultNormalizerConfig() {
		t.Error("fixture and library normalizer defaults must stay distinct")
	}
}
