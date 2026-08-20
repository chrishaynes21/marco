package observesession_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

// The keylogging regression, asserted across the WHOLE terminal record rather than at one type.
//
// # Why this is not enough to check InputEvent
//
// InputEvent has its own shape test, and it is a good one. It also only proves that ONE type is
// clean. What a session actually hands over is a Result: it reaches the service protocol, the
// CLI, the JSON a user can pipe to a file, and any fixture captured from it. A physical-key
// identity added anywhere in that graph — on a track, on a state, on a statistic, three types
// down inside a diagnostic — would be just as durable and would not trip a test bound to
// InputEvent.
//
// So this walks the type graph of everything that leaves a session and fails on any field that
// names a physical input. It is a structural test: it never runs a session, so it cannot be
// satisfied by a session that happened not to populate the field.
//
// # Why names, and not only types
//
// Because `RawKey int` is a keylogger and an int is not suspicious. The rule that catches it is
// the NAME, which is why the forbidden list below is about vocabulary rather than about shape.
// A string field elsewhere in a Result is often perfectly legitimate — a window title, a
// diagnostic reason authored by Marco — and a type-based rule alone would either miss the
// integer or condemn all of those.

// forbiddenInputNames is the vocabulary of physical-input identity.
//
// Deliberately including "raw" and "device": the point is not to enumerate every spelling of
// "key code", it is that anything in a passive observation record naming the hardware the user
// touched is wrong on its face.
var forbiddenInputNames = []string{
	"keycode", "scancode", "vkcode", "rawkey", "rawbutton", "buttonmask",
	"scan_code", "key_code", "deviceid", "device_id", "keystroke", "keypress",
	"rune", "charcode", "character", "keyname", "keysym",
}

// walked guards against the cycles a type graph can contain.
type walked map[reflect.Type]bool

// checkPhysicalInputIdentity walks a type graph and reports any field naming physical input.
func checkPhysicalInputIdentity(t *testing.T, rt reflect.Type, path string, seen walked) {
	t.Helper()
	for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
		rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {

		if rt.Kind() == reflect.Map {
			checkPhysicalInputIdentity(t, rt.Key(), path+"[key]", seen)
		}
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct || seen[rt] {
		return
	}
	seen[rt] = true

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name := strings.ToLower(f.Name)
		tag := strings.ToLower(f.Tag.Get("json"))
		for _, bad := range forbiddenInputNames {
			if strings.Contains(name, bad) || strings.Contains(tag, bad) {
				t.Errorf("%s.%s (%s) names a physical input. A passive observation record "+
					"may remember what the user MEANT — a closed navigation intent — and "+
					"never which key they pressed. An integer key code is keylogging "+
					"material exactly as much as a string one",
					path, f.Name, f.Type)
			}
		}
		checkPhysicalInputIdentity(t, f.Type, path+"."+f.Name, seen)
	}
}

// Nothing a session hands over may name a key.
func TestNoPhysicalKeyIdentitySurvivesIntoASessionResult(t *testing.T) {
	checkPhysicalInputIdentity(t, reflect.TypeOf(observesession.Result{}), "Result", walked{})
}

// Nor may a captured trace, which is the durable one.
//
// A Result lives in memory until the service restarts. A trace is a file on disk that outlives
// everything, gets attached to an experiment record, and is the artifact most likely to be
// shared — so it is the one where a leaked key identity would do the most lasting damage.
func TestNoPhysicalKeyIdentitySurvivesIntoACapturedTrace(t *testing.T) {
	checkPhysicalInputIdentity(t, reflect.TypeOf(shadowreplay.Slot{}), "Slot", walked{})
}

// The forbidden list must actually catch what it is written for.
//
// A structural test that silently matches nothing is worse than no test: it reports green
// forever and everyone stops thinking about it. This is the fixture that proves the walker
// finds a violation planted three levels down inside a slice of a map value.
func TestTheKeyIdentityWalkerCatchesAPlantedViolation(t *testing.T) {
	type deepest struct{ ScanCode uint32 }
	type middle struct{ Events []deepest }
	type outer struct{ ByState map[string]middle }

	fake := &testing.T{}
	checkPhysicalInputIdentity(fake, reflect.TypeOf(outer{}), "outer", walked{})
	if !fake.Failed() {
		t.Fatal("the walker did not find a ScanCode planted three types deep. It is " +
			"therefore not proving anything about the Result graph either")
	}
}
