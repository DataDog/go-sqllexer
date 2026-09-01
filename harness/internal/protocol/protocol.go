// Package protocol defines the cross-language wire format used by the migration
// harness. Every implementation under validation (Go today, Rust later) speaks the
// same newline-delimited JSON protocol on stdin/stdout, which lets the differential
// driver compare implementations without embedding either of them.
//
// The field names of ObfuscatorConfig and NormalizerConfig intentionally match the
// JSON tags of the library's internal config structs, so the corpus can be generated
// directly from testdata/**.json without translation.
package protocol

import (
	"encoding/base64"
	"encoding/json"
	"unicode/utf8"
)

// Mode selects which part of the library a request exercises.
type Mode string

const (
	ModeTokenize              Mode = "tokenize"
	ModeObfuscate             Mode = "obfuscate"
	ModeNormalize             Mode = "normalize"
	ModeObfuscateAndNormalize Mode = "obfuscate_and_normalize"
)

// Modes lists every supported mode.
var Modes = []Mode{ModeTokenize, ModeObfuscate, ModeNormalize, ModeObfuscateAndNormalize}

// ObfuscatorConfig mirrors the library's obfuscator options.
type ObfuscatorConfig struct {
	DollarQuotedFunc           bool `json:"dollar_quoted_func"`
	ReplaceDigits              bool `json:"replace_digits"`
	ReplacePositionalParameter bool `json:"replace_positional_parameter"`
	ReplaceBoolean             bool `json:"replace_boolean"`
	ReplaceNull                bool `json:"replace_null"`
	KeepJsonPath               bool `json:"keep_json_path"`
	ReplaceBindParameter       bool `json:"replace_bind_parameter"`
}

// NormalizerConfig mirrors the library's normalizer options.
type NormalizerConfig struct {
	CollectTables                 bool `json:"collect_tables"`
	CollectCommands               bool `json:"collect_commands"`
	CollectComments               bool `json:"collect_comments"`
	CollectProcedure              bool `json:"collect_procedure"`
	KeepSQLAlias                  bool `json:"keep_sql_alias"`
	UppercaseKeywords             bool `json:"uppercase_keywords"`
	RemoveSpaceBetweenParentheses bool `json:"remove_space_between_parentheses"`
	KeepTrailingSemicolon         bool `json:"keep_trailing_semicolon"`
	KeepIdentifierQuotation       bool `json:"keep_identifier_quotation"`
}

// DefaultObfuscatorConfig matches the defaults used by the library's test suite.
func DefaultObfuscatorConfig() ObfuscatorConfig {
	return ObfuscatorConfig{
		ReplaceDigits:  true,
		ReplaceBoolean: true,
		ReplaceNull:    true,
	}
}

// DefaultNormalizerConfig matches the defaults used by the library's test suite.
func DefaultNormalizerConfig() NormalizerConfig {
	return NormalizerConfig{
		CollectTables:   true,
		CollectCommands: true,
		CollectComments: true,
	}
}

// FixtureObfuscatorConfig is the configuration the testdata suite applies to a
// fixture output that does not carry an explicit obfuscator_config. It differs from
// DefaultObfuscatorConfig, so corpora generated from testdata must record it
// explicitly rather than relying on either implementation's notion of a default.
func FixtureObfuscatorConfig() ObfuscatorConfig {
	return ObfuscatorConfig{
		DollarQuotedFunc:           true,
		ReplaceDigits:              true,
		ReplacePositionalParameter: true,
		ReplaceBoolean:             true,
		ReplaceNull:                true,
	}
}

// FixtureNormalizerConfig is the testdata suite's implicit normalizer configuration.
func FixtureNormalizerConfig() NormalizerConfig {
	return NormalizerConfig{
		CollectTables:    true,
		CollectCommands:  true,
		CollectComments:  true,
		CollectProcedure: true,
	}
}

// Text is a string that survives JSON encoding even when it is not valid UTF-8.
//
// This matters because the library must accept arbitrary bytes: encoding/json (and
// every other conformant JSON encoder) replaces invalid UTF-8 with U+FFFD, which
// would silently rewrite the very inputs the fuzz corpus exists to exercise. Valid
// UTF-8 is encoded as a plain JSON string so corpora stay readable; anything else is
// encoded as {"b64": "..."}. Implementations in other languages must do the same.
type Text string

func (t Text) String() string { return string(t) }

// MarshalJSON implements json.Marshaler.
func (t Text) MarshalJSON() ([]byte, error) {
	if utf8.ValidString(string(t)) {
		return json.Marshal(string(t))
	}
	return json.Marshal(struct {
		B64 string `json:"b64"`
	}{base64.StdEncoding.EncodeToString([]byte(t))})
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Text) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = Text(s)
		return nil
	}
	var wrapped struct {
		B64 string `json:"b64"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(wrapped.B64)
	if err != nil {
		return err
	}
	*t = Text(raw)
	return nil
}

// Request is one unit of work: a single SQL statement plus the configuration to
// apply to it. Requests are streamed as one JSON object per line.
type Request struct {
	ID         string            `json:"id"`
	SQL        Text              `json:"sql"`
	DBMS       string            `json:"dbms,omitempty"`
	Mode       Mode              `json:"mode"`
	Obfuscator *ObfuscatorConfig `json:"obfuscator,omitempty"`
	Normalizer *NormalizerConfig `json:"normalizer,omitempty"`
}

// Metadata mirrors sqllexer.StatementMetadata. Order is significant: it is part of
// the compatibility contract, so the differ compares slices element by element.
type Metadata struct {
	Size       int    `json:"size"`
	Tables     []Text `json:"tables"`
	Comments   []Text `json:"comments"`
	Commands   []Text `json:"commands"`
	Procedures []Text `json:"procedures"`
}

// Token is one lexer token, used by ModeTokenize.
type Token struct {
	Type  int  `json:"type"`
	Value Text `json:"value"`
}

// Response is the result for a single Request, echoed back with the same ID.
type Response struct {
	ID       string    `json:"id"`
	Output   Text      `json:"output,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
	Tokens   []Token   `json:"tokens,omitempty"`
	Error    string    `json:"error,omitempty"`
}
