//go:build windows

package secrets

import "testing"

// TestRoundTrip exercises the real Windows Credential Manager: set, get, list,
// delete. Cleans up after itself.
func TestRoundTrip(t *testing.T) {
	s := New()
	const name = "marco-test-secret-xyz"
	defer s.Delete(name)

	if err := s.Set(name, "hunter2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, found, err := s.Get(name)
	if err != nil || !found || v != "hunter2" {
		t.Fatalf("Get = (%q, %v, %v), want (hunter2, true, nil)", v, found, err)
	}

	names, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	seen := false
	for _, n := range names {
		if n == name {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("List did not include %q: %v", name, names)
	}

	if err := s.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, found, _ := s.Get(name); found {
		t.Fatalf("secret still present after delete")
	}
}

func TestGetMissing(t *testing.T) {
	if v, found, err := New().Get("definitely-not-a-real-marco-secret"); found || err != nil || v != "" {
		t.Fatalf("missing Get = (%q, %v, %v), want (\"\", false, nil)", v, found, err)
	}
}
