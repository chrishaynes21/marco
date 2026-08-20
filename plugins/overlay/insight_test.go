package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The payloads here mirror what `marco director perception --json` actually returned from
// a running service, including the shapes that are easy to get wrong: `label` is omitempty,
// so an anonymous observation has NO label key rather than an empty one, and `duration` is
// nanoseconds rather than a string.

func decode(t *testing.T, payload string) insight {
	t.Helper()
	var in insight
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return in
}

func joined(lines []string) string { return strings.Join(lines, "\n") }

func TestInsightNotRunning(t *testing.T) {
	got := joined(insightLines(decode(t, `{"running":false}`), &feed{}, false))
	if !strings.Contains(got, "not running") {
		t.Errorf("want a not-running line, got:\n%s", got)
	}
	if !strings.Contains(got, "director serve") {
		t.Errorf("a stopped Director should say how to start it, got:\n%s", got)
	}
}

// The engine being unreachable and the Director not running are different problems with
// different fixes, and the panel must not report one as the other.
func TestInsightEngineUnreachableIsDistinct(t *testing.T) {
	got := joined(insightLines(insight{err: "engine unreachable"}, &feed{}, false))
	if !strings.Contains(got, "unreachable") {
		t.Errorf("want the engine error, got:\n%s", got)
	}
	if strings.Contains(got, "director serve") {
		t.Error("an unreachable engine must not advise starting the Director")
	}
}

// The whole point of the panel: a provider that is registered and healthy but reaching
// none of the samples. It is invisible in every other view and it is the shape of the
// unpinned-accessibility defect.
func TestInsightFlagsAProviderThatContributedNothing(t *testing.T) {
	in := decode(t, `{
	  "running": true, "uptime": "8s",
	  "perception": {
	    "providers": [
	      {"name":"accessibility","sources":["accessibility"],"observations":3},
	      {"name":"vision","sources":["vision"],"observations":0}
	    ],
	    "cycles": [{"count":3,"duration":75000000,"scoped":true}],
	    "fusion": {"observation_count":3,"element_count":1,"merged":0,"rejected":0},
	    "provenance_ok": true
	  }
	}`)
	got := joined(insightLines(in, &feed{}, false))

	if !strings.Contains(got, "accessibility") || strings.Contains(
		lineFor(got, "accessibility"), "<- none") {
		t.Errorf("a contributing provider was flagged as empty:\n%s", got)
	}
	if !strings.Contains(lineFor(got, "vision"), "<- none") {
		t.Errorf("vision contributed nothing and was not flagged:\n%s", got)
	}
	if !strings.Contains(got, "3 obs -> 1 elements") {
		t.Errorf("want the fusion summary, got:\n%s", got)
	}
	// Duration arrives as nanoseconds; rendering it raw would print "75000000".
	if !strings.Contains(got, "75ms") {
		t.Errorf("cycle duration not rendered as a duration:\n%s", got)
	}
	if !strings.Contains(got, "(scoped)") {
		t.Errorf("a scoped cycle should say so:\n%s", got)
	}
}

func TestInsightSurfacesDegradedAndProvenance(t *testing.T) {
	in := decode(t, `{
	  "running": true,
	  "perception": {
	    "providers": [{"name":"ocr","sources":["ocr"],"observations":2}],
	    "fusion": {"observation_count":2,"element_count":2,"degraded":["ocr"]},
	    "provenance_ok": false
	  }
	}`)
	got := joined(insightLines(in, &feed{}, false))
	if !strings.Contains(got, "degraded: ocr") {
		t.Errorf("want the degraded source, got:\n%s", got)
	}
	if !strings.Contains(got, "PROVENANCE INCOMPLETE") {
		t.Errorf("broken provenance must be visible — it means a belief cannot be traced:\n%s", got)
	}
}

// `label` is omitempty, so an anonymous observation has no key at all. Decoding it as ""
// is what makes the count correct; a struct that required the key would count zero.
func TestInsightDeepCountsAnonymousFromAbsentLabels(t *testing.T) {
	in := decode(t, `{
	  "running": true, "deep": true,
	  "perception": {
	    "providers": [{"name":"accessibility","sources":["accessibility"],"observations":3}],
	    "fusion": {"observation_count":3,"element_count":2},
	    "provenance_ok": true,
	    "observations": [{"label":"DNFC"},{"label":"DNFC"},{"source":"accessibility"}],
	    "observations_total": 3
	  }
	}`)
	got := joined(insightLines(in, &feed{}, false))
	if !strings.Contains(got, "anonymous 33%") {
		t.Errorf("want 1 of 3 anonymous, got:\n%s", got)
	}
	if !strings.Contains(got, "frozen") {
		t.Errorf("a deep snapshot must say it is frozen, or it reads as live:\n%s", got)
	}
}

// The browser view is bounded server-side, so a percentage over the shown rows is not a
// figure for the whole cycle and must not be presented as one.
func TestInsightDeepSaysWhenTheCycleHeldMore(t *testing.T) {
	in := decode(t, `{
	  "running": true, "deep": true,
	  "perception": {
	    "fusion": {"observation_count":900,"element_count":400},
	    "provenance_ok": true,
	    "observations": [{"label":"Save"},{"source":"vision"}],
	    "observations_total": 900
	  }
	}`)
	got := joined(insightLines(in, &feed{}, false))
	if !strings.Contains(got, "cycle held 900") {
		t.Errorf("a truncated sample must disclose the true total:\n%s", got)
	}
}

// The cheap poll cannot know the anonymous share, and showing 0%% would read as "nothing
// is anonymous" — the opposite of what is currently true.
func TestInsightShallowDoesNotClaimAnAnonymousShare(t *testing.T) {
	in := decode(t, `{
	  "running": true,
	  "perception": {
	    "providers": [{"name":"accessibility","sources":["accessibility"],"observations":3}],
	    "fusion": {"observation_count":3,"element_count":1},
	    "provenance_ok": true
	  }
	}`)
	got := joined(insightLines(in, &feed{}, false))
	if strings.Contains(got, "anonymous 0%") {
		t.Errorf("the shallow path must not invent an anonymous share:\n%s", got)
	}
	if !strings.Contains(got, "type 'explain'") {
		t.Errorf("want the pointer to the deep view, got:\n%s", got)
	}
}

func TestInsightHandlesAnEmptyRegistry(t *testing.T) {
	in := decode(t, `{"running":true,"perception":{"fusion":{},"provenance_ok":true}}`)
	got := joined(insightLines(in, &feed{}, false))
	if !strings.Contains(got, "no providers registered") {
		t.Errorf("want the empty-registry line, got:\n%s", got)
	}
}

func TestTruncate(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		n        int
	}{
		{"short", "short", 10},
		{"exactly10!", "exactly10!", 10},
		{"this is far too long", "this is f.", 10},
	} {
		if got := truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("truncate(%q,%d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// lineFor returns the rendered line mentioning needle, for asserting per-row formatting.
func lineFor(all, needle string) string {
	for _, l := range strings.Split(all, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

// ── world payload ─────────────────────────────────────────────────────────────

// The overlay renders what the Director decided. A redacted label arrives redacted, and
// the panel shows its digest rather than inventing a name or blanking the row.
func TestWorldRendersARedactedLabelAsItsDigest(t *testing.T) {
	in := decode(t, `{
	  "running": true,
	  "world": {"believed": true, "total": 1, "app": "chrome", "freshness_ms": 90,
	    "entities": [
	      {"identity":"ab12cd34","role":"icon","confidence":0.8,
	       "label":{"digest":"ff5d43925d81","length":12,"redacted":true}}
	    ]},
	  "perception": {"fusion": {}, "provenance_ok": true}
	}`)
	got := joined(insightLines(in, &feed{}, false))

	if !strings.Contains(got, "ff5d4392") {
		t.Errorf("a withheld label should show its digest:\n%s", got)
	}
	for _, leak := range []string{"Chris", "Haynes"} {
		if strings.Contains(got, leak) {
			t.Errorf("plaintext leaked into the panel:\n%s", got)
		}
	}
}

// "Has not looked" and "looked and found nothing" are different states.
func TestUnbelievedWorldSaysSoRatherThanShowingZero(t *testing.T) {
	in := decode(t, `{"running":true,"world":{"believed":false},
	  "perception":{"fusion":{},"provenance_ok":true}}`)
	got := joined(insightLines(in, &feed{}, false))
	if !strings.Contains(got, "not observed yet") {
		t.Errorf("want the unobserved state, got:\n%s", got)
	}
}

// Inspector shows the fields a validation pass compares against `marco director world`.
func TestInspectorAddsIdentityAndStability(t *testing.T) {
	payload := `{
	  "running": true,
	  "world": {"believed": true, "total": 1,
	    "entities": [
	      {"identity":"ab12cd34","role":"button","confidence":0.9,
	       "stable_cycles":7,"stable":true,"age_ms":4200,
	       "sources":["accessibility"],
	       "label":{"text":"Save"}}
	    ]},
	  "perception": {"fusion": {}, "provenance_ok": true}
	}`
	plain := joined(insightLines(decode(t, payload), &feed{}, false))
	deep := joined(insightLines(decode(t, payload), &feed{}, true))

	if strings.Contains(plain, "ab12cd34") {
		t.Errorf("the HUD should not show raw identity:\n%s", plain)
	}
	if !strings.Contains(deep, "ab12cd34") || !strings.Contains(deep, "7c") {
		t.Errorf("inspector should show identity and run length:\n%s", deep)
	}
}

// ── event feed ────────────────────────────────────────────────────────────────

func TestFeedAppendsEventsAndAdvancesTheCursor(t *testing.T) {
	f := &feed{}
	f.absorb(decode(t, `{"events":{"epoch":"e1","newest":2,"oldest":1,
	  "events":[{"seq":1,"kind":"cycle","count":3},
	            {"seq":2,"kind":"source_silent","source":"ocr"}]}}`))

	if f.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", f.cursor)
	}
	if len(f.lines) != 2 {
		t.Fatalf("want 2 feed lines, got %v", f.lines)
	}
	if !strings.Contains(f.lines[1], "source_silent") || !strings.Contains(f.lines[1], "ocr") {
		t.Errorf("event not rendered: %q", f.lines[1])
	}
}

// A quiet poll must not invent a line, and must not rewind the cursor.
func TestFeedStaysQuietWhenNothingHappened(t *testing.T) {
	f := &feed{}
	f.absorb(decode(t, `{"events":{"epoch":"e1","newest":5,"oldest":1,
	  "events":[{"seq":5,"kind":"cycle"}]}}`))
	before := len(f.lines)

	f.absorb(decode(t, `{"events":{"epoch":"e1","newest":5,"oldest":1}}`))
	if len(f.lines) != before {
		t.Errorf("a quiet poll added lines: %v", f.lines)
	}
	if f.cursor != 5 || f.missed {
		t.Errorf("cursor moved or a gap was invented: cursor=%d missed=%v", f.cursor, f.missed)
	}
}

// The ring rolled past us. Dropped overlay frames are fine; dropped Director events are
// not, so this is reported rather than papered over.
func TestFeedDetectsMissedEvents(t *testing.T) {
	f := &feed{epoch: "e1", cursor: 2}
	f.absorb(decode(t, `{"events":{"epoch":"e1","newest":40,"oldest":12,
	  "events":[{"seq":40,"kind":"cycle"}]}}`))

	if !f.missed {
		t.Fatal("a gap between cursor and oldest was not detected")
	}
	if !strings.Contains(joined(f.lines), "missed events 3..11") {
		t.Errorf("the gap was not reported in the feed: %v", f.lines)
	}
}

// After a restart sequence numbers begin again, which to a client holding a high cursor
// looks exactly like a rollover. The epoch is what tells them apart.
func TestFeedDetectsARestartRatherThanAGap(t *testing.T) {
	f := &feed{epoch: "e1", cursor: 40}
	f.absorb(decode(t, `{"events":{"epoch":"e2","newest":2,"oldest":1,
	  "events":[{"seq":1,"kind":"cycle"},{"seq":2,"kind":"cycle"}]}}`))

	if f.missed {
		t.Error("a restart was misreported as missed events")
	}
	if f.epoch != "e2" || f.cursor != 2 {
		t.Errorf("cursor was not reset for the new epoch: epoch=%s cursor=%d", f.epoch, f.cursor)
	}
	if !strings.Contains(joined(f.lines), "restarted") {
		t.Errorf("the restart was not reported: %v", f.lines)
	}
}

// The first poll of a fresh feed is not a restart — there was no previous epoch.
func TestFirstPollIsNotReportedAsARestart(t *testing.T) {
	f := &feed{}
	f.absorb(decode(t, `{"events":{"epoch":"e1","newest":1,"oldest":1,
	  "events":[{"seq":1,"kind":"cycle"}]}}`))
	if strings.Contains(joined(f.lines), "restarted") {
		t.Errorf("the first poll claimed a restart: %v", f.lines)
	}
}

func TestFeedIsBounded(t *testing.T) {
	f := &feed{}
	for i := 1; i <= 200; i++ {
		f.absorb(insight{Events: eventsWith(uint64(i))})
	}
	if len(f.lines) > feedLines {
		t.Errorf("feed grew to %d lines, cap is %d", len(f.lines), feedLines)
	}
	if f.cursor != 200 {
		t.Errorf("cursor = %d, want 200", f.cursor)
	}
}

// eventsWith builds a one-event payload at the given sequence.
func eventsWith(seq uint64) struct {
	Epoch  string `json:"epoch"`
	Newest uint64 `json:"newest"`
	Oldest uint64 `json:"oldest"`
	Events []struct {
		Seq      uint64 `json:"seq"`
		Kind     string `json:"kind"`
		Source   string `json:"source"`
		Count    int    `json:"count"`
		Detail   string `json:"detail"`
		Duration int64  `json:"duration"`
	} `json:"events"`
} {
	var in insight
	in.Events.Epoch = "e1"
	in.Events.Newest = seq
	in.Events.Oldest = 1
	in.Events.Events = append(in.Events.Events, struct {
		Seq      uint64 `json:"seq"`
		Kind     string `json:"kind"`
		Source   string `json:"source"`
		Count    int    `json:"count"`
		Detail   string `json:"detail"`
		Duration int64  `json:"duration"`
	}{Seq: seq, Kind: "cycle"})
	return in.Events
}
