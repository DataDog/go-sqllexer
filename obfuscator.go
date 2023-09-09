package sqllexer

import (
	"strings"
)

type obfuscatorConfig struct {
	ReplaceDigits    bool
	DollarQuotedFunc bool
}

func WithReplaceDigits(replaceDigits bool) func(*obfuscatorConfig) {
	return func(c *obfuscatorConfig) {
		c.ReplaceDigits = replaceDigits
	}
}

func WithDollarQuotedFunc(dollarQuotedFunc bool) func(*obfuscatorConfig) {
	return func(c *obfuscatorConfig) {
		c.DollarQuotedFunc = dollarQuotedFunc
	}
}

type Obfuscator struct {
	config *obfuscatorConfig
}

func NewObfuscator(opts ...func(*obfuscatorConfig)) *Obfuscator {
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
func (o *Obfuscator) Obfuscate(input string) string {
	var obfuscatedSQL strings.Builder

	lexer := New(input)
	for token := range lexer.ScanAllTokens() {
		switch token.Type {
		case NUMBER:
			obfuscatedSQL.WriteString(NumberPlaceholder)
		case DOLLAR_QUOTED_FUNCTION:
			if o.config.DollarQuotedFunc {
				// obfuscate the content of dollar quoted function
				quotedFunc := token.Value[6 : len(token.Value)-6] // remove the $func$ prefix and suffix
				obfuscatedSQL.WriteString("$func$")
				obfuscatedSQL.WriteString(o.Obfuscate(quotedFunc))
				obfuscatedSQL.WriteString("$func$")
			} else {
				obfuscatedSQL.WriteString(StringPlaceholder)
			}
		case STRING, INCOMPLETE_STRING, DOLLAR_QUOTED_STRING:
			obfuscatedSQL.WriteString(StringPlaceholder)
		case IDENT:
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
