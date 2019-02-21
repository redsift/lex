package lex

import (
	"fmt"
	"unicode/utf8"
)

// TokenType identifies the type of lex items.
// One can extend known tokens by starting from iota + NoopToken
type TokenType int

const (
	_ TokenType = iota
	// TokEOF used to define end of input
	TokEOF
	// TokError used to define error tokens; value of the token should be text of error
	TokError
	// NoopToken should only be used as base value for iota
	NoopToken
)

const (
	eof = -1
)

// Pos represents token position in the input
type Pos int

// Token represents a token returned from the scanner.
type Token struct {
	Typ TokenType // Type
	Pos Pos       // The starting position, in bytes, of this Token in the input string
	Val string    // Value
}

func (i Token) String() string {
	switch i.Typ {
	case NoopToken:
		panic("NoopToken is not valid token type; should be used as base value for iota")
	case TokEOF:
		return "EOF"
	case TokError:
		return i.Val
	}
	if len(i.Val) > 10 {
		return fmt.Sprintf("%.10q…", i.Val)
	}
	return fmt.Sprintf("%q", i.Val)
}

// StateFn represents the state of the scanner
// as a function that returns the Next state
type StateFn func(*Lexer) StateFn

// Lexer holds the state of the scanner.
type Lexer struct {
	name  string     // used only for error reports
	input string     // the string being scanned
	start Pos        // start position of this Token
	pos   Pos        // current position in the input
	width Pos        // width of last rune read from input
	items chan Token // channel of scanned items
}

// New creates a new *Lexer that will scan given input starting from state
func New(input string, state StateFn) *Lexer {
	l := &Lexer{
		input: input,
		items: make(chan Token),
	}
	go l.run(state)
	return l
}

// run scans the input by executing state functions until
// the state is nil
func (l *Lexer) run(start StateFn) {
	for state := start; state != nil; {
		state = state(l)
	}
	close(l.items) // No more tokens will be delivered
}

// Next returns the next rune in the input
func (l *Lexer) Next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
	return r
}

// Peek returns but does not consume the next rune in the input
func (l *Lexer) Peek() rune {
	r := l.Next()
	l.Backup()
	return r
}

// Backup steps back one rune. Can only be called once per call of Next
func (l *Lexer) Backup() {
	l.pos -= l.width
}

// Emit passes an Token back to the client
func (l *Lexer) Emit(t TokenType) {
	l.items <- Token{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

// Ignore skips over the pending input before this point
func (l *Lexer) Ignore() {
	l.start = l.pos
}

// Errorf emits an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.NextToken
func (l *Lexer) Errorf(format string, args ...interface{}) StateFn {
	l.items <- Token{TokError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// NextToken returns the next token from the input.
// Called by the parser, not in the lexing goroutine
func (l *Lexer) NextToken() Token {
	return <-l.items
}

// IgnoreRunes ignore all runes for which skip return true
func (l *Lexer) IgnoreRunes(skip func(rune) bool) {
	for skip(l.Peek()) {
		l.Next()
	}
	l.Ignore()
}

func indexRune(l rune, set ...rune) int {
	for i, r := range set {
		if l == r {
			return i
		}
	}
	return -1
}

// Accept consumes the Next rune if it's from the valid set
func (l *Lexer) Accept(set ...rune) bool {
	if indexRune(l.Next(), set...) < 0 {
		l.Backup()
		return false
	}
	return true
}

// AcceptRun consumes a run of runes from the valid set.
// It returns false when no runes were consumed
func (l *Lexer) AcceptRun(set ...rune) bool {
	accepted := false
	for indexRune(l.Next(), set...) >= 0 {
		accepted = true
	}
	l.Backup()
	return accepted
}

// AcceptUntil consumes a run of any runes except given.
// It returns false when no runes were consumed
func (l *Lexer) AcceptUntil(set ...rune) bool {
	accepted := false
	for {
		r := l.Next()
		if indexRune(r, set...) >= 0 || r == eof {
			l.Backup()
			return accepted
		}
		accepted = true
	}
}

// Drain drains the output so the lexing goroutine will exit.
// Called by the parser, not in the lexing goroutine
func (l *Lexer) Drain() {
	for range l.items {
	}
}

// EOF emits TokEOF and returns nil
func EOF(l *Lexer) StateFn {
	l.Emit(TokEOF)
	return nil
}
