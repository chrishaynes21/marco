package visionbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Declared ground truth, and why the benchmark now needs it.
//
// # What the proxy metrics could not do
//
// Version 1 measured false structure by asking whether a detection PERSISTED across frames,
// on the reasoning that a box which vanishes was probably never a box. That is a sound proxy
// for an ordered sequence of one scene. The corpus was not one — it was six unrelated crops —
// so "persisted" mostly meant "two unrelated rectangles fell in the same coarse ninth", and
// the score came out anti-correlated with precision: a detector that filtered nothing won.
//
// See [[Experiment-003-classical-cv-tuning]] for the measurement that established this.
//
// The fix is not a better proxy. It is to write down what is actually in the frame, and to
// write down what is NOT — because "this rectangle is arena texture" is the single most
// valuable statement in this corpus and no proxy can infer it.
//
// # What this is not
//
// Not a training set. The annotation is coarse, generic, and deliberately partial: only
// structures the benchmark scores are marked, nothing is exhaustively segmented, and no
// game-specific concept ever appears. A corpus that grew into training data would stop being
// a measuring stick and start being a thing to overfit to.

// GroundTruthSchema is the version of the annotation format.
//
// Bumped for any change to the fields or to the vocabulary, because two corpora annotated
// under different rules are not comparable and a report that did not say which would invite
// exactly the wrong comparison.
const GroundTruthSchema = 1

// TruthKind is the closed, GAME-AGNOSTIC vocabulary an annotation may use.
//
// Generic on purpose. "boost", "health", "inventory" and "scoreboard" are game-pack
// semantics; this corpus measures whether a detector can see a rectangle and say what SHAPE
// of control it is. Mixing the two would make the benchmark a test of Rocket League.
type TruthKind string

const (
	TruthButton     TruthKind = "button"
	TruthMenuItem   TruthKind = "menu_item"
	TruthMenu       TruthKind = "menu"
	TruthPanel      TruthKind = "panel"
	TruthTab        TruthKind = "tab"
	TruthCheckbox   TruthKind = "checkbox"
	TruthRadio      TruthKind = "radio"
	TruthField      TruthKind = "field"
	TruthBar        TruthKind = "bar"
	TruthMeter      TruthKind = "meter"
	TruthGrid       TruthKind = "grid"
	TruthGridCell   TruthKind = "grid_cell"
	TruthTextRegion TruthKind = "text_region"
	TruthIcon       TruthKind = "icon"
	TruthOverlay    TruthKind = "overlay"
	TruthUnknownUI  TruthKind = "unknown_ui"
)

// NegativeKind names a thing that is emphatically NOT interface.
//
// The half of the corpus version 1 had no way to express. A detector that finds arena texture
// and holds onto it for a hundred frames was previously rewarded for consistency; now the
// frame says outright that there is nothing there.
type NegativeKind string

const (
	NegScenery    NegativeKind = "scene_texture"
	NegGeometry   NegativeKind = "background_geometry"
	NegShadow     NegativeKind = "shadow"
	NegLighting   NegativeKind = "lighting_gradient"
	NegParticles  NegativeKind = "particles"
	NegMotionBlur NegativeKind = "motion_blur"
	NegDecorative NegativeKind = "decorative_border"
)

var truthKinds = map[TruthKind]bool{
	TruthButton: true, TruthMenuItem: true, TruthMenu: true, TruthPanel: true,
	TruthTab: true, TruthCheckbox: true, TruthRadio: true, TruthField: true,
	TruthBar: true, TruthMeter: true, TruthGrid: true, TruthGridCell: true,
	TruthTextRegion: true, TruthIcon: true, TruthOverlay: true, TruthUnknownUI: true,
}

var negativeKinds = map[NegativeKind]bool{
	NegScenery: true, NegGeometry: true, NegShadow: true, NegLighting: true,
	NegParticles: true, NegMotionBlur: true, NegDecorative: true,
}

// Known reports whether a kind is in the vocabulary.
func (k TruthKind) Known() bool { return truthKinds[k] }

// Nameable reports whether this kind's role may be said in plaintext.
//
// Mirrors the privacy allowlist as a string comparison, the same way observe and the
// nameable-coverage metric do, and for the same reason: this states what a detector COULD
// offer, not what the Director may store.
func (k TruthKind) Nameable() bool {
	switch k {
	case TruthButton, TruthMenuItem, TruthMenu, TruthTab, TruthCheckbox, TruthRadio:
		return true
	}
	return false
}

// OCRWorthy reports whether a region is a sensible place to read text.
//
// A control that carries a label is; an icon, a panel background or a decorative line is not.
// Measured rather than assumed because proposing good OCR regions may be this detector
// family's most valuable output — reading fewer, better rectangles beats reading many.
func (k TruthKind) OCRWorthy() bool {
	switch k {
	case TruthButton, TruthMenuItem, TruthTab, TruthField, TruthTextRegion:
		return true
	}
	return false
}

// Known reports whether a negative kind is in the vocabulary.
func (k NegativeKind) Known() bool { return negativeKinds[k] }

// NormRect is a rectangle in FRACTIONS of the frame: x, y, width, height, each 0..1.
//
// Normalised because the whole point of corpus v2 is that geometry must mean the same thing
// at every resolution. Version 1's crops made "fraction of frame" a different quantity in
// every file, which is how a size threshold that removes 75% of false positives on the corpus
// turned out to reject 79 of 83 candidates on a real screen.
type NormRect struct {
	X, Y, W, H float64
}

// Valid reports whether a normalised rectangle is inside the frame and has area.
func (r NormRect) Valid() bool {
	return r.W > 0 && r.H > 0 &&
		r.X >= 0 && r.Y >= 0 && r.X+r.W <= 1.0001 && r.Y+r.H <= 1.0001
}

// Pixels converts to frame coordinates.
func (r NormRect) Pixels(frame image.Rectangle) image.Rectangle {
	fw, fh := float64(frame.Dx()), float64(frame.Dy())
	return image.Rect(
		frame.Min.X+int(r.X*fw), frame.Min.Y+int(r.Y*fh),
		frame.Min.X+int((r.X+r.W)*fw), frame.Min.Y+int((r.Y+r.H)*fh),
	)
}

// TruthRegion is one annotated interface element.
type TruthRegion struct {
	Kind   TruthKind `json:"kind"`
	Bounds NormRect  `json:"bounds"`
	// Identity ties the SAME element across frames of a sequence.
	//
	// The field that makes temporal correctness measurable rather than guessable. Without
	// it a benchmark can say a detector found a button in frame 3 and a button in frame 4;
	// it cannot say whether they were the same button, which is the entire question.
	Identity string `json:"identity,omitempty"`
	// Nameable overrides the kind's default, for the rare control whose role is known but
	// whose label must not be read.
	Nameable *bool `json:"nameable,omitempty"`
}

// IsNameable resolves the override against the kind's default.
func (r TruthRegion) IsNameable() bool {
	if r.Nameable != nil {
		return *r.Nameable
	}
	return r.Kind.Nameable()
}

// NegativeRegion is a place a detector must NOT find interface.
type NegativeRegion struct {
	Kind   NegativeKind `json:"kind"`
	Bounds NormRect     `json:"bounds"`
}

// FrameTruth is everything declared about one frame.
type FrameTruth struct {
	Schema int    `json:"schema"`
	Frame  string `json:"frame"`
	// Sequence and Index place this frame in an ordered run.
	Sequence string `json:"sequence"`
	Index    int    `json:"index"`
	// InterfacePresent is the frame-level claim. False means every detection anywhere in
	// the frame is a false positive, which is the cheapest and strongest annotation
	// available and the one the v1 manifest already carried in prose.
	InterfacePresent bool             `json:"interface_present"`
	Regions          []TruthRegion    `json:"regions,omitempty"`
	NegativeRegions  []NegativeRegion `json:"negative_regions,omitempty"`

	// IgnoreRegions are areas the BENCHMARK altered — privacy masks, principally.
	//
	// A third category, and it has to be third. The masked band is not interface, because
	// sanitisation destroyed whatever was there; and it is not declared scenery either,
	// because Marco painted it. Scoring a detector against a flat rectangle this project
	// created would measure the sanitiser, not the model.
	//
	// So a detection lying wholly inside one is dropped before scoring: neither credit nor
	// penalty. The first ScreenParser held-out run had no such concept and punished the
	// detector for finding the mask.
	IgnoreRegions []NormRect `json:"ignore_regions,omitempty"`
}

// Key is a frame's identity within a corpus.
//
// SEQUENCE-SCOPED, and that is the whole point. Corpus v2's transition sequences deliberately
// share boundary frames with pause-stable — the same image is a legitimate observation in two
// temporal contexts — but every lookup was keyed by basename, so one annotation silently won
// and the other sequence lost its menu frames entirely. `pause-open` scored 0 true positives
// on frames that plainly contain a menu.
//
// Reusing an image is fine. Reusing an IDENTITY is not.
func (t FrameTruth) Key() string { return t.Sequence + "/" + t.Frame }

// Validate reports every problem with one frame's annotation.
//
// Every problem, not the first: an annotator fixing one error at a time through a benchmark
// run is the slowest possible loop, and this format is meant to be edited by hand.
func (t FrameTruth) Validate() []error {
	var out []error
	if t.Schema != GroundTruthSchema {
		out = append(out, fmt.Errorf("frame %s: schema %d, this build understands %d",
			t.Frame, t.Schema, GroundTruthSchema))
	}
	if strings.TrimSpace(t.Frame) == "" {
		out = append(out, fmt.Errorf("a frame truth names no frame"))
	}
	if strings.TrimSpace(t.Sequence) == "" {
		out = append(out, fmt.Errorf("frame %s belongs to no sequence; temporal metrics "+
			"need an ordered run", t.Frame))
	}
	for i, r := range t.Regions {
		if !r.Kind.Known() {
			out = append(out, fmt.Errorf("frame %s region %d: unknown kind %q",
				t.Frame, i, r.Kind))
		}
		if !r.Bounds.Valid() {
			out = append(out, fmt.Errorf("frame %s region %d: bounds %+v are not a "+
				"normalised rectangle inside the frame", t.Frame, i, r.Bounds))
		}
	}
	for i, n := range t.NegativeRegions {
		if !n.Kind.Known() {
			out = append(out, fmt.Errorf("frame %s negative %d: unknown kind %q",
				t.Frame, i, n.Kind))
		}
		if !n.Bounds.Valid() {
			out = append(out, fmt.Errorf("frame %s negative %d: bounds %+v are not a "+
				"normalised rectangle inside the frame", t.Frame, i, n.Bounds))
		}
	}
	// A frame declaring no interface must not also annotate interface. The two statements
	// cannot both be true, and silently preferring one would make the corpus lie.
	if !t.InterfacePresent && len(t.Regions) > 0 {
		out = append(out, fmt.Errorf("frame %s declares no interface present but annotates "+
			"%d interface region(s)", t.Frame, len(t.Regions)))
	}
	// An annotation inside a mask claims a region sanitisation destroyed. Seven of these
	// survived the first sanitisation pass and depressed ScreenParser's recall on evidence
	// that no longer existed — the exact defect this check exists to make impossible.
	for i, r := range t.Regions {
		for _, ig := range t.IgnoreRegions {
			if !ig.Valid() {
				continue
			}
			if normContainment(r.Bounds, ig) >= AnnotationInsideIgnore {
				out = append(out, fmt.Errorf(
					"frame %s region %d (%s) lies inside an ignore region — sanitisation "+
						"removed what it describes; clip it or delete it",
					t.Frame, i, r.Kind))
			}
		}
	}
	return out
}

// AnnotationInsideIgnore is how much of an annotation must sit inside a mask before the
// annotation is considered destroyed.
//
// 0.5. Below that the element is still substantially visible and clipping is defensible;
// above it, the annotation is describing paint.
const AnnotationInsideIgnore = 0.5

// normContainment is the fraction of `inner` that lies inside `outer`, in normalised space.
func normContainment(inner, outer NormRect) float64 {
	x1 := max(inner.X, outer.X)
	y1 := max(inner.Y, outer.Y)
	x2 := min(inner.X+inner.W, outer.X+outer.W)
	y2 := min(inner.Y+inner.H, outer.Y+outer.H)
	if x2 <= x1 || y2 <= y1 || inner.W <= 0 || inner.H <= 0 {
		return 0
	}
	return ((x2 - x1) * (y2 - y1)) / (inner.W * inner.H)
}

// LoadTruth reads a corpus's ground truth and refuses anything it cannot trust.
//
// Every problem is reported at once rather than the first: a corpus is edited by hand, and
// fixing one annotation error per run is the slowest possible loop.
func LoadTruth(dir string) ([]FrameTruth, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "ground-truth.json"))
	if err != nil {
		return nil, fmt.Errorf("reading the ground truth: %w", err)
	}
	var truths []FrameTruth
	if err := json.Unmarshal(raw, &truths); err != nil {
		return nil, fmt.Errorf("parsing the ground truth: %w", err)
	}
	var problems []error
	for _, t := range truths {
		problems = append(problems, t.Validate()...)
	}
	problems = append(problems, DuplicateKeys(truths)...)
	_, gaps := Sequences(truths)
	problems = append(problems, gaps...)
	if len(problems) > 0 {
		msg := fmt.Sprintf("%d problem(s) in %s:", len(problems), dir)
		for _, p := range problems {
			msg += "\n  " + p.Error()
		}
		return nil, errors.New(msg)
	}
	return truths, nil
}

// DuplicateKeys reports frames whose sequence-scoped identity collides.
//
// Note what this does NOT forbid: the same IMAGE appearing in two sequences. Corpus v2 shares
// boundary frames between pause-stable and the transition sequences deliberately, because one
// picture is a legitimate observation in two temporal contexts. What must never collide is the
// identity a lookup resolves, and before this existed four such collisions silently discarded
// half the menu evidence.
func DuplicateKeys(truths []FrameTruth) []error {
	seen := map[string]bool{}
	var out []error
	for _, t := range truths {
		if seen[t.Key()] {
			out = append(out, fmt.Errorf(
				"duplicate frame identity %q — two annotations resolve to one lookup and "+
					"one of them will be silently discarded", t.Key()))
		}
		seen[t.Key()] = true
	}
	return out
}

// Sequences groups frame truths into ordered runs.
//
// Sorted by index, and gaps are reported rather than closed: a sequence missing frame 4 is a
// sequence somebody trimmed, and temporal metrics computed across the gap would silently
// treat frames 3 and 5 as adjacent.
func Sequences(truths []FrameTruth) (map[string][]FrameTruth, []error) {
	out := map[string][]FrameTruth{}
	for _, t := range truths {
		out[t.Sequence] = append(out[t.Sequence], t)
	}
	var problems []error
	names := make([]string, 0, len(out))
	for name := range out {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		seq := out[name]
		sort.SliceStable(seq, func(i, j int) bool { return seq[i].Index < seq[j].Index })
		out[name] = seq
		for i, t := range seq {
			if t.Index != i {
				problems = append(problems, fmt.Errorf(
					"sequence %q: frame %q has index %d at position %d — a gap or a "+
						"duplicate makes temporal metrics measure the wrong interval",
					name, t.Frame, t.Index, i))
			}
		}
	}
	return out, problems
}
