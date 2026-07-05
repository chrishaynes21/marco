package voiceteach

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Cmd
	}{
		{"click this", Cmd{Kind: Click}},
		{"Click here.", Cmd{Kind: Click}},
		{"tap that", Cmd{Kind: Click}},
		{"anchor this", Cmd{Kind: Anchor}},
		{"remember this", Cmd{Kind: Anchor}},
		{"click the text Mute", Cmd{Kind: ClickText, Text: "Mute"}},
		{"click the word Deafen", Cmd{Kind: ClickText, Text: "Deafen"}},
		{"click text Start Game", Cmd{Kind: ClickText, Text: "Start Game"}},
		{"wait for this screen", Cmd{Kind: WaitScreen}},
		{"wait for it to load", Cmd{Kind: None}}, // not a recognized exact phrase
		{"type Hello World", Cmd{Kind: Type, Text: "Hello World"}},
		{"say hi there", Cmd{Kind: Type, Text: "hi there"}},
		{"type arg 1", Cmd{Kind: Type, Text: "{{1}}"}},
		{"type argument 2", Cmd{Kind: Type, Text: "{{2}}"}},
		{"type the first argument", Cmd{Kind: Type, Text: "{{1}}"}},
		{"type second argument", Cmd{Kind: Type, Text: "{{2}}"}},
		{"press enter", Cmd{Kind: Key, Key: "enter"}},
		{"hit the escape key", Cmd{Kind: Key, Key: "esc"}},
		{"press return", Cmd{Kind: Key, Key: "enter"}},
		// Chords: a spoken or punctuated combo folds to "mod+key".
		{"press control c", Cmd{Kind: Key, Key: "ctrl+c"}},
		{"press ctrl+c", Cmd{Kind: Key, Key: "ctrl+c"}},
		{"press control shift escape", Cmd{Kind: Key, Key: "ctrl+shift+esc"}},
		{"press alt f4", Cmd{Kind: Key, Key: "alt+f4"}},
		// Menu navigation: bare directions, arrow synonyms, and nav keys.
		{"down", Cmd{Kind: Key, Key: "down"}},
		{"up arrow", Cmd{Kind: Key, Key: "up"}},
		{"arrow left", Cmd{Kind: Key, Key: "left"}},
		{"press the down arrow", Cmd{Kind: Key, Key: "down"}},
		{"press down key", Cmd{Kind: Key, Key: "down"}},
		{"tab", Cmd{Kind: Key, Key: "tab"}},
		{"escape", Cmd{Kind: Key, Key: "esc"}},
		{"select", Cmd{Kind: Key, Key: "enter"}},
		{"page down", Cmd{Kind: Key, Key: "pagedown"}},
		{"switch to notepad", Cmd{Kind: Activate, App: "notepad"}},
		{"open Chrome", Cmd{Kind: Activate, App: "chrome"}},
		{"wait 2 seconds", Cmd{Kind: Wait, Ms: 2000}},
		{"wait 500 ms", Cmd{Kind: Wait, Ms: 500}},
		{"pause 1.5s", Cmd{Kind: Wait, Ms: 1500}},
		{"wait a second", Cmd{Kind: Wait, Ms: 1000}},
		{"undo", Cmd{Kind: Undo}},
		{"scratch that", Cmd{Kind: Undo}},
		{"done", Cmd{Kind: Done}},
		{"that's it", Cmd{Kind: Done}},
		{"cancel", Cmd{Kind: Cancel}},
		{"", Cmd{Kind: None}},
		{"do a barrel roll", Cmd{Kind: None}},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestNormalizeChord(t *testing.T) {
	cases := []struct{ in, want string }{
		{"enter", "enter"},
		{"page up", "pageup"},   // multi-word single key, no modifier
		{"escape key", "esc"},   // " key" suffix stripped
		{"control c", "ctrl+c"}, // spoken chord
		{"ctrl+c", "ctrl+c"},    // punctuated chord
		{"ctrl + c", "ctrl+c"},  // spaced punctuation
		{"control shift escape", "ctrl+shift+esc"},
		{"control alt delete", "ctrl+alt+delete"},
		{"alt f4", "alt+f4"},
		{"command s", "win+s"},          // synonym → win
		{"control control c", "ctrl+c"}, // duplicate modifier collapses
		{"control", "ctrl"},             // bare modifier presses on its own
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := NormalizeChord(c.in); got != c.want {
			t.Errorf("NormalizeChord(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in string
		ms int
		ok bool
	}{
		{"2", 2000, true},
		{"2 seconds", 2000, true},
		{"500 ms", 500, true},
		{"500 milliseconds", 500, true},
		{"1.5s", 1500, true},
		{"1 minute", 60000, true},
		{"half a second", 500, true},
		{"forever", 0, false},
	}
	for _, c := range cases {
		ms, ok := parseDuration(c.in)
		if ms != c.ms || ok != c.ok {
			t.Errorf("parseDuration(%q) = (%d,%v), want (%d,%v)", c.in, ms, ok, c.ms, c.ok)
		}
	}
}
