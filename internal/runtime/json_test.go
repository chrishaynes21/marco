package runtime

import (
	"reflect"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	set := NewSet()
	set.Put("X", Number(10))
	set.Put("Name", Text("e"))
	set.Put("On", Bool(true))

	cases := []struct {
		name string
		v    Value
	}{
		{"absent", Absent()},
		{"text", Text("hello")},
		{"number", Number(42)},
		{"bool", Bool(true)},
		{"set", SetVal(set)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValueFromJSON(JSONFromValue(tc.v))
			if JSONFromValue(got) == nil && !tc.v.IsAbsent() {
				t.Fatalf("round-trip lost value %v", tc.v)
			}
			// Compare via re-encoding (Values aren't directly comparable).
			if !reflect.DeepEqual(JSONFromValue(got), JSONFromValue(tc.v)) {
				t.Fatalf("round-trip mismatch: got %#v want %#v",
					JSONFromValue(got), JSONFromValue(tc.v))
			}
		})
	}
}

func TestJSONErrorEncoding(t *testing.T) {
	v := ErrVal(&Err{Message: "boom"})
	j := JSONFromValue(v)
	back := ValueFromJSON(j)
	if !back.IsError() || back.AsError().Message != "boom" {
		t.Fatalf("error round-trip failed: %#v", back)
	}
}

func TestJSONListEncoding(t *testing.T) {
	l := NewList()
	l.Append(Text("a"))
	l.Append(Number(2))
	j := JSONFromValue(ListVal(l))
	arr, ok := j.([]any)
	if !ok || len(arr) != 2 || arr[0] != "a" || arr[1] != float64(2) {
		t.Fatalf("list encoding wrong: %#v", j)
	}
}
