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
	for token := range lexer.ScanAllTokens() {
		switch token.Type {
		case NUMBER:
			obfuscatedSQL.WriteString(NumberPlaceholder)
		case STRING:
			obfuscatedSQL.WriteString(StringPlaceholder)
		case INCOMPLETE_STRING:
			obfuscatedSQL.WriteString(StringPlaceholder)
		case IDENT:
			if o.config.ReplaceDigits {
				// regex to replace digits in identifier
				// we try to avoid using regex as much as possible,
				// as regex isn't the most performant,
				// but it's the easiest to implement and maintain
				obfuscatedSQL.WriteString(digitsRegex.ReplaceAllString(token.Value, "?"))
			} else {
				obfuscatedSQL.WriteString(token.Value)
			}
		case COMMENT:
			obfuscatedSQL.WriteString(token.Value)
		case MULTILINE_COMMENT:
			obfuscatedSQL.WriteString(token.Value)
		case DOLLAR_QUOTED_STRING:
			obfuscatedSQL.WriteString(StringPlaceholder)
		case DOLLAR_QUOTED_FUNCTION:
			if o.config.DollarQuotedFunc {
				// obfuscate the content of dollar quoted function
				quotedFunc := strings.TrimPrefix(token.Value, "$func$")
				quotedFunc = strings.TrimSuffix(quotedFunc, "$func$")
				obfuscatedSQL.WriteString("$func$")
				obfuscatedSQL.WriteString(o.Obfuscate(quotedFunc, lexerOpts...))
				obfuscatedSQL.WriteString("$func$")
			} else {
				// treat dollar quoted function as dollar quoted string
				obfuscatedSQL.WriteString(StringPlaceholder)
			}
		case ERROR | UNKNOWN:
			// if we encounter an error or unknown token, we just append the value
			obfuscatedSQL.WriteString(token.Value)
		default:
			obfuscatedSQL.WriteString(token.Value)
		}
	}

	return strings.TrimSpace(obfuscatedSQL.String())
}
