package demo

// THE PICTURES OF A DEMONSTRATION, KEPT BESIDE IT, GOING AWAY WITH IT, AND NEVER SHOWN BESIDE
// ANOTHER ONE.
//
// Two of those three are the part worth a test. These are crops of somebody's desktop: a routine
// that is forgotten while its pictures stay behind leaves images of their screen filed under the
// name of a thing that no longer exists, where nothing will ever look at them again and nothing
// will ever clean them up. And a routine demonstrated AGAIN under the same name has new words,
// new steps and new controls — pictures of the previous recording shown beside those are a
// review misrepresenting what Play would perform, which is the one job a review has.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func aStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// aRoutine is one revision of a saved routine: the name and the instant, which together are what
// its pictures are filed under.
func aRoutine(name string, at time.Time) workflowstore.Artifact {
	return workflowstore.Artifact{Version: workflowstore.Version, Name: name,
		SavedAt: at.UTC()}
}

func TestAPictureIsKeptUnderTheLabelTheArtifactUses(t *testing.T) {
	s := aStore(t)
	png := []byte("not really a png, and this layer does not care")
	graphics := aRoutine("ark graphics", time.Now())
	audio := aRoutine("ark audio", time.Now())

	if err := s.SavePreview(graphics, "screen_a", png); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(graphics, "control_1", png); err != nil {
		t.Fatal(err)
	}
	// AND A SECOND ROUTINE'S PICTURES ARE ITS OWN, which is what the directory per
	// routine is for: two demonstrations both have a `screen_a`.
	if err := s.SavePreview(audio, "screen_a", []byte("another one")); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Preview(graphics, "screen_a")
	if !ok || string(got) != string(png) {
		t.Errorf("read back %q (ok=%v)", got, ok)
	}
	if other, ok := s.Preview(audio, "screen_a"); !ok ||
		string(other) == string(png) {
		t.Errorf("one routine's picture was served for another: %q", other)
	}
	if labels := s.Previews(graphics); len(labels) != 2 ||
		labels[0] != "control_1" || labels[1] != "screen_a" {
		t.Errorf("the labels with a picture are %v", labels)
	}
	// AND A LABEL WITH NO PICTURE IS NOT AN ERROR. A demonstration recorded before
	// pictures were kept has none, and that is not a fault.
	if _, ok := s.Preview(graphics, "screen_z"); ok {
		t.Error("a picture came back for a label that has none")
	}
	if labels := s.Previews(aRoutine("nothing by this name", time.Now())); labels != nil {
		t.Errorf("a routine nobody recorded has pictures: %v", labels)
	}
}

// PICTURES OF ONE RECORDING ARE NEVER SERVED FOR ANOTHER, WHATEVER IS ON DISK.
//
// The read side on its own, with nothing cleared and nothing saved: a store holding the pictures
// of one revision is asked for them as another revision, and has none. That is what makes the
// review honest even if removing the old ones failed, or if a file was put there by hand, or if
// this build is reading a store an older one wrote.
func TestPicturesOfOneRevisionAreNeverServedForAnother(t *testing.T) {
	s := aStore(t)
	first := aRoutine("the audio tab", time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC))
	second := aRoutine("the audio tab", time.Date(2026, 10, 20, 11, 30, 0, 0, time.UTC))
	if err := s.SavePreview(first, "screen_a", []byte("the FIRST one")); err != nil {
		t.Fatal(err)
	}

	if raw, ok := s.Preview(second, "screen_a"); ok {
		t.Errorf("a picture of one recording was served for another: %q", raw)
	}
	if labels := s.Previews(second); len(labels) != 0 {
		t.Errorf("another recording's labels are reported as this one's: %v", labels)
	}
	// AND THE REVISION IT BELONGS TO STILL HAS IT. A link checked on every read is not
	// a link that refuses everything.
	if raw, ok := s.Preview(first, "screen_a"); !ok || string(raw) != "the FIRST one" {
		t.Errorf("the recording that kept it is served %q (ok=%v)", raw, ok)
	}
}

// A ROUTINE DEMONSTRATED AGAIN NEVER SHOWS THE PREVIOUS RECORDING'S PICTURES.
//
// The defect this closes, as reported: pictures were filed under the routine's NAME, and
// `SaveSequence` lets the same name be replaced. Record it again without pictures and nothing is
// written and nothing is cleared, so the old recording's `screen_a` is served beside the new
// recording's words, steps and controls — of a demonstration that may have started somewhere
// else entirely.
//
// Both halves of the report are asserted here, because they fail differently: a replacement with
// NO pictures at all is the case where nothing in the preview path runs, and a replacement with
// only SOME is the case where what runs does not cover what is already there.
func TestARerecordedRoutineNeverShowsTheOldRecordingsPictures(t *testing.T) {
	s := aStore(t)
	first := aRoutine("the audio tab", time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC))
	if err := s.SaveSequence(first); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"screen_a", "screen_b", "control_1"} {
		if err := s.SavePreview(first, label,
			[]byte("the FIRST recording's "+label)); err != nil {
			t.Fatal(err)
		}
	}

	// ── (1) RECORDED AGAIN, WITH NO PICTURES AT ALL ──
	second := aRoutine("the audio tab", time.Date(2026, 10, 20, 11, 30, 0, 0, time.UTC))
	if err := s.SaveSequence(second); err != nil {
		t.Fatal(err)
	}
	if labels := s.Previews(second); len(labels) != 0 {
		t.Errorf("the new recording serves %v, which it never kept", labels)
	}
	for _, label := range []string{"screen_a", "screen_b", "control_1"} {
		if raw, ok := s.Preview(second, label); ok {
			t.Errorf("%s served %q from a recording this is not", label, raw)
		}
	}
	// AND THE OLD ONES ARE NOT JUST UNREACHABLE, THEY ARE GONE. Crops of a desktop
	// under the name of a demonstration that has moved on are read by nothing and
	// cleaned up by nothing.
	dir, derr := s.previewsOf(first)
	if derr != nil {
		t.Fatal(derr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the first recording's pictures are still on disk at %s (%v)",
			dir, err)
	}

	// ── (2) AND RECORDED AGAIN WITH ONLY SOME OF THEM ──
	third := aRoutine("the audio tab", time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	if err := s.SavePreview(second, "screen_a", []byte("the SECOND one")); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(second, "control_1", []byte("the SECOND one")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(third); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(third, "screen_a", []byte("the THIRD one")); err != nil {
		t.Fatal(err)
	}
	if raw, ok := s.Preview(third, "screen_a"); !ok || string(raw) != "the THIRD one" {
		t.Errorf("the screen it starts on served %q (ok=%v)", raw, ok)
	}
	// THE CONTROL IT DID NOT KEEP A PICTURE OF THIS TIME SHOWS NONE, rather than the
	// one it kept last time. The words beside it are this recording's.
	if raw, ok := s.Preview(third, "control_1"); ok {
		t.Errorf("the control served %q, kept by the recording before this one", raw)
	}

	// ── AND AN ARTIFACT WITH NO INSTANT IS STILL SAVED ──
	//
	// It cannot be the recording any kept picture came from, and no picture can be
	// kept for it. So its pictures go and the ROUTINE is still written: refusing to
	// save somebody's routine because its pictures could not be keyed would be this
	// feature deciding whether a demonstration exists.
	unstamped := workflowstore.Artifact{Version: workflowstore.Version,
		Name: "the audio tab", Application: "arkascended",
		Controls: []workflowstore.Control{{Label: "control_1"}},
		Steps: []workflowstore.Step{{Position: 0, Control: "control_1",
			Verb: "open"}}}
	if err := s.SaveSequence(unstamped); err != nil {
		t.Fatalf("a routine with no instant was refused: %v", err)
	}
	if _, ok := s.Sequence("the audio tab"); !ok {
		t.Error("the routine was not saved")
	}
	dir, derr = s.previewsFor("the audio tab")
	if derr != nil {
		t.Fatal(derr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("pictures of a recording this one cannot be are still at %s (%v)",
			dir, err)
	}
}

// A ROUTINE THAT IS FORGOTTEN TAKES ITS PICTURES WITH IT.
//
// Crops of a desktop must not outlive the thing they were crops of. Nothing reads them, so they
// would never be noticed again — which is exactly why this cannot be left to be noticed.
func TestForgettingARoutineForgetsItsPictures(t *testing.T) {
	s := aStore(t)
	art := aRoutine("ark graphics", time.Now())
	if err := s.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(art, "screen_a", []byte("a picture")); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(art, "control_1", []byte("another")); err != nil {
		t.Fatal(err)
	}

	if err := s.ForgetSequence("ark graphics"); err != nil {
		t.Fatal(err)
	}
	if labels := s.Previews(art); len(labels) != 0 {
		t.Errorf("the pictures of a forgotten routine are still there: %v", labels)
	}
	// AND THE DIRECTORY ITSELF IS GONE, not left empty with a name in it. The whole of
	// it, every revision: forgetting is by name, because there is then no revision left
	// to keep anything for.
	dir := filepath.Join(s.dir, previewDir, safeName("ark graphics"))
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s survived forgetting the routine (%v)", dir, err)
	}
}

// AND NOTHING NAMELESS IS STORED, because a file with no routine, no label or no revision has
// nothing to belong to and nothing would ever find it again.
func TestAPictureNeedsSomethingToBelongTo(t *testing.T) {
	s := aStore(t)
	at := time.Now()
	for _, c := range []struct {
		why   string
		art   workflowstore.Artifact
		label string
	}{
		{"no routine", aRoutine("", at), "screen_a"},
		{"a blank routine", aRoutine("  ", at), "screen_a"},
		{"no label", aRoutine("ark graphics", at), ""},
		{"a blank label", aRoutine("ark graphics", at), "   "},
		// AND NO REVISION. A stamp of "zero" would be one directory every
		// unstamped artifact shared, which is the collision the stamp exists to
		// prevent — so this is refused rather than filed under nothing.
		{"no instant", workflowstore.Artifact{Name: "ark graphics"}, "screen_a"},
	} {
		if err := s.SavePreview(c.art, c.label, []byte("x")); err == nil {
			t.Errorf("a picture was stored with %s", c.why)
		}
	}
	if err := s.SavePreview(aRoutine("ark graphics", at), "screen_a", nil); err == nil {
		t.Error("an empty picture was stored")
	}
}
