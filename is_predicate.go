package sqllexer

import "strings"

type isPredicateState uint8

const (
	noISPredicate isPredicateState = iota
	afterIS
	afterISNot
	afterISDistinct
	afterISNotDistinct
	afterISDistinctFrom
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
		switch {
		case keyword("NOT"):
			c.state = afterISNot
			return false
		case keyword("DISTINCT"):
			c.state = afterISDistinct
			return false
		default:
			keep = operand
		}
	case afterISNot:
		if keyword("DISTINCT") {
			c.state = afterISNotDistinct
			return false
		}
		keep = operand
	case afterISDistinct, afterISNotDistinct:
		if keyword("FROM") {
			c.state = afterISDistinctFrom
			return false
		}
	case afterISDistinctFrom:
		keep = operand
	}

	c.state = noISPredicate
	if keyword("IS") {
		c.state = afterIS
	}
	return keep
}
