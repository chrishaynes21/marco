package macroir

import (
	"sort"
	"strconv"
	"strings"
)

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
	// THE REST OF A FULL KEYBOARD, which games bind (ADR-335): the numpad apart from the top row,
	// its operators spelt out because `+` joins a chord, and the lock, pause, print-screen and
	// menu keys. F13-F24 are in init.
	"numstar": true, "numplus": true, "numminus": true, "numdot": true, "numslash": true,
	"numenter": true, "numlock": true, "scrolllock": true, "pause": true, "printscreen": true,
	"menu": true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyNames[string(c)] = true
	}
	for c := '0'; c <= '9'; c++ {
		keyNames[string(c)] = true
		keyNames["num"+string(c)] = true
	}
	for n := 1; n <= 24; n++ {
		keyNames[fName(n)] = true
	}
}

func fName(n int) string { return "f" + strconv.Itoa(n) }

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

// modifierRank is the order a combo names its modifiers in: ctrl, alt, shift, win. It is the
// order the recorder writes a chord in, so a combo somebody types and the same keys pressed come
// out as the same text.
var modifierRank = map[string]int{"ctrl": 1, "control": 1, "alt": 2, "shift": 3,
	"win": 4, "lwin": 4, "rwin": 4}

// IsModifierKey says whether a key name is Ctrl, Alt, Shift or Win.
func IsModifierKey(name string) bool { return modifierRank[name] != 0 }

// keyAliases are the second names a key answers to, each to the name the recorder writes, so a
// combo typed with one is the combo the keys make when they are pressed.
var keyAliases = map[string]string{"control": "ctrl", "return": "enter", "escape": "esc",
	"bs": "backspace", "del": "delete", "ins": "insert", "pgup": "pageup", "pgdn": "pagedown",
	"lwin": "win", "rwin": "win", "backtick": "`"}

// CanonicalChord is a combo as Marco spells it: lower case, no spaces, each key by the name the recorder writes (`control` as `ctrl`), the
// modifiers first in the order ctrl, alt, shift, win, and the other keys after them in the order
// written. "Shift + Ctrl + 1" is `ctrl+shift+1`. It does not check the names; IsChord does.
func CanonicalChord(spec string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if alias, ok := keyAliases[p]; ok {
			p = alias
		}
		parts[i] = p
	}
	sort.SliceStable(parts, func(i, j int) bool {
		ri, rj := modifierRank[parts[i]], modifierRank[parts[j]]
		if ri == 0 {
			ri = 99
		}
		if rj == 0 {
			rj = 99
		}
		return ri < rj
	})
	return strings.Join(parts, "+")
}
