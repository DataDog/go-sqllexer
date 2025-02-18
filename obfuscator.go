package sqllexer

import (
	"strings"
)

type obfuscatorConfig struct {
	DollarQuotedFunc           bool `json:"dollar_quoted_func"`
	ReplaceDigits              bool `json:"replace_digits"`
	ReplacePositionalParameter bool `json:"replace_positional_parameter"`
	ReplaceBoolean             bool `json:"replace_boolean"`
	ReplaceNull                bool `json:"replace_null"`
	KeepJsonPath               bool `json:"keep_json_path"` // by default, we replace json path with placeholder
	ReplaceBindParameter       bool `json:"replace_bind_parameter"`
}

type obfuscatorOption func(*obfuscatorConfig)

func WithReplaceDigits(replaceDigits bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplaceDigits = replaceDigits
	}
}

func WithReplacePositionalParameter(replacePositionalParameter bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplacePositionalParameter = replacePositionalParameter
	}
}

func WithReplaceBoolean(replaceBoolean bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplaceBoolean = replaceBoolean
	}
}

func WithReplaceNull(replaceNull bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplaceNull = replaceNull
	}
}

func WithDollarQuotedFunc(dollarQuotedFunc bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.DollarQuotedFunc = dollarQuotedFunc
	}
}

func WithKeepJsonPath(keepJsonPath bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.KeepJsonPath = keepJsonPath
	}
}

func WithReplaceBindParameter(replaceBindParameter bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplaceBindParameter = replaceBindParameter
	}
}

type Obfuscator struct {
	config *obfuscatorConfig
}

func NewObfuscator(opts ...obfuscatorOption) *Obfuscator {
	obfuscator := &Obfuscator{
		config: &obfuscatorConfig{},
	}

	for _, opt := range opts {
		opt(obfuscator.config)
	}

	return obfuscator
}

const (
	StringPlaceholder = "?"
	NumberPlaceholder = "?"
)

// Obfuscate takes an input SQL string and returns an obfuscated SQL string.
// The obfuscator replaces all literal values with a single placeholder
func (o *Obfuscator) Obfuscate(input string, lexerOpts ...lexerOption) string {
	var obfuscatedSQL = new(strings.Builder)
	obfuscatedSQL.Grow(len(input))

	lexer := New(
		input,
		lexerOpts...,
	)

	for {
		token := lexer.Scan()
		if token.Type == EOF {
			break
		}
		o.ObfuscateTokenValue(token, lexerOpts...)
		obfuscatedSQL.WriteString(token.String())
	}

	return strings.TrimSpace(obfuscatedSQL.String())
}

func (o *Obfuscator) ObfuscateTokenValue(token *Token, lexerOpts ...lexerOption) {
	switch token.Type {
	case NUMBER:
		if o.config.KeepJsonPath && token.PreviousValueToken.Type == JSON_OP {
			break
		}
		token.OutputValue = NumberPlaceholder
	case DOLLAR_QUOTED_FUNCTION:
		if o.config.DollarQuotedFunc {
			// obfuscate the content of dollar quoted function
			quotedFunc := (*token.Source)[token.Start+6 : token.End-6] // remove the $func$ prefix and suffix
			var obfuscatedDollarQuotedFunc strings.Builder
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			obfuscatedDollarQuotedFunc.WriteString(o.Obfuscate(quotedFunc, lexerOpts...))
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			token.OutputValue = obfuscatedDollarQuotedFunc.String()
			break
		}
		token.OutputValue = StringPlaceholder
	case STRING, INCOMPLETE_STRING, DOLLAR_QUOTED_STRING:
		if o.config.KeepJsonPath && token.PreviousValueToken.Type == JSON_OP {
			break
		}
		token.OutputValue = StringPlaceholder
	case POSITIONAL_PARAMETER:
		if o.config.ReplacePositionalParameter {
			token.OutputValue = StringPlaceholder
		}
	case BIND_PARAMETER:
		if o.config.ReplaceBindParameter {
			token.OutputValue = StringPlaceholder
		}
	case BOOLEAN:
		if o.config.ReplaceBoolean {
			token.OutputValue = StringPlaceholder
		}
	case NULL:
		if o.config.ReplaceNull {
			token.OutputValue = StringPlaceholder
		}
	case IDENT, QUOTED_IDENT:
		if o.config.ReplaceDigits {
			token.OutputValue = replaceDigits(token, NumberPlaceholder)
		}
	}
}
