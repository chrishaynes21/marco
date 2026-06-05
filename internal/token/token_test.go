package token

import "testing"

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{Illegal, "illegal"},
		{EOF, "eof"},
		{Word, "word"},
		{String, "string"},
		{Number, "number"},
		{Boolean, "boolean"},
		{Period, "."},
		{Bang, "!"},
		{Question, "?"},
		{Comma, ","},
		{Ellipsis, "..."},
		{Possessive, "'s"},
		{Plus, "+"},
		{Indent, "INDENT"},
		{Dedent, "DEDENT"},
		{Newline, "NL"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(c.k), got, c.want)
		}
	}
}

// TestKindStringExhaustive guards against a new Kind being added to the iota
// block without a matching String arm: every declared Kind must render to
// something other than the unknown fallback.
func TestKindStringExhaustive(t *testing.T) {
	for k := Illegal; k <= Newline; k++ {
		if got := k.String(); got == "" {
			t.Errorf("Kind(%d) renders empty", int(k))
		}
		// The fallback is "kind(%d)"; no real Kind should hit it.
		if got := k.String(); got == "kind("+itoa(int(k))+")" {
			t.Errorf("Kind(%d) hit the unknown-kind fallback; missing String arm", int(k))
		}
	}
}

func TestKindStringUnknownFallback(t *testing.T) {
	k := Kind(9999)
	if got := k.String(); got != "kind(9999)" {
		t.Errorf("unknown Kind = %q, want kind(9999)", got)
	}
}

func TestPosString(t *testing.T) {
	p := Pos{Line: 12, Col: 4}
	if got := p.String(); got != "12:4" {
		t.Errorf("Pos.String() = %q, want 12:4", got)
	}
}

func TestTokenString(t *testing.T) {
	// Valueless token: kind@pos.
	bang := Token{Kind: Bang, Pos: Pos{Line: 3, Col: 7}}
	if got := bang.String(); got != "!@3:7" {
		t.Errorf("valueless Token.String() = %q, want !@3:7", got)
	}
	// Value-bearing token: kind("value")@pos, with %q quoting.
	word := Token{Kind: Word, Value: "Save", Pos: Pos{Line: 1, Col: 1}}
	if got := word.String(); got != `word("Save")@1:1` {
		t.Errorf("Token.String() = %q, want word(\"Save\")@1:1", got)
	}
	// %q escapes embedded quotes.
	str := Token{Kind: String, Value: `a"b`, Pos: Pos{Line: 2, Col: 5}}
	if got := str.String(); got != `string("a\"b")@2:5` {
		t.Errorf("Token.String() = %q, want escaped quote", got)
	}
}

// itoa avoids importing strconv just for the exhaustiveness guard.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
