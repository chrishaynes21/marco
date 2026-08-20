package marcoexec

import (
	"fmt"
	"strings"
)

// Marco's string-literal rules, from internal/lexer/lexer.go readString.
//
// The lexer accepts EXACTLY five escapes:
//
//	\"   \'   \\   \n   \t
//
// Anything else is `invalid escape \c`. There is no \x, no \u, no \r. Bytes that are
// not a quote, a backslash or a newline are written through verbatim, which is why
// UTF-8 needs no escaping at all — and why it must not be escaped.
//
// This matters because strconv.Quote, the obvious choice, is WRONG here. It emits
// é for é, \x00 for a NUL and \r for a carriage return, and Marco rejects all
// three. A Director that typed any non-ASCII text — a name with an accent, a Japanese
// message, an emoji — would have generated a program the lexer refused. Round-tripped
// against the real lexer in encode_test.go rather than argued from the source.

// ErrUnencodable reports text Marco's string literals cannot carry.
type ErrUnencodable struct {
	// Offset is the byte position of the offending character.
	Offset int
	// Why explains what cannot be represented.
	Why string
}

func (e *ErrUnencodable) Error() string {
	return fmt.Sprintf("marcoexec: the text cannot be written as a Marco string literal at byte %d: %s",
		e.Offset, e.Why)
}

// Quote renders a Go string as a Marco string literal, including the quotes.
//
// It fails rather than approximating. A literal that lexed back to something different
// from what was asked for is the worst possible outcome here: the Director would type,
// paste or set a value the user never asked for, verify it against its own wrong
// expectation, and report success.
func Quote(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			// A NUL has no escape and no honest raw form: it terminates the string in
			// most of the toolchain between here and the target application, so a
			// literal carrying one would not survive intact.
			return "", &ErrUnencodable{Offset: i, Why: "a NUL character cannot be represented"}
		default:
			// Everything else verbatim, which is what readString does with it. That
			// includes every byte of a UTF-8 sequence: é, 日本 and 🎉 are ordinary
			// bytes to the lexer, and escaping them would produce invalid source.
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}

// MustQuote is Quote for text known to be safe — act names, capability names, the
// fixed strings this package writes itself. It panics on failure, which is correct
// for a programming error and never reachable from user text.
func MustQuote(s string) string {
	q, err := Quote(s)
	if err != nil {
		panic(err)
	}
	return q
}
