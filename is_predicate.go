package sqllexer

import "strings"

type isPredicateState uint8

const (
	noISPredicate isPredicateState = iota
	afterIS
	afterISNot
)

// isPredicateContext follows only the keyword sequence of a PostgreSQL IS
// predicate. Whitespace and comments do not interrupt the sequence.
type isPredicateContext struct {
	state isPredicateState
}

func (c *isPredicateContext) keep(token *Token) bool {
	if token.Type == SPACE || token.Type == COMMENT || token.Type == MULTILINE_COMMENT {
		return false
	}

	keyword := func(value string) bool {
		return token.Type == KEYWORD && strings.EqualFold(token.Value, value)
	}
	operand := token.Type == BOOLEAN || token.Type == NULL
	keep := false
	switch c.state {
	case afterIS:
		if keyword("NOT") {
			c.state = afterISNot
			return false
		}
		keep = operand
	case afterISNot:
		keep = operand
	}

	c.state = noISPredicate
	if keyword("IS") {
		c.state = afterIS
	}
	return keep
}
