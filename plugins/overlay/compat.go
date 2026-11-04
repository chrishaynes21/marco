package main

// THE WIDGET DOES NOT PRESS A SHORTCUT THROUGH AN ENGINE THAT WOULD NOT CHECK IT.
//
// A key after the leader carries the application the leader was pressed in (`--armed-in=`), and
// the engine refuses the press if something else is in front by the time it reads (cmd/marco
// pressArmed). An engine older than that check takes `marco hotkey e --armed-in=discord`, reads
// only the key, and runs whatever `e` is in front: exactly the press the check exists to stop,
// with nothing anywhere saying the check did not happen. So the widget asks first, and with an
// engine that cannot say yes it refuses the press itself and says why.
//
// # How it asks
//
// There is no version handshake between the widget and `marco`, so the question is the feature
// itself: `marco bindings --here --armed-in= --json`. An engine with the check answers with an
// `armed_in` field (listShortcuts echoes it only for a carried listing). An engine from S2 refuses
// the argument, and one from before S2 has no `bindings` at all. Either way, no `armed_in`, and
// the answer is no.
//
// The answer is kept under the path, size and modification time of the `marco` it was asked of.
// That is a cheap sign the file has not changed, not proof its contents are the same, and it costs
// a press one stat. A `marco` rebuilt under a running widget gets a new time and is asked again;
// one copied in with its size and time preserved would not be (ADR-299).
//
// A press made through a script (`Overlay Hotkey` with no leader before it) carries nothing and
// is not gated: it never had the check, and the engine runs it in whatever is in front as before.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// contextCheck answers whether the engine checks a carried press, and if not, why not. A
// variable so a test about something else can state the engine is current.
var contextCheck = engineChecksContext

var engineCheck struct {
	sync.Mutex
	key, why string
	ok, have bool
}

// engineChecksContext is the cached answer for the `marco` the widget would spawn now.
func engineChecksContext() (ok bool, why string) {
	bin := marcoBin()
	key := bin
	if p, err := exec.LookPath(bin); err == nil {
		if st, err := os.Stat(p); err == nil {
			key = fmt.Sprintf("%s|%d|%d", p, st.Size(), st.ModTime().UnixNano())
		}
	}
	engineCheck.Lock()
	defer engineCheck.Unlock()
	if engineCheck.have && engineCheck.key == key {
		return engineCheck.ok, engineCheck.why
	}
	ok, why, definite := askEngine(bin)
	if definite {
		engineCheck.key, engineCheck.ok, engineCheck.why, engineCheck.have = key, ok, why, true
	}
	return ok, why
}

// askEngine asks bin once. An answer is definite and kept until the binary changes. No answer in
// time is a no for THIS press, with its own reason, and is not kept: a slow first start must not
// switch shortcuts off for the rest of the session.
func askEngine(bin string) (ok bool, why string, definite bool) {
	ctx, cancel := context.WithTimeout(context.Background(), armListWithin)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "bindings", "--here", armedFlag, "--json").Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) && ctx.Err() == nil {
		return false, "shortcuts are off for now: " + bin + " could not be started (" + err.Error() + ")", false
	}
	if ctx.Err() != nil {
		return false, "shortcuts are off for now: " + bin + " did not say within " + armListWithin.String() +
			" whether it checks which application a key was pressed for", false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(out, &doc) == nil {
		if _, ok := doc["armed_in"]; ok {
			return true, "", true
		}
	}
	return false, "shortcuts are off: " + bin + " is older than this widget and does not check " +
		"which application a key was pressed for — rebuild marco beside the widget", true
}

// forgetEngineCheck drops the kept answer, for a test that changes the engine under it.
func forgetEngineCheck() {
	engineCheck.Lock()
	engineCheck.have = false
	engineCheck.Unlock()
}

// warmEngineCheck asks at start-up, off every hot path, so the first press does not.
func warmEngineCheck() { go func() { _, _ = contextCheck() }() }
