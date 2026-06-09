package routes

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in    string
		route string
		args  []string
	}{
		{"say hello", "say hello", nil},
		{"say hello with hello, world", "say hello", []string{"hello", "world"}},
		{"search with funny cats", "search", []string{"funny cats"}}, // one arg (no comma)
		{"login with chris , hunter2", "login", []string{"chris", "hunter2"}},
		{"WITH leading", "WITH leading", nil}, // " with " needs surrounding spaces
	}
	for _, c := range cases {
		route, args := SplitArgs(c.in)
		if route != c.route || !reflect.DeepEqual(args, c.args) {
			t.Errorf("SplitArgs(%q) = (%q, %v), want (%q, %v)", c.in, route, args, c.route, c.args)
		}
	}
}

func TestApplyArgs(t *testing.T) {
	// Named args fill {{name}}.
	named := map[string]string{"greeting": "hi", "who": "sam"}
	if got := ApplyArgs(`"{{greeting}} {{who}}"`, named, nil); got != `"hi sam"` {
		t.Errorf("named: got %q", got)
	}
	// Positional args fill {{N}}.
	src := `do OS's Type with "{{1}}".` + "\n" + `do OS's Type with "{{2}}".`
	got := ApplyArgs(src, nil, []string{"hello", "world"})
	want := `do OS's Type with "hello".` + "\n" + `do OS's Type with "world".`
	if got != want {
		t.Fatalf("positional: got %q, want %q", got, want)
	}
	// Missing arg → empty.
	if got := ApplyArgs(`"{{1}}-{{2}}"`, nil, []string{"a"}); got != `"a-"` {
		t.Errorf("missing arg: got %q", got)
	}
	// Values are escaped for the surrounding string literal.
	if got := ApplyArgs(`"{{name}}"`, map[string]string{"name": `he said "hi"\`}, nil); got != `"he said \"hi\"\\"` {
		t.Errorf("escaping: got %q", got)
	}
	// No placeholders → unchanged.
	if got := ApplyArgs(`do OS's Key with "enter".`, nil, []string{"x"}); got != `do OS's Key with "enter".` {
		t.Errorf("no-placeholder: got %q", got)
	}
}

func TestArgNames(t *testing.T) {
	src := `do OS's Type with "{{username}}".
do OS's Secret with "login-to-facebook:password".
do OS's Secret with "global-pin".`
	got := ArgNames(src)
	// {{username}} + the route-qualified secret arg "password"; the global secret
	// (no ":") is pre-set, not an invocation arg, so it's skipped.
	want := []string{"username", "password"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArgNames = %v, want %v", got, want)
	}
}

func TestParseInvocation(t *testing.T) {
	route, named, pos := ParseInvocation("say hello name:chris")
	if route != "say hello" || named["name"] != "chris" || pos != nil {
		t.Errorf("named: route=%q named=%v pos=%v", route, named, pos)
	}
	route, named, _ = ParseInvocation("dm person:sam message:hi there")
	if route != "dm" || named["person"] != "sam" || named["message"] != "hi there" {
		t.Errorf("multi-word value: route=%q named=%v", route, named)
	}
	route, named, pos = ParseInvocation("say hello with chris")
	if route != "say hello" || named != nil || len(pos) != 1 || pos[0] != "chris" {
		t.Errorf("positional fallback: route=%q named=%v pos=%v", route, named, pos)
	}
}
