package sqllexer

type TokenType int

const (
	ERROR TokenType = iota
	EOF
	WS                     // whitespace
	STRING                 // string literal
	INCOMPLETE_STRING      // illegal string literal so that we can obfuscate it, e.g. 'abc
	NUMBER                 // number literal
	IDENT                  // identifier
	OPERATOR               // operator
	WILDCARD               // wildcard *
	COMMENT                // comment
	MULTILINE_COMMENT      // multiline comment
	PUNCTUATION            // punctuation
	DOLLAR_QUOTED_FUNCTION // dollar quoted function
	DOLLAR_QUOTED_STRING   // dollar quoted string
	NUMBERED_PARAMETER     // numbered parameter
	UNKNOWN                // unknown token
)

// Token represents a SQL token with its type and value.
type Token struct {
	Type  TokenType
	Value string
}

// SQL Lexer inspired from Rob Pike's talk on Lexical Scanning in Go
type Lexer struct {
	src    string // the input src string
	cursor int    // the current position of the cursor
	start  int    // the start position of the current token
}

func New(input string) *Lexer {
	return &Lexer{src: input}
}

// ScanAll scans the entire input string and returns a slice of tokens.
func (s *Lexer) ScanAll() []Token {
	var tokens []Token
	for {
		token := s.Scan()
		if token.Type == EOF {
			// don't include EOF token in the result
			break
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// ScanAllTokens scans the entire input string and returns a channel of tokens.
// Use this if you want to process the tokens as they are scanned.
func (s *Lexer) ScanAllTokens() <-chan *Token {
	tokenCh := make(chan *Token)

	go func() {
		defer close(tokenCh)

		for {
			token := s.Scan()
			if token.Type == EOF {
				// don't include EOF token in the result
				break
			}
			tokenCh <- &token
		}
	}()

	return tokenCh
}

// Scan scans the next token and returns it.
func (s *Lexer) Scan() Token {
	ch := s.peek()
	switch {
	case isWhitespace(ch):
		return s.scanWhitespace()
	case isLetter(ch):
		return s.scanIdentifier()
	case isDoubleQuote(ch):
		return s.scanDoubleQuotedIdentifier()
	case isSingleQuote(ch):
		return s.scanString()
	case isSingleLineComment(ch, s.lookAhead(1)):
		return s.scanSingleLineComment()
	case isMultiLineComment(ch, s.lookAhead(1)):
		return s.scanMultiLineComment()
	case isLeadingSign(ch):
		// if the leading sign is followed by a digit, then it's a number
		// although this is not strictly true, it's good enough for our purposes
		nextCh := s.lookAhead(1)
		if isDigit(nextCh) || nextCh == '.' {
			return s.scanNumber(true)
		}
		return s.scanOperator()
	case isDigit(ch):
		return s.scanNumber(false)
	case isWildcard(ch):
		return s.scanWildcard()
	case ch == '$':
		if isDigit(s.lookAhead(1)) {
			// if the dollar sign is followed by a digit, then it's a numbered parameter
			return s.scanNumberedParameter()
		}
		return s.scanDollarQuotedString()
	case isOperator(ch):
		return s.scanOperator()
	case isPunctuation(ch):
		return s.scanPunctuation()
	case isEOF(ch):
		return Token{EOF, ""}
	default:
		return s.scanUnknown()
	}
}

// lookAhead returns the rune n positions ahead of the cursor.
func (s *Lexer) lookAhead(n int) rune {
	if s.cursor+n >= len(s.src) || s.cursor+n < 0 {
		return 0
	}
	return rune(s.src[s.cursor+n])
}

// peek returns the rune at the cursor position.
func (s *Lexer) peek() rune {
	return s.lookAhead(0)
}

// nextBy advances the cursor by n positions and returns the rune at the cursor position.
func (s *Lexer) nextBy(n int) rune {
	// advance the cursor by n and return the rune at the cursor position
	if s.cursor+n > len(s.src) {
		return 0
	}
	s.cursor += n
	if s.cursor == len(s.src) {
		return 0
	}
	return rune(s.src[s.cursor])
}

// next advances the cursor by 1 position and returns the rune at the cursor position.
func (s *Lexer) next() rune {
	return s.nextBy(1)
}

func (s *Lexer) matchAt(match []rune) bool {
	if s.cursor+len(match) > len(s.src) {
		return false
	}
	for i, ch := range match {
		if s.src[s.cursor+i] != byte(ch) {
			return false
		}
	}
	return true
}

func (s *Lexer) scanNumber(leadingSign bool) Token {
	s.start = s.cursor

	if leadingSign {
		s.next() // consume the leading sign
	}

	if s.peek() == '0' {
		nextCh := s.lookAhead(1)
		if nextCh == 'x' || nextCh == 'X' {
			return s.scanHexNumber()
		} else if nextCh >= '0' && nextCh <= '7' {
			return s.scanOctalNumber()
		}
	}

	return s.scanDecimalNumber()
}

func (s *Lexer) scanDecimalNumber() Token {
	ch := s.next()

	// scan digits
	for isDigit(ch) || ch == '.' || isExpontent(ch) {
		if isExpontent(ch) {
			ch = s.next()
			if isLeadingSign(ch) {
				ch = s.next()
			}
		} else {
			ch = s.next()
		}
	}
	return Token{NUMBER, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanHexNumber() Token {
	s.nextBy(2) // consume the leading 0x
	ch := s.peek()

	for isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F') {
		ch = s.next()
	}
	return Token{NUMBER, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanOctalNumber() Token {
	s.nextBy(2) // consume the leading 0 and number
	ch := s.peek()

	for '0' <= ch && ch <= '7' {
		ch = s.next()
	}
	return Token{NUMBER, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanString() Token {
	s.start = s.cursor
	ch := s.next() // consume the opening quote
	escaped := false

	for {
		if escaped {
			// encountered an escape character
			// reset the escaped flag and continue
			escaped = false
			ch = s.next()
			continue
		}

		if ch == '\\' {
			escaped = true
			ch = s.next()
			continue
		}

		if ch == '\'' {
			ch = s.next() // consume the closing quote
			return Token{STRING, s.src[s.start:s.cursor]}
		}

		if isEOF(ch) {
			// encountered EOF before closing quote
			// this usually happens when the string is truncated
			return Token{INCOMPLETE_STRING, s.src[s.start:s.cursor]}
		}
		ch = s.next()
	}
}

func (s *Lexer) scanIdentifier() Token {
	// NOTE: this func does not distinguish between SQL keywords and identifiers
	s.start = s.cursor
	ch := s.next()
	for isLetter(ch) || isDigit(ch) || ch == '.' || ch == '?' {
		ch = s.next()
	}
	// return the token as uppercase so that we can do case insensitive matching
	return Token{IDENT, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanDoubleQuotedIdentifier() Token {
	s.start = s.cursor
	ch := s.next() // consume the opening quote
	for {
		// encountered the closing quote
		// BUT if it's followed by .", then we should keep going
		// e.g. postgre "foo"."bar"
		if ch == '"' {
			if s.matchAt([]rune(`"."`)) {
				ch = s.nextBy(3) // consume the "."
				continue
			}
			break
		}
		if isEOF(ch) {
			return Token{ERROR, s.src[s.start:s.cursor]}
		}
		ch = s.next()
	}
	s.next() // consume the closing quote
	return Token{IDENT, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanWhitespace() Token {
	// scan whitespace, tab, newline, carriage return
	s.start = s.cursor
	ch := s.next()
	for isWhitespace(ch) {
		ch = s.next()
	}
	return Token{WS, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanOperator() Token {
	s.start = s.cursor
	ch := s.next()
	for isOperator(ch) {
		ch = s.next()
	}
	return Token{OPERATOR, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanWildcard() Token {
	s.start = s.cursor
	s.next()
	return Token{WILDCARD, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanSingleLineComment() Token {
	s.start = s.cursor
	ch := s.nextBy(2) // consume the opening dashes
	for ch != '\n' && !isEOF(ch) {
		ch = s.next()
	}
	return Token{COMMENT, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanMultiLineComment() Token {
	s.start = s.cursor
	ch := s.nextBy(2) // consume the opening slash and asterisk
	for {
		if ch == '*' && s.lookAhead(1) == '/' {
			ch = s.nextBy(2) // consume the closing asterisk and slash
			break
		}
		if isEOF(ch) {
			// encountered EOF before closing comment
			// this usually happens when the comment is truncated
			return Token{ERROR, s.src[s.start:s.cursor]}
		}
		ch = s.next()
	}
	return Token{MULTILINE_COMMENT, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanPunctuation() Token {
	s.start = s.cursor
	s.next()
	return Token{PUNCTUATION, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanDollarQuotedString() Token {
	s.start = s.cursor
	ch := s.next() // consume the dollar sign
	tagStart := s.cursor

	for s.cursor < len(s.src) && ch != '$' {
		ch = s.next()
	}
	ch = s.next()                       // consume the closing dollar sign of the tag
	tag := s.src[tagStart-1 : s.cursor] // include the opening and closing dollar sign e.g. $tag$

	for s.cursor < len(s.src) {
		if s.matchAt([]rune(tag)) {
			s.nextBy(len(tag)) // consume the closing tag
			if tag == "$func$" {
				return Token{DOLLAR_QUOTED_FUNCTION, s.src[s.start:s.cursor]}
			}
			return Token{DOLLAR_QUOTED_STRING, s.src[s.start:s.cursor]}
		}
		ch = s.next()
	}
	return Token{ERROR, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanNumberedParameter() Token {
	s.start = s.cursor
	ch := s.nextBy(2) // consume the dollar sign and the number
	for {
		if !isDigit(ch) {
			break
		}
		ch = s.next()
	}
	return Token{NUMBERED_PARAMETER, s.src[s.start:s.cursor]}
}

func (s *Lexer) scanUnknown() Token {
	// When we see an unknown token, we advance the cursor until we see something that looks like a token boundary.
	s.start = s.cursor
	s.next()
	return Token{UNKNOWN, s.src[s.start:s.cursor]}
}
