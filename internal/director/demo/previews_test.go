package demo

// THE PICTURES OF A DEMONSTRATION, KEPT BESIDE IT AND GOING AWAY WITH IT.
//
// The second of those is the part worth a test. These are crops of somebody's desktop: a routine
// that is forgotten while its pictures stay behind leaves images of their screen filed under the
// name of a thing that no longer exists, where nothing will ever look at them again and nothing
// will ever clean them up.

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

func TestAPictureIsKeptUnderTheLabelTheArtifactUses(t *testing.T) {
	s := aStore(t)
	png := []byte("not really a png, and this layer does not care")

	if err := s.SavePreview("ark graphics", "screen_a", png); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview("ark graphics", "control_1", png); err != nil {
		t.Fatal(err)
	}
	// AND A SECOND ROUTINE'S PICTURES ARE ITS OWN, which is what the directory per
	// routine is for: two demonstrations both have a `screen_a`.
	if err := s.SavePreview("ark audio", "screen_a", []byte("another one")); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Preview("ark graphics", "screen_a")
	if !ok || string(got) != string(png) {
		t.Errorf("read back %q (ok=%v)", got, ok)
	}
	if other, ok := s.Preview("ark audio", "screen_a"); !ok ||
		string(other) == string(png) {
		t.Errorf("one routine's picture was served for another: %q", other)
	}
	if labels := s.Previews("ark graphics"); len(labels) != 2 ||
		labels[0] != "control_1" || labels[1] != "screen_a" {
		t.Errorf("the labels with a picture are %v", labels)
	}
	// AND A LABEL WITH NO PICTURE IS NOT AN ERROR. A demonstration recorded before
	// pictures were kept has none, and that is not a fault.
	if _, ok := s.Preview("ark graphics", "screen_z"); ok {
		t.Error("a picture came back for a label that has none")
	}
	if labels := s.Previews("nothing by this name"); labels != nil {
		t.Errorf("a routine nobody recorded has pictures: %v", labels)
	}
}

// A ROUTINE THAT IS FORGOTTEN TAKES ITS PICTURES WITH IT.
//
// Crops of a desktop must not outlive the thing they were crops of. Nothing reads them, so they
// would never be noticed again — which is exactly why this cannot be left to be noticed.
func TestForgettingARoutineForgetsItsPictures(t *testing.T) {
	s := aStore(t)
	art := workflowstore.Artifact{Version: workflowstore.Version,
		Name: "ark graphics", SavedAt: time.Now().UTC()}
	if err := s.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview("ark graphics", "screen_a", []byte("a picture")); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview("ark graphics", "control_1", []byte("another")); err != nil {
		t.Fatal(err)
	}

	if err := s.ForgetSequence("ark graphics"); err != nil {
		t.Fatal(err)
	}
	if labels := s.Previews("ark graphics"); len(labels) != 0 {
		t.Errorf("the pictures of a forgotten routine are still there: %v", labels)
	}
	// AND THE DIRECTORY ITSELF IS GONE, not left empty with a name in it.
	dir := filepath.Join(s.dir, previewDir, safeName("ark graphics"))
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s survived forgetting the routine (%v)", dir, err)
	}
}

// AND NOTHING NAMELESS IS STORED, because a file with no routine and no label has nothing to
// belong to and nothing would ever find it again.
func TestAPictureNeedsSomethingToBelongTo(t *testing.T) {
	s := aStore(t)
	for _, c := range []struct{ routine, label string }{
		{"", "screen_a"},
		{"  ", "screen_a"},
		{"ark graphics", ""},
		{"ark graphics", "   "},
	} {
		if err := s.SavePreview(c.routine, c.label, []byte("x")); err == nil {
			t.Errorf("a picture was stored as routine=%q label=%q",
				c.routine, c.label)
		}
	}
	if err := s.SavePreview("ark graphics", "screen_a", nil); err == nil {
		t.Error("an empty picture was stored")
	}
}
