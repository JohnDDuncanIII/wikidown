package main

import (
	"fmt"
)

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// https://www.youtube.com/watch?v=HxaD_trXwRE
// https://talks.golang.org/2011/lex.slide#1

func main() {
	fmt.Println("test")
}

type lexer struct {
	name  string    // used only for error reports.
	input string    // the string being scanned.
	start int       // start position of this item.
	pos   int       // current position in the input.
	width int       // width of last rune read.
	items chan item // channel of scanned items.
}

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

func lex(name, input string) (*lexer, chan item) {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item, 2),
	}

	go l.run() // Concurrently run state machine.
	return l, l.items
}

// run lexes the input by executing state functions untill
// the state is nil.
func (l *lexer) run() {
	for state := initState; state != nil; {
		state = state(l)
	}
	close(l.items) // No more tokens will be delivered to parser
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	// send an item constructed from the type
	// and lobs the item to the parser
	l.items <- item{t, l.input[l.start:l.pos]}
	// move the input
	l.start = l.pos
}

// lexText scans until an opening action delimiter, "{{".
func lexText(l *lexer) stateFn {
	l.width = 0
	if x := strings.Index(l.input[l.pos:], l.leftDelim); x >= 0 {
		ldn := Pos(len(l.leftDelim))
		l.pos += Pos(x)
		trimLength := Pos(0)
		if strings.HasPrefix(l.input[l.pos+ldn:], leftTrimMarker) {
			trimLength = rightTrimLength(l.input[l.start:l.pos])
		}
		l.pos -= trimLength
		if l.pos > l.start {
			l.emit(itemText)
		}
		l.pos += trimLength
		l.ignore()
		return lexLeftDelim
	} else {
		l.pos = Pos(len(l.input))
	}
	// Correctly reached EOF.
	if l.pos > l.start {
		l.emit(itemText)
	}
	l.emit(itemEOF)
	return nil
}

// lexLeftMeta
// lexLeftDelim scans the left delimiter, which is known to be present, possibly with a trim marker.
func lexLeftDelim(l *lexer) stateFn {
	l.pos += Pos(len(l.leftDelim))
	trimSpace := strings.HasPrefix(l.input[l.pos:], leftTrimMarker)
	afterMarker := Pos(0)
	if trimSpace {
		afterMarker = trimMarkerLen
	}
	if strings.HasPrefix(l.input[l.pos+afterMarker:], leftComment) {
		l.pos += afterMarker
		l.ignore()
		return lexComment
	}
	l.emit(itemLeftDelim)
	l.pos += afterMarker
	l.ignore()
	l.parenDepth = 0
	return lexInsideAction
}

// lexInsideAction scans the elements inside action delimiters.
func lexInsideAction(l *lexer) stateFn {
	// Either number, quoted string, or identifier.
	// Spaces separate arguments; runs of spaces turn into itemSpace.
	// Pipe symbols separate and are emitted.
	delim, _ := l.atRightDelim()
	if delim {
		if l.parenDepth == 0 {
			return lexRightDelim
		}
		return l.errorf("unclosed left paren")
	}
	switch r := l.next(); {
	case r == eof || isEndOfLine(r):
		return l.errorf("unclosed action")
	case isSpace(r):
		return lexSpace
	case r == '=':
		l.emit(itemAssign)
	case r == ':':
		if l.next() != '=' {
			return l.errorf("expected :=")
		}
		l.emit(itemDeclare)
	case r == '|':
		l.emit(itemPipe)
	case r == '"':
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '$':
		return lexVariable
	case r == '\'':
		return lexChar
	case r == '.':
		// special look-ahead for ".field" so we don't break l.backup().
		if l.pos < Pos(len(l.input)) {
			r := l.input[l.pos]
			if r < '0' || '9' < r {
				return lexField
			}
		}
		fallthrough // '.' can start a number.
	case r == '+' || r == '-' || ('0' <= r && r <= '9'):
		l.backup()
		return lexNumber
	case isAlphaNumeric(r):
		l.backup()
		return lexIdentifier
	case r == '(':
		l.emit(itemLeftParen)
		l.parenDepth++
	case r == ')':
		l.emit(itemRightParen)
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right paren %#U", r)
		}
	case r <= unicode.MaxASCII && unicode.IsPrint(r):
		l.emit(itemChar)
		return lexInsideAction
	default:
		return l.errorf("unrecognized character in action: %#U", r)
	}
	return lexInsideAction
}
