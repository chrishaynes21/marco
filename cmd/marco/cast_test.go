package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
)

// The CAST table on the Advanced page, driven through the real handler with the roster faked at
// its seam. Nothing here discovers a bridge, launches one, or touches the desktop.

func readCast(t *testing.T, r theaterReport) (string, []castRow, string) {
	t.Helper()
	old := castReport
	castReport = func() theaterReport { return r }
	defer func() { castReport = old }()

	w := httptest.NewRecorder()
	handleCast(w, httptest.NewRequest(http.MethodGet, "/api/cast", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/cast = %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Provider string    `json:"provider"`
		Cast     []castRow `json:"cast"`
		Why      string    `json:"why"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode /api/cast: %v (%s)", err, w.Body.String())
	}
	return got.Provider, got.Cast, got.Why
}

// THE CAST CARRIES ALL FIVE COLUMNS, AND THE REASON IS ONE OF THEM.
//
// "accessibility: cannot act" tells somebody their machine is broken and nothing about what to do.
// The Actor asks its provider and keeps the sentence that came back — usually the operating
// system's own refusal, naming the binary it could not run — and this surface is the last place
// that sentence could be dropped before it reaches the person who has to fix it.
func TestTheCastSaysWhoCanActAndWhyNot(t *testing.T) {
	provider, rows, why := readCast(t, theaterReport{
		Bridge: `C:\marco\uia-bridge.exe`,
		Roster: []theaterhost.Player{
			{Name: "accessibility", Provider: "uia-bridge", Path: `C:\marco\uia-bridge.exe`, Available: true},
			{Name: "vision", Provider: "onnx", Available: false, Reason: "the model file is missing"},
		},
	})
	if provider == "" {
		t.Error("the provider that was found is not reported; 'no bridge' and 'a bridge somewhere unexpected' look identical without it")
	}
	if why != "" {
		t.Errorf("why = %q with a roster present; the empty state is being shown over a table", why)
	}
	if len(rows) != 2 {
		t.Fatalf("cast = %+v, want both actors", rows)
	}
	if rows[0].Actor != "accessibility" || !rows[0].Available || rows[0].Why != "" {
		t.Errorf("row = %+v, want the ready actor with no reason", rows[0])
	}
	if rows[1].Available {
		t.Errorf("row = %+v, want the unavailable actor reported as unavailable rather than dropped", rows[1])
	}
	if rows[1].Why != "the model file is missing" {
		t.Errorf("row.Why = %q — the sentence that says what to fix was dropped", rows[1].Why)
	}
	if rows[0].Where == "" || rows[0].Provider == "" {
		t.Errorf("row = %+v, want the provider and where it lives", rows[0])
	}
}

// AN ABSENT CAST IS AN ABSENT CAST.
//
// The one thing this surface must never do is invent a row. "Marco could not ask" and "nobody is
// available" are different claims, and only one of them is true when the roster is empty.
//
// Mutation: drop the empty-roster arm and let the page render a bare table. This goes red.
func TestTheCastRendersAnHonestEmptyState(t *testing.T) {
	provider, rows, why := readCast(t, theaterReport{})
	if len(rows) != 0 {
		t.Fatalf("cast = %+v, want no rows at all", rows)
	}
	if provider != "" {
		t.Errorf("provider = %q with nothing found", provider)
	}
	if why == "" {
		t.Fatal("an empty cast with no explanation — a person is shown a blank and told nothing")
	}
	if !strings.Contains(why, "could not ask") {
		t.Errorf("why = %q; it should say Marco could not ask, not that nobody is available", why)
	}
	// And the page renders that sentence rather than an empty table.
	if !strings.Contains(editPage, "r.why||'Marco could not ask.'") {
		t.Error("the Advanced page does not render the empty state the endpoint sends")
	}
}

// IT IS CALLED CAST, NOT ACTORS.
//
// `director learn --actor` already means the play's SUBJECT, and `actor` is a Marco keyword with a
// third meaning again. A table headed "Actors" beside those would add a confusion rather than
// remove one. The COLUMN is still Actor, singular, because each row is one.
func TestTheCastIsCalledCast(t *testing.T) {
	i := strings.Index(editPage, `<section id="view-advanced"`)
	if i < 0 {
		t.Fatal("there is no Advanced page")
	}
	adv := editPage[i:]
	adv = adv[:strings.Index(adv, "</section>")]
	if !strings.Contains(adv, "CAST") {
		t.Error("the Advanced page does not head the table Cast")
	}
	if strings.Contains(adv, "<h2>Actors") || strings.Contains(adv, ">ACTORS<") {
		t.Error("the table is headed Actors, which already means the play's subject elsewhere")
	}
	if !strings.Contains(editPage, "<th>Actor</th>") {
		t.Error("the table has no Actor column")
	}
	for _, col := range []string{"<th>Provider</th>", "<th>Where</th>", "<th>Available</th>", "<th>Why not</th>"} {
		if !strings.Contains(editPage, col) {
			t.Errorf("the Cast table is missing %s", col)
		}
	}
}

// THE CAST IS THE SAME ACCOUNT `marco director diagnose` PRINTS.
//
// Not a second roster assembled here. The Roster asks each Actor the same availability question
// casting itself asks, and a surface that worked it out its own way would agree with the product
// right up until the moment somebody needed it to disagree.
func TestTheCastReadsTheOneRoster(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/cast.go")
	if !strings.Contains(src, "var castReport = theaterDiagnostics") {
		t.Error("the Cast surface no longer reads the roster the diagnose command reads")
	}
	if strings.Contains(src, "newTheaterHost(") || strings.Contains(src, "Availability(") {
		t.Error("the Cast surface builds its own opinion of who can act")
	}
}
