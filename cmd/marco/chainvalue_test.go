package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A STEP OF A CHAIN IS READ AS A LINE IS. Said live: "discord mute then notepad type with hello"
// ran both, and "discord mute then notepad type hello" went to Director and failed, because a
// chain's step understood `with` and `name: value` and not the words after a routine's name that
// the same line alone is read with. Words after a name are its value only when the routine takes
// exactly one; with two they could land on either, so that chain still belongs to Director.
//
// Reading the steps with ParseInvocation again, or dropping the one-value check, must fail this.
func TestAChainStepTakesItsValueAsALineDoes(t *testing.T) {
	d := intakeWorld(t)
	for rt, src := range map[routes.Route]string{
		{App: "discord", Focus: true, Slug: "discord-mute"}:  intakeSrc,
		{App: "notepad", Focus: true, Slug: "notepad-type"}:  "script main...\n  do nothing.\n// {{text}}\n",
		{App: "notepad", Focus: true, Slug: "notepad-write"}: "script main...\n  do nothing.\n// {{title}} {{body}}\n",
	} {
		if err := d.Reg.Save(rt, src); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct {
		phrase string
		want   []string // the value each step was read with, "" for none
	}{
		{"discord mute then notepad type hello", []string{"", "hello"}},
		{"discord mute then notepad type hello there", []string{"", "hello there"}},
		{"discord mute then notepad type with hello", []string{"", "hello"}},
		{"discord mute then notepad type text: hello", []string{"", "hello"}},
	} {
		steps, ok := grammar(d, c.phrase, "")
		if !ok || len(steps) != len(c.want) {
			t.Errorf("%q: read as %d step(s), ok=%v", c.phrase, len(steps), ok)
			continue
		}
		for i, s := range steps {
			got := ""
			if len(s.positional) > 0 {
				got = s.positional[0]
			}
			if v, ok := s.named["text"]; ok {
				got = v
			}
			if got != c.want[i] {
				t.Errorf("%q step %d: value %q, want %q", c.phrase, i+1, got, c.want[i])
			}
		}
	}

	// TWO VALUES AND A LINE OF WORDS: not guessed at.
	if steps, ok := grammar(d, "discord mute then notepad write hello there", ""); ok {
		t.Errorf("words after a play taking two values were read as one: %+v", steps)
	}
}
