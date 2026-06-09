package recorder

import "testing"

func keyEv(name string, down bool) RecordedEvent {
	return RecordedEvent{Kind: EvKey, KeyName: name, Down: down}
}

func TestStopKeySingle(t *testing.T) {
	sk := ParseStopKey("esc")
	if sk.Triggered(keyEv("a", true)) {
		t.Fatal("unrelated key should not trigger")
	}
	if !sk.Triggered(keyEv("esc", true)) {
		t.Fatal("esc down should trigger a single-key stop")
	}
}

func TestStopKeyDefault(t *testing.T) {
	sk := ParseStopKey("")
	if sk.Label() != "F12" {
		t.Fatalf("default label = %q, want F12", sk.Label())
	}
	if sk.Triggered(keyEv("esc", true)) {
		t.Fatal("esc must NOT trigger the default (F12) stop — it stays a recordable key")
	}
	if !sk.Triggered(keyEv("f12", true)) {
		t.Fatal("f12 should trigger the default stop")
	}
}

func TestStopKeyChord(t *testing.T) {
	sk := ParseStopKey("ctrl+f12")
	if sk.Label() != "Ctrl+F12" {
		t.Fatalf("label = %q, want Ctrl+F12", sk.Label())
	}
	if sk.Triggered(keyEv("ctrl", true)) {
		t.Fatal("ctrl alone should not trigger")
	}
	if !sk.Triggered(keyEv("f12", true)) {
		t.Fatal("ctrl held + f12 down should trigger")
	}
	// Releasing one key drops the chord again.
	sk.Triggered(keyEv("f12", false))
	if sk.Triggered(keyEv("ctrl", true)) {
		t.Fatal("chord should not be active after a key was released")
	}
}

func TestStopKeyHas(t *testing.T) {
	sk := ParseStopKey("space+esc")
	if !sk.Has("ESC") || !sk.Has("space") {
		t.Fatal("Has should match (case-insensitively) the chord keys")
	}
	if sk.Has("a") {
		t.Fatal("Has should not match unrelated keys")
	}
}
