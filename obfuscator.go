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

	var lastValueToken *Token

	for {
		token := lexer.Scan()
		if token.Type == EOF {
			break
		}
		o.ObfuscateTokenValue(token, lastValueToken, lexerOpts...)
		obfuscatedSQL.WriteString(token.String())
		if isValueToken(token) {
			lastValueToken = token
		}
	}

	return strings.TrimSpace(obfuscatedSQL.String())
}

func (o *Obfuscator) ObfuscateTokenValue(token *Token, lastValueToken *Token, lexerOpts ...lexerOption) {
	switch token.Type {
	case NUMBER:
		if o.config.KeepJsonPath && lastValueToken.Type == JSON_OP {
			break
		}
		token.SetOutputValue(NumberPlaceholder)
	case DOLLAR_QUOTED_FUNCTION:
		if o.config.DollarQuotedFunc {
			// obfuscate the content of dollar quoted function
			quotedFunc := (*token.Source)[token.Start+6 : token.End-6] // remove the $func$ prefix and suffix
			var obfuscatedDollarQuotedFunc strings.Builder
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			obfuscatedDollarQuotedFunc.WriteString(o.Obfuscate(quotedFunc, lexerOpts...))
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			token.SetOutputValue(obfuscatedDollarQuotedFunc.String())
			break
		}
		token.SetOutputValue(StringPlaceholder)
	case STRING, INCOMPLETE_STRING, DOLLAR_QUOTED_STRING:
		if o.config.KeepJsonPath && lastValueToken.Type == JSON_OP {
			break
		}
		token.SetOutputValue(StringPlaceholder)
	case POSITIONAL_PARAMETER:
		if o.config.ReplacePositionalParameter {
			token.SetOutputValue(StringPlaceholder)
		}
	case BIND_PARAMETER:
		if o.config.ReplaceBindParameter {
			token.SetOutputValue(StringPlaceholder)
		}
	case BOOLEAN:
		if o.config.ReplaceBoolean {
			token.SetOutputValue(StringPlaceholder)
		}
	case NULL:
		if o.config.ReplaceNull {
			token.SetOutputValue(StringPlaceholder)
		}
	case IDENT, QUOTED_IDENT:
		if o.config.ReplaceDigits && token.ExtraInfo != nil && len(token.ExtraInfo.Digits) > 0 {
			token.SetOutputValue(replaceDigits(token, NumberPlaceholder))
		}
	}
}
