package sqllexer

import (
	"strings"
)

type obfuscatorConfig struct {
	DollarQuotedFunc           bool
	ReplaceDigits              bool
	ReplacePositionalParameter bool
	ReplaceBoolean             bool
	ReplaceNull                bool
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
	var obfuscatedSQL strings.Builder

	lexer := New(
		input,
		lexerOpts...,
	)
	for _, token := range lexer.ScanAll() {
		switch token.Type {
		case NUMBER:
			obfuscatedSQL.WriteString(NumberPlaceholder)
		case DOLLAR_QUOTED_FUNCTION:
			if o.config.DollarQuotedFunc {
				// obfuscate the content of dollar quoted function
				quotedFunc := token.Value[6 : len(token.Value)-6] // remove the $func$ prefix and suffix
				obfuscatedSQL.WriteString("$func$")
				obfuscatedSQL.WriteString(o.Obfuscate(quotedFunc, lexerOpts...))
				obfuscatedSQL.WriteString("$func$")
			} else {
				obfuscatedSQL.WriteString(StringPlaceholder)
			}
		case STRING, INCOMPLETE_STRING, DOLLAR_QUOTED_STRING:
			obfuscatedSQL.WriteString(StringPlaceholder)
		case POSITIONAL_PARAMETER:
			if o.config.ReplacePositionalParameter {
				obfuscatedSQL.WriteString(StringPlaceholder)
			} else {
				obfuscatedSQL.WriteString(token.Value)
			}
		case IDENT:
			if o.config.ReplaceBoolean && isBoolean(token.Value) {
				obfuscatedSQL.WriteString(StringPlaceholder)
				continue
			}
			if o.config.ReplaceNull && isNull(token.Value) {
				obfuscatedSQL.WriteString(StringPlaceholder)
				continue
			}

			if o.config.ReplaceDigits {
				obfuscatedSQL.WriteString(replaceDigits(token.Value, "?"))
			} else {
				obfuscatedSQL.WriteString(token.Value)
			}
		default:
			obfuscatedSQL.WriteString(token.Value)
		}
	}

	return strings.TrimSpace(obfuscatedSQL.String())
}
