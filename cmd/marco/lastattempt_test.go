package main

// "WHY DIDN'T THAT WORK?" HAS AN ANSWER, through the one intake (lastattempt.go).
//
// Each case asks for something through runInvocation — what `marco do`, the widget and a hotkey all
// reach — and then reads the record the intake left, as `marco last` does. The refusals that happen
// BEFORE a run (a value missing, the routine changed since it was confirmed, another application in
// front, nothing answering) used to leave nothing; a run leaves its reference. And nothing anybody
// supplied is in the file.
//
// See [[ADR-320-why-didnt-that-work]].

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// theLastAttempt is the record, and its raw bytes for asserting what is NOT in it.
func theLastAttempt(t *testing.T) (lastattempt.Attempt, string) {
	t.Helper()
	a, ok, err := lastattempt.Read(marcopaths.Home())
	if err != nil || !ok {
		t.Fatalf("no record of the attempt (%v, %v)", ok, err)
	}
	raw, _ := os.ReadFile(lastattempt.Path(marcopaths.Home()))
	return a, string(raw)
}

const suppliedValue = "zq7value9f3k"

// A VALUE NOTHING SUPPLIED: refused before the run, at `values`, saying which value and how to
// give it — and no reference, because nothing ran.
//
// Not noting the refusal at take() must fail this.
func TestARefusalBeforeTheRunIsWrittenDown(t *testing.T) {
	d, _ := aRoutineThatTypesAValue(t)
	saidOnStdout(t, func() { _, _ = doAsProduct(t, d, "write a note", nil, nil) })
	a, _ := theLastAttempt(t)
	if a.Outcome != "refused" || a.Stage != lastattempt.StageValues || a.Run != "" {
		t.Fatalf("recorded %+v", a)
	}
	if a.Routine != "write a note" || !strings.Contains(a.Because, "message") ||
		!strings.Contains(a.Next, "“write a note message:…”") || !strings.Contains(a.Because, "\"write a note\"") || a.Words != 3 || a.Source != string(invoke.SourceCLI) {
		t.Errorf("recorded %+v", a)
	}
	says := strings.Join(a.Says(time.Now()), "\n")
	for _, want := range []string{"“write a note” was refused before anything ran", "Why: ", "Next: say it again"} {
		if !strings.Contains(says, want) {
			t.Errorf("the explanation does not say %q:\n%s", want, says)
		}
	}
}

// THE ROUTINE CHANGED SINCE IT WAS CONFIRMED: `version`, and the next step is to read it again.
// The value given with it is not in the record.
func TestAStaleConfirmationIsWrittenDownWithoutItsValue(t *testing.T) {
	d, host := aRoutineThatTypesAValue(t)
	saidOnStdout(t, func() {
		_, _ = doAsProductExpecting(t, d, "write a note", map[string]string{"message": suppliedValue},
			"not-the-digest-it-has")
	})
	if len(typed(host)) != 0 {
		t.Fatalf("it typed %v", typed(host))
	}
	a, raw := theLastAttempt(t)
	if a.Outcome != "refused" || a.Stage != lastattempt.StageVersion || !strings.Contains(a.Next, "read-back") {
		t.Errorf("recorded %+v", a)
	}
	if strings.Contains(raw, suppliedValue) {
		t.Errorf("the value is in the record:\n%s", raw)
	}
}

// ANOTHER APPLICATION IN FRONT: `scope`, in the door's own sentence.
func TestARoutineRefusedForWhatIsInFrontIsWrittenDown(t *testing.T) {
	d := intakeWorld(t) // discord is in front
	noDirector(t)
	_, _ = captureStdout(t, func() error {
		_, e := runInvocation(d, explicitly(notepadOnly, invoke.SourceHotkey))
		return e
	})
	a, _ := theLastAttempt(t)
	if a.Outcome != "refused" || a.Stage != lastattempt.StageScope ||
		!strings.Contains(a.Because, "discord is in front") || a.Source != string(invoke.SourceHotkey) {
		t.Errorf("recorded %+v", a)
	}
}

// NOTHING ANSWERED: `matching`, with what to do — and the words that matched nothing are not in
// the record. AND A STOP AFTERWARDS DOES NOT REPLACE IT: the command somebody stopped is the one
// they ask about.
func TestNothingAnsweringIsWrittenDownAndAStopDoesNotReplaceIt(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	phrase := "conjure the " + suppliedValue
	_, _ = captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{Text: phrase, Source: invoke.SourceSpoken})
		return e
	})
	a, raw := theLastAttempt(t)
	if a.Outcome != "unavailable" || a.Stage != lastattempt.StageMatching || !strings.Contains(a.Next, "learn") ||
		a.Words != 3 || a.Source != string(invoke.SourceSpoken) {
		t.Errorf("recorded %+v", a)
	}
	if strings.Contains(raw, suppliedValue) || strings.Contains(raw, "conjure") {
		t.Errorf("the request is in the record:\n%s", raw)
	}
	_, _ = captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{Text: "stop", Source: invoke.SourceSpoken})
		return e
	})
	if again, _ := theLastAttempt(t); again != a {
		t.Errorf("a stop replaced the record:\n%+v\nwas\n%+v", again, a)
	}
}

// A RUN THAT PERFORMED leaves its reference, so the answer can point at the run record. The value
// it typed is in neither.
func TestARunIsWrittenDownWithItsReference(t *testing.T) {
	d, host := aRoutineThatTypesAValue(t)
	saidOnStdout(t, func() {
		_, _ = doAsProduct(t, d, "write a note", map[string]string{"message": suppliedValue}, nil)
	})
	if len(typed(host)) == 0 {
		t.Fatal("nothing was typed; this case needs a run that performs")
	}
	a, raw := theLastAttempt(t)
	if a.Outcome != "performed" || a.Stage != lastattempt.StageRunning || a.Run == "" {
		t.Errorf("recorded %+v", a)
	}
	if strings.Contains(raw, suppliedValue) {
		t.Errorf("the value is in the record:\n%s", raw)
	}
}

// WHAT AN ARM NOTED IS SCRUBBED OF WHAT WAS SUPPLIED before it is written, wherever the value came
// from: the request's words, a named value, a value in order, or a routine step's.
//
// Writing the noted sentence as it is must fail this.
func TestTheRecordCarriesNothingSupplied(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	beginAttempt()
	noteStep(playInvocation{rt: routes.Route{App: "game-b", Slug: "cloud-pull"},
		named: map[string]string{"resource": "obsidian"}, positional: []string{"marble"}})
	noteEnd(lastattempt.StageRunning, "typing obsidian then marble for “pull zq7 box” was refused", "")
	keepLastAttempt(invoke.Decision{Kind: invoke.KindDirector, Phrase: "pull zq7 box"},
		invoke.Request{Text: "pull zq7 box", Named: map[string]string{"note": "hunter2"}}, OutcomeFailed, nil)
	a, raw := theLastAttempt(t)
	for _, said := range []string{"obsidian", "marble", "zq7", "hunter2"} {
		if strings.Contains(raw, said) {
			t.Errorf("%q is in the record:\n%s", said, raw)
		}
	}
	if !strings.Contains(a.Because, "typing "+lastattempt.Withheld) || a.Routine != "cloud pull" {
		t.Errorf("recorded %+v", a)
	}
}

// `marco last --json` is the record and the sentences, which is what the widget shows.
func TestMarcoLastSaysWhatHappened(t *testing.T) {
	d, _ := aRoutineThatTypesAValue(t)
	saidOnStdout(t, func() { _, _ = doAsProduct(t, d, "write a note", nil, nil) })
	out := saidOnStdout(t, func() { runLast([]string{"--json"}) })
	var got struct {
		Known   bool                `json:"known"`
		Attempt lastattempt.Attempt `json:"attempt"`
		Says    []string            `json:"says"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !got.Known || got.Attempt.Stage != lastattempt.StageValues || len(got.Says) < 3 {
		t.Errorf("marco last --json said %+v", got)
	}
	t.Setenv("MARCO_HOME", t.TempDir())
	if s := saidOnStdout(t, func() { runLast(nil) }); !strings.Contains(s, "Nothing has been asked for yet") {
		t.Errorf("with no record it says %q", s)
	}
}
