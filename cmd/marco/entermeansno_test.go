package main

import (
	"os"
	"testing"
)

// F6: ENTER MEANS NO at every yes/no question the CLI asks through askYes: forgetting one routine
// or all of them, "Did you mean …?" before a simplify, and `marco dispatch`'s offers. An EOF (a
// script piping nothing) is no answer and is no as well.
//
// Taking Enter as yes again must fail this.
func TestEnterDeclinesEveryCLIConfirmation(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{{"\n", false}, {"", false}, {"n\n", false}, {"y\n", true}, {"Yes\n", true}} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		prev := os.Stdin
		os.Stdin = r
		_, _ = w.WriteString(c.in)
		w.Close()
		_, err = captureStdout(t, func() error {
			if got := askYes("Forget \"x\"? [y]es / [N]o: "); got != c.want {
				t.Errorf("%q answered %v, want %v", c.in, got, c.want)
			}
			return nil
		})
		os.Stdin = prev
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
