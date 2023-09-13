package sqllexer

import (
	"strings"
)

type obfuscatorConfig struct {
	ReplaceDigits    bool
	DollarQuotedFunc bool
}

type obfuscatorOption func(*obfuscatorConfig)

func WithReplaceDigits(replaceDigits bool) obfuscatorOption {
	return func(c *obfuscatorConfig) {
		c.ReplaceDigits = replaceDigits
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
		obfuscatedSQL.WriteString(o.ObfuscateTokenValue(token, lexerOpts...))
	}

	return strings.TrimSpace(obfuscatedSQL.String())
}

func (o *Obfuscator) ObfuscateTokenValue(token Token, lexerOpts ...lexerOption) string {
	switch token.Type {
	case NUMBER:
		return NumberPlaceholder
	case DOLLAR_QUOTED_FUNCTION:
		if o.config.DollarQuotedFunc {
			// obfuscate the content of dollar quoted function
			quotedFunc := token.Value[6 : len(token.Value)-6] // remove the $func$ prefix and suffix
			var obfuscatedDollarQuotedFunc strings.Builder
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			obfuscatedDollarQuotedFunc.WriteString(o.Obfuscate(quotedFunc, lexerOpts...))
			obfuscatedDollarQuotedFunc.WriteString("$func$")
			return obfuscatedDollarQuotedFunc.String()
		} else {
			return StringPlaceholder
		}
	case STRING, INCOMPLETE_STRING, DOLLAR_QUOTED_STRING:
		return StringPlaceholder
	case IDENT:
		if o.config.ReplaceDigits {
			return replaceDigits(token.Value, "?")
		} else {
			return token.Value
		}
	default:
		return token.Value
	}
}
