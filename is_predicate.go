package sqllexer

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
	switch token.Type {
	case SPACE, COMMENT, MULTILINE_COMMENT:
		return false
	case KEYWORD:
		// The lexer classifies SQL keywords from ASCII letters.
		value := token.Value
		if len(value) == 2 && value[0]|0x20 == 'i' && value[1]|0x20 == 's' {
			c.state = afterIS
			return false
		}
		if c.state == afterIS && len(value) == 3 && value[0]|0x20 == 'n' && value[1]|0x20 == 'o' && value[2]|0x20 == 't' {
			c.state = afterISNot
			return false
		}
	case BOOLEAN, NULL:
		keep := c.state == afterIS || c.state == afterISNot
		c.state = noISPredicate
		return keep
	}

	c.state = noISPredicate
	return false
}
