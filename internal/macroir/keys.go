package macroir

import "strings"

// THE KEYS MARCO CAN PRESS, BY NAME.
//
// A key step carries a Marco key name (`Step.Key`), and three places have to agree about what
// those names are: the recorder that WRITES them, the host that PRESSES them, and anything that
// lets a person TYPE one, which since step editing can change a key is Studio. The first two
// are Windows tables (`recorder.vkNames`, `oshost.keyMap`); this is the cross-platform list a
// person's typing is checked against, so an edit can be refused while the person is still
// looking at it rather than at run time.
//
// ONE FACT, THREE LISTS, AND A GUARD. On Windows, `oshost` holds that every name here resolves
// and that it presses nothing that is not named here, and `recorder` holds that every name it
// writes is here. A name added to one table and not the others fails one of those tests.
var keyNames = map[string]bool{
	"enter": true, "return": true, "esc": true, "escape": true,
	"tab": true, "space": true, "backspace": true, "bs": true,
	"delete": true, "del": true, "insert": true, "ins": true,
	"home": true, "end": true,
	"pageup": true, "pgup": true, "pagedown": true, "pgdn": true,
	"up": true, "down": true, "left": true, "right": true,
	"shift": true, "ctrl": true, "control": true, "alt": true,
	"win": true, "lwin": true, "rwin": true,
	"capslock": true,
	"`":        true, "~": true, "backtick": true,
	"-": true, "=": true, "[": true, "]": true, "\\": true,
	";": true, "'": true, ",": true, ".": true, "/": true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyNames[string(c)] = true
	}
	for c := '0'; c <= '9'; c++ {
		keyNames[string(c)] = true
	}
	for n := 1; n <= 12; n++ {
		keyNames[fName(n)] = true
	}
}

func fName(n int) string {
	if n < 10 {
		return "f" + string(rune('0'+n))
	}
	return "f1" + string(rune('0'+n-10))
}

// KeyNames is every key name Marco can press, for a guard to compare a table against.
func KeyNames() []string {
	out := make([]string, 0, len(keyNames))
	for k := range keyNames {
		out = append(out, k)
	}
	return out
}

// IsKeyName says whether `name` is one key Marco can press, as `Step.Key` spells it: lowercased.
func IsKeyName(name string) bool { return keyNames[name] }

// IsChord says whether `spec` is keys Marco can press together, written `ctrl+shift+s`: one or
// more key names joined by `+`, each a name `IsKeyName` knows. A single key is a chord of one.
//
// `+` itself is not a key name — the `=` key is — so a chord never has an empty part.
func IsChord(spec string) bool {
	if spec == "" {
		return false
	}
	for _, part := range strings.Split(spec, "+") {
		if !keyNames[part] {
			return false
		}
	}
	return true
}
