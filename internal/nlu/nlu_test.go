package nlu

import "testing"

var routes = []string{"login-to-facebook", "start-sea-of-thieves", "open-chest", "farm-wood"}

func TestExactNormalized(t *testing.T) {
	m := Resolve("login to facebook", routes)
	if !m.Exact || m.Route != "login-to-facebook" {
		t.Fatalf("got %+v", m)
	}
}

func TestParaphraseFuzzy(t *testing.T) {
	m := Resolve("log into facebook", routes)
	if m.Route != "login-to-facebook" || m.Exact || m.Score < 0.6 {
		t.Fatalf("got %+v, want fuzzy login-to-facebook >=0.6", m)
	}
}

func TestStartGame(t *testing.T) {
	m := Resolve("start sea of thieves", routes)
	if !m.Exact || m.Route != "start-sea-of-thieves" {
		t.Fatalf("got %+v", m)
	}
}

func TestUnrelatedLowScore(t *testing.T) {
	m := Resolve("make me a sandwich", routes)
	if m.Score >= 0.6 {
		t.Fatalf("unrelated input scored too high: %+v", m)
	}
}

func TestPartialTokens(t *testing.T) {
	m := Resolve("chest", routes)
	if m.Route != "open-chest" {
		t.Fatalf("got %+v, want open-chest", m)
	}
}
