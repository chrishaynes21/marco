package observe

// How much of a window an observation actually reached.
//
// # The distinction this file exists for
//
// [Place] already keeps two failures apart, and says why at its own definition: a screen that
// could not be described at all is not the same fact as one that was described and matched
// nothing, "and collapsing that into 'not recognised' would make 'I could not look'
// indistinguishable from 'I looked and did not know it'."
//
// There is a third state between them, and it was collapsed into the second.
//
// Measured live, on Windows Settings. Marco had the right application, the right window, the
// foreground, and a full-screen frame. What the accessibility provider returned was SIXTEEN
// structures where the same page had been learned with a hundred and forty-eight: the caption
// buttons, a title strip, an account tile — and one 1594x926 rectangle covering the whole content
// area with nothing observed inside it. Settings is a hosted application, and when its content is
// suspended or unpainted the tree collapses to the frame it lives in.
//
// Marco said "I don't recognise this screen". True, and it sent the diagnosis to the page rather
// than to the reading, which is the failure ADR-031 already forbids one level up.
//
// # What this is NOT
//
// It is not a count. 148 → 16 is the evidence that exposed the defect; it is not the rule, and a
// rule built on it would be wrong: a responsive page that collapses its navigation, a window
// resized, an application updated, or a personalised page with fewer cards all legitimately
// change how rich a screen is. 148 → 130 must keep meaning what it means today.
//
// It is not provider-specific either. Nothing here knows what UIA is. The evidence is geometry
// and arrangement — where observed structures are, relative to the space the window occupies —
// which OCR, vision or a fused observer produce in exactly the same terms.

// Reach is how far into a window an observation got.
type Reach string

const (
	// ReachContent is an observation that reached the page. The ordinary case, and the only
	// one on which recognition means anything.
	ReachContent Reach = "content"
	// ReachShell is the window, and an unread space where its content should be.
	//
	// It says nothing about WHICH page is there — that is the point. A shell-only reading
	// cannot support recognising a screen, cannot support failing to recognise one either,
	// and must never become a durable Place: what would be remembered is the frame every
	// page of that application shares.
	ReachShell Reach = "shell"
)

// Vacancy is the evidence behind [ReachShell]: a space the window gives to its content, with
// nothing observed in it.
//
// Kept as evidence rather than reduced to a boolean because the diagnosis is the useful part. A
// person told "I can see Settings but not the page" wants to know that four fifths of the window
// came back empty; a future Fusion deciding whether OCR is worth running wants the same fact.
type Vacancy struct {
	// Region is the empty space, normalised to the window. Share is how much of the window
	// it covers.
	Region Region
	Share  float64
	// Inside is how many observed structures fall within it, and Structures how many the
	// whole observation found. The RATIO is the discriminator — see [ReachOfState].
	Inside     int
	Structures int
}

// Found reports whether there is a vacancy to talk about.
func (v Vacancy) Found() bool { return v.Share > 0 }

// The bounds of the judgement, and each one is a statement about what it would be wrong to say.
const (
	// minVacantShare is how much of a window a space must occupy before its emptiness says
	// anything. Below this it is a panel, a sidebar, a preview pane — an ordinary empty
	// region on a page that was read perfectly well.
	minVacantShare = 0.40
	// maxVacantOccupancy is the share of an observation's structures that may lie inside
	// that space before it stops being empty.
	//
	// A RATIO, not a count, and that is the whole of why this does not fire on a small
	// dialog. A message box with one button puts a large fraction of what it has inside its
	// body; Settings with its content missing put one structure of sixteen inside a region
	// covering three quarters of the window. The first is a sparse page read correctly; the
	// second is a page not read at all, and no absolute number separates them.
	maxVacantOccupancy = 0.10
	// minStructuresToJudge is how much of an observation there must be before this is
	// willing to call anything shell-only.
	//
	// Under this there is not enough to reason about arrangement: two structures inside a
	// third tells you nothing about whether a page was read. An observation that thin is
	// already handled — it does not settle, so it is not placeable, and `Placed` says so.
	// Refusing to judge is the honest answer, and it keeps the smallest legitimate windows
	// out of reach of this rule entirely.
	minStructuresToJudge = 6
	// panelShare is how big an occupied space has to be to count as a PANEL -- somewhere in
	// the window that has things in it.
	//
	// Smaller than minVacantShare on purpose. The question it answers is not "is this the
	// content area" but "is there anywhere in this window with anything in it", and one
	// populated panel is enough to say the reading worked. A list beside a blank reading
	// pane, a project tree beside an empty editor: those windows were read, and one part of
	// them happens to be empty.
	panelShare = 0.15
)

// ReachOfState judges what an observation of one screen state actually reached.
//
// # The rule, and why it is arrangement rather than richness
//
// A window that is showing its content has that content spread through the space the window gives
// it. A window whose content did not come back has the same space — the container is still
// reported, at full size — and nothing in it.
//
// So: find the largest space any observed structure claims. If it covers a serious share of the
// window and almost nothing else was observed INSIDE it, the page is present and unread.
//
// Every part of that is available to any observer that reports what it saw and where. None of it
// asks how many controls a page ought to have, which is knowledge this layer does not have and
// should not acquire.
//
// # What it refuses to decide
//
// A thin observation is not judged at all. Neither is one with no dominant space. Both return
// [ReachContent], which means "nothing here says the page went unread" rather than "the page was
// read well" — the caller's existing checks still have to pass.
//
// Deleting the occupancy ratio, or replacing it with a count, must fail
// TestASparseWindowIsNotADegradedOne.
func ReachOfState(t ShadowTotals, id ScreenStateID) (Reach, Vacancy) {
	tracks := tracksInState(t, id)
	if len(tracks) < minStructuresToJudge {
		return ReachContent, Vacancy{}
	}

	var empty Vacancy
	var fullest float64
	for _, candidate := range tracks {
		share := area(candidate.Reference)
		inside := 0
		for _, other := range tracks {
			if other.ID == candidate.ID {
				continue
			}
			x, y := centre(other.Reference)
			if contains(candidate.Reference, x, y) {
				inside++
			}
		}
		if float64(inside)/float64(len(tracks)) > maxVacantOccupancy {
			// A SPACE WITH THINGS IN IT. Remembered, because one of these is what
			// separates a page from a frame — see the populated-panel rule below.
			if share > fullest {
				fullest = share
			}
			continue
		}
		if share >= minVacantShare && share > empty.Share {
			empty = Vacancy{Region: candidate.Reference, Share: share,
				Inside: inside, Structures: len(tracks)}
		}
	}
	if !empty.Found() {
		return ReachContent, Vacancy{}
	}

	// A WINDOW WITH A POPULATED PANEL WAS READ, whatever else in it is empty.
	//
	// Emptiness alone is not enough, and this is the case that proves it: a mail client with
	// nothing selected shows a full list beside a blank reading pane, and the blank pane is
	// large. So is an editor with no file open beside its project tree. Those windows were
	// read perfectly well; one part of them simply has nothing in it, which is a fact about
	// the application rather than about the reading.
	//
	// What the live Settings failure had that none of those has is NOWHERE with anything in
	// it. Every structure observed was window furniture around the edge, and every space big
	// enough to be a panel came back blank.
	//
	// Deleting this must fail TestASparseWindowIsNotADegradedOne's empty-panel case.
	if fullest >= panelShare {
		return ReachContent, Vacancy{}
	}
	return ReachShell, empty
}

// tracksInState is the structures one screen state was made of.
//
// Scoped to the state on purpose. A session that crossed several screens holds tracks from all of
// them, and judging a page by structures observed somewhere else would be the stale-evidence
// mistake this repository has already made twice.
func tracksInState(t ShadowTotals, id ScreenStateID) []ShadowTrack {
	if id == "" || id == ScreenStateUnknown {
		return nil
	}
	var out []ShadowTrack
	for _, tr := range t.Tracks {
		for _, st := range tr.States {
			if st.State == id && st.Seen > 0 {
				out = append(out, tr)
				break
			}
		}
	}
	return out
}

// area is a normalised region's share of its window.
func area(r Region) float64 {
	if r.Width <= 0 || r.Height <= 0 {
		return 0
	}
	return r.Width * r.Height
}

// centre is a region's middle point, in the same normalised space.
func centre(r Region) (x, y float64) {
	return r.X + r.Width/2, r.Y + r.Height/2
}

// contains reports whether a point falls inside a region. Half-open on purpose: a structure
// flush against a container's far edge belongs to it.
func contains(r Region, x, y float64) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
