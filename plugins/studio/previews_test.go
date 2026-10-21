package main

// THE PICTURES, AS THE WINDOW SERVES THEM.
//
// What this protects is the join. A picture is filed under the artifact's own label, and the page
// asks for it by that label — so the interaction the page is showing has to carry the label of
// the thing it is showing, and a label that does not match is a card captioned "presses this"
// above a picture of something else.

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
)

func TestAPictureIsServedForTheLabelAnInteractionCarries(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	// THE SAME STORE THE WINDOW READS, found the way the window finds it: the fixture
	// points MARCO_HOME at a temporary directory and Acts opens it from there.
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	// THE PNG BYTES DO NOT MATTER HERE. What is under test is which label is asked
	// for and what comes back under it; the picture itself is internal/director/preview.
	//
	// FILED AGAINST THE ARTIFACT, because that is what they are pictures of: the
	// routine's name AND the revision of it, so the window serving them has to find the
	// same revision the store kept them under.
	art, ok := store.Sequence("ark graphics live")
	if !ok {
		t.Fatal("the fixture's demonstration is not in the store")
	}
	for label, body := range map[string]string{
		"screen_a":  "the screen it starts on",
		"screen_b":  "the screen afterwards",
		"control_1": "the control",
	} {
		if err := store.SavePreview(art, label, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	// ── THE LABELS THE WINDOW SAYS IT HAS ──
	got := a.Pictures("ark graphics live")
	if len(got.Screens) != 2 || len(got.Controls) != 1 {
		t.Fatalf("the pictures kept are %+v", got)
	}
	if got.Controls[0] != "control_1" {
		t.Errorf("the control picture is filed as %q", got.Controls[0])
	}

	// ── AND THE INTERACTION CARRIES THE KEYS TO THEM ──
	//
	// The reason this is asserted here rather than left to the page: the page shows
	// `reads` and `on`, which are WORDS, and asks for pictures by label. If the
	// interaction stopped carrying the labels, every card would simply render without a
	// picture and nothing would fail.
	var found bool
	for _, d := range a.Demonstrations() {
		if d.Name != "ark graphics live" {
			continue
		}
		for _, in := range d.Steps {
			if !in.Pressable {
				continue
			}
			found = true
			if in.ControlLabel == "" {
				t.Error("a pressing interaction carries no control label, so " +
					"its picture cannot be asked for")
			}
			if in.FromLabel == "" || in.ToLabel == "" {
				t.Errorf("interaction %d carries from=%q to=%q",
					in.Position, in.FromLabel, in.ToLabel)
			}
		}
	}
	if !found {
		t.Fatal("the fixture has no pressing interaction to carry a label")
	}

	// ── AND THE PICTURE COMES BACK AS SOMETHING THE PAGE CAN RENDER ──
	src := a.Picture("ark graphics live", "control_1")
	if !strings.HasPrefix(src, "data:image/png;base64,") {
		t.Fatalf("the picture is served as %q", src)
	}
	raw, derr := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(src, "data:image/png;base64,"))
	if derr != nil {
		t.Fatalf("what was served does not decode: %v", derr)
	}
	if string(raw) != "the control" {
		t.Errorf("the control's label served %q", raw)
	}
}

// A DEMONSTRATION WITH NO PICTURES IS NOT A FAILURE.
//
// Everything recorded before pictures were kept has none, and the card falls back to exactly
// what it showed before: the words. A window that errored here would make every old
// demonstration look broken.
func TestADemonstrationWithNoPicturesShowsItsWordsInstead(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	if got := a.Pictures("ark graphics live"); len(got.Screens) != 0 ||
		len(got.Controls) != 0 {
		t.Errorf("a demonstration recorded without pictures reports %+v", got)
	}
	if src := a.Picture("ark graphics live", "control_1"); src != "" {
		t.Errorf("a picture was served where none is kept: %q", src)
	}
	if src := a.Picture("nothing by this name", "screen_a"); src != "" {
		t.Errorf("a picture was served for a routine nobody recorded: %q", src)
	}
}

// AND STUDIO ASKS FOR THEM, which is the whole of the opt-in.
//
// The recorder keeps none by default and says "No frames and no screenshots are kept". That
// default is right for a command and wrong for a window whose job is to show somebody what they
// taught — so the window asks, and the card it asks from says what is kept. Asserted by reading
// the arguments the child was actually given, because a window that said "pictures are kept" and
// did not pass the flag would be a window making a promise about somebody's desktop that it then
// did not keep.
func TestStudioAsksTheRecorderForPictures(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)

	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	if why := a.Done(true); why != "" {
		t.Fatal(why)
	}
	said := strings.Join(learningSaid(t, a).Transcript, "\n")
	if !strings.Contains(said, "--previews") {
		t.Errorf("the recorder was not asked to keep pictures:\n%s", said)
	}
}

// AND A DEMONSTRATION RECORDED AGAIN NEVER SHOWS THE PREVIOUS RECORDING'S PICTURES.
//
// The window's half of the defect reported against the first version of this. Pictures were
// filed under the routine's NAME, and the same name can be demonstrated again — so a review
// could show the old recording's starting screen beside the new recording's words, steps and
// controls, which is a review misrepresenting what Play would perform.
//
// Asserted through the window and not only in the store, because this is the surface that
// decides which revision to ask for: a window that asked by name alone would serve the old
// pictures however carefully the store filed them.
func TestAReDemonstratedSceneShowsNoPicturesFromTheRecordingBeforeIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	first, ok := store.Sequence("ark graphics live")
	if !ok {
		t.Fatal("the fixture's demonstration is not in the store")
	}
	for _, label := range []string{"screen_a", "screen_b", "control_1"} {
		if err := store.SavePreview(first, label,
			[]byte("the FIRST recording's "+label)); err != nil {
			t.Fatal(err)
		}
	}
	if got := a.Pictures("ark graphics live"); len(got.Screens) != 2 {
		t.Fatalf("the first recording's pictures are not being served: %+v", got)
	}

	// ── AND IT IS DEMONSTRATED AGAIN, WITHOUT PICTURES ──
	//
	// Which is the case where nothing in the preview path runs at all: no picture is
	// written, so nothing in writing one could have cleared the old ones.
	again := first
	again.SavedAt = first.SavedAt.Add(17 * time.Minute)
	if err := store.SaveSequence(again); err != nil {
		t.Fatal(err)
	}

	if got := a.Pictures("ark graphics live"); len(got.Screens) != 0 ||
		len(got.Controls) != 0 {
		t.Errorf("the new recording is shown with %+v, which it never kept", got)
	}
	for _, label := range []string{"screen_a", "screen_b", "control_1"} {
		if src := a.Picture("ark graphics live", label); src != "" {
			t.Errorf("%s served a picture from the recording before this one",
				label)
		}
	}
}
