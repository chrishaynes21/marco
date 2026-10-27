package fixture_test

// WHAT THE FIXTURE LOGS OF A DRAG'S TRAVEL, AS A PROPERTY OF ITS SOURCE.
//
// A drag is not its two ends — those are two clicks — and the travel between them is the one part
// of the gesture no account outside the application can see. So the fixture logs movement, and
// ONLY while a button is down: movement without one is somebody's cursor crossing the window, a
// replay aims before every click, and a log that recorded all of it would bury the presses in
// thousands of records and stop being evidence.
//
// Checked against the C# because that is where the rule lives and the alternative is checking it
// by running a window on somebody's desktop. It is the same kind of test as the no-input rule in
// `plugins/e2e`'s `boundary_test.go`: about what the code can reach, failing on the day somebody
// takes a shortcut rather than going green beside a scenario that has stopped establishing
// anything.
//
// It lives beside the C# rather than beside the harness because `plugins/e2e` is a module of its
// own, and `go test` caches a result straight past a change to a file outside the module it is
// testing. Measured: the gate below was deleted and the suite said `ok (cached)`. See source.go.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/plugins/fixture"
)

// fixtureSource is the C# as a build input, not as a file read at test time. See the package
// comment in source.go: a file outside the module is one `go test` caches straight past.
func fixtureSource(t *testing.T) string {
	t.Helper()
	if fixture.Source == "" {
		t.Fatal("the fixture's source did not embed")
	}
	return fixture.Source
}

// THE MOVE HANDLER IS GATED ON A BUTTON BEING DOWN, AND THE GATE IS THE FIRST THING IN IT.
//
// First, because a gate below the record-building would still write the record. Asserted on the
// shape rather than on a log from a run: a run that happened to log nothing proves nothing about
// why.
func TestTheFixtureLogsMovementOnlyWhileAButtonIsDown(t *testing.T) {
	src := fixtureSource(t)

	body := regexp.MustCompile(`(?s)private void NoteMove\(Message m\)\s*\{(.*?)\n        \}`).
		FindStringSubmatch(src)
	if body == nil {
		t.Fatal("the fixture has no NoteMove, so a drag's travel reaches no log and " +
			"nothing outside Marco can tell a drag from two clicks")
	}
	lines := strings.Split(strings.TrimSpace(body[1]), "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "buttonsDown <= 0") ||
		!strings.Contains(lines[0], "return") {
		t.Errorf("NoteMove's first line is %q, and it has to be the gate: a check "+
			"below the record-building would still write the record",
			strings.TrimSpace(lines[0]))
	}
	// AND THE MESSAGE FILTER CALLS IT.
	//
	// The gap this had: everything above is about a function, and a perfect `NoteMove`
	// nobody calls logs exactly as much as no `NoteMove` at all. `WM_MOUSEMOVE` used to
	// be `break;` with a comment saying so, which is the thing that has to have changed.
	if !regexp.MustCompile(`case WmMouseMove:\s*form\.NoteMove\(m\);`).MatchString(src) {
		t.Error("the fixture's message filter does not hand WM_MOUSEMOVE to " +
			"NoteMove, so the travel of a drag reaches no log however well " +
			"NoteMove is written")
	}
	// AND THE COUNT COMES FROM THE EDGES THIS WINDOW RECEIVED, not from the mouse now.
	//
	// `MouseButtons` is the state at the instant it is read, which is after the one being
	// logged — and a witness that answered about a later instant is the defect this whole
	// file exists to prevent, one layer in.
	// Asked of the CODE and not of the file, because the comment above `buttonsDown`
	// explains why `MouseButtons` is the wrong instrument and names it — and a test that
	// failed on the explanation would be a test that punishes writing one down.
	if strings.Contains(csharpCode(src), "MouseButtons") {
		t.Error("the fixture reads MouseButtons somewhere; whether a button is down " +
			"has to come from the edges this window was sent, because that is " +
			"the only account that is about the right instant")
	}
	if !regexp.MustCompile(`buttonsDown \+= down \? 1 : -1`).MatchString(src) {
		t.Error("the held-button count is not kept from the down and up edges " +
			"NoteClick receives")
	}
	// AND IT IS A COUNT RATHER THAN A FLAG, because two buttons can be down at once and
	// releasing one of them is not the end of the gesture.
	if !strings.Contains(src, "private int buttonsDown") {
		t.Error("what is held is not a count")
	}
	// AND IT CANNOT GO NEGATIVE. A release this window saw without its press — the press
	// having gone somewhere else — would otherwise leave the count below zero and make
	// every later move invisible, which is a log that silently stops being evidence.
	if !strings.Contains(src, "if (buttonsDown < 0) { buttonsDown = 0; }") {
		t.Error("the held-button count is not floored at zero, so one unmatched " +
			"release makes every later move invisible")
	}
}

// csharpCode is the fixture with its `//` and `///` lines removed.
//
// Crude on purpose — it does not understand strings or block comments, and it does not need to:
// what it is for is keeping a test about what the code DOES from failing on a comment that says
// what the code deliberately does not do. `withoutComments` next door is the Go equivalent and
// parses, which a second language does not get for free.
func csharpCode(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
