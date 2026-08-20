package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Watch: seeing what the Director currently believes, and why.
//
// # One state, three readings
//
// This file fetches ONE value — the Director's playbill — and the overlay renders it
// three ways:
//
//	NORMAL       the idle status line. One word, and a sentence when there is one.
//	WATCH        what Marco sees, believes, is learning and needs.
//	DIAGNOSTICS  the evidence underneath what Watch said.
//
// None of them is a separate system and none of them holds state the others do not.
// The overlay does not decide what is recognised, does not score anything, does not form
// a hypothesis and does not compose a sentence: `view.Watch()` and `view.Normal()` come
// from the shared package, so this surface and `marco director watch` cannot disagree.
//
// # Why polling this is safe
//
// `marco director watch` reads state the service already holds. It starts no observation,
// takes no sample, attaches no provider, runs no OCR and no vision pass, and forms no
// interpretation. A panel refreshing twice a second therefore cannot perturb the session
// it describes — which is the property that makes a live view possible at all on a system
// where looking changes what is seen.
//
// # Why NORMAL polls and the panels do not
//
// The one-line reading runs whenever Marco is doing something, because it is the
// consumer surface and a person needs to know Marco has a question without opening a
// panel. It is deliberately slow. WATCH and DIAGNOSTICS poll faster and only while open,
// because each refresh spawns marco.exe and a background poll for a panel nobody is
// looking at is pure cost.

const (
	// watchInterval is the refresh rate while a panel is open. A perception cycle is a
	// few hundred milliseconds, so this is comfortably slower than the thing it reports
	// and fast enough to feel live.
	watchInterval = 1500 * time.Millisecond
	// headlineInterval is the background rate for the one-line reading. Slow on purpose:
	// it exists so a pending question is never invisible, not so the HUD animates.
	headlineInterval = 4 * time.Second
)

// fetchPlaybill runs the thin client and decodes one account.
//
// cursor is the highest moment already rendered, so the service returns only what is new.
// deep asks for the diagnostics section, which is opt-in precisely so that watching costs
// nothing extra.
func fetchPlaybill(deep bool, cursor uint64) playbill.View {
	sub := "watch"
	if deep {
		sub = "diagnose"
	}
	cmd := exec.Command(marcoBin(), "director", sub, "--json",
		fmt.Sprintf("--cursor=%d", cursor))
	out, err := cmd.Output()
	// A non-zero exit is EXPECTED when the Director is not running, and the payload is
	// still a valid account saying so. The output is therefore decoded before the error
	// is judged: "Marco is asleep" and "marco.exe is missing" want different things done
	// about them, and only the second is a fault.
	var v playbill.View
	if jsonErr := json.Unmarshal(out, &v); jsonErr != nil {
		if err != nil {
			return playbill.Unavailable(playbill.Unreachable,
				"the engine did not answer — is marco.exe on the path?")
		}
		return playbill.Unavailable(playbill.Unreachable, "the engine sent something I can't read")
	}
	return v
}

// watchFeed is the overlay's own bounded memory of the timeline.
//
// The Director publishes only MATERIAL changes, so this appends what arrived and drops
// the oldest. It never invents an entry and never infers one from a difference between
// polls — an inference over a gap is how a panel ends up confidently showing something
// that was withdrawn.
type watchFeed struct {
	moments []playbill.Moment
	cursor  uint64
	epoch   string
	// missed is set when the log rolled past the cursor. Reported rather than papered
	// over: dropped overlay frames are acceptable, dropped Director moments are not.
	missed bool
}

// watchFeedMax bounds what the overlay keeps.
//
// Bounded here as well as in the playbill, deliberately. The playbill's bound is what
// crosses the wire; this one is what a long-running overlay accumulates, and a surface
// that trusted somebody else's bound would grow forever the day that bound changed.
const watchFeedMax = 40

// absorb folds one account's moments into the feed and advances the cursor.
//
// Three cases, told apart by the payload rather than by guessing:
//   - a different epoch means the Director RESTARTED. Sequence numbers begin again, so
//     the cursor is reset rather than compared, and the feed says so — a restart shown
//     as a quiet moment is stale certainty.
//   - an oldest newer than cursor+1 means the log rolled PAST us and moments were lost.
//   - otherwise the slice is simply what is new, possibly empty.
func (f *watchFeed) absorb(v playbill.View) {
	if !v.Reach.Live() {
		return
	}
	if v.Epoch != f.epoch {
		if f.epoch != "" {
			f.moments = append(f.moments, playbill.Moment{
				At: time.Now(), Says: "— Marco restarted —", Tone: playbill.Doubt})
		}
		f.epoch, f.cursor, f.missed = v.Epoch, 0, false
	}
	if f.cursor > 0 && v.Oldest > f.cursor+1 {
		f.missed = true
		f.moments = append(f.moments, playbill.Moment{
			At: time.Now(), Says: "— I missed some of what happened —", Tone: playbill.Doubt})
	}
	for _, m := range v.Recent {
		if m.Seq != 0 && m.Seq <= f.cursor {
			continue
		}
		f.moments = append(f.moments, m)
		if m.Seq > f.cursor {
			f.cursor = m.Seq
		}
	}
	if v.Cursor > f.cursor {
		f.cursor = v.Cursor
	}
	if len(f.moments) > watchFeedMax {
		f.moments = f.moments[len(f.moments)-watchFeedMax:]
	}
}

// merged returns the account with the overlay's accumulated timeline in place of the
// slice this poll happened to carry.
//
// The rendering then shows the recent past rather than only the last 1.5 seconds, and it
// shows it in the Director's own words. Nothing else about the account is touched.
func (f *watchFeed) merged(v playbill.View) playbill.View {
	v.Recent = append([]playbill.Moment(nil), f.moments...)
	return v
}

// pollWatch refreshes whichever panel is open, and sleeps cheaply while none is.
//
// Polling stops the moment the panel closes. The Director neither knows nor cares whether
// anybody is watching — a disconnected overlay changes nothing about what it does, which
// is exactly the property an observability surface has to have.
func pollWatch(m *model) {
	f := &watchFeed{}
	for {
		mode := m.watchMode()
		if mode == watchOff {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		v := fetchPlaybill(mode == watchDeep, f.cursor)
		f.absorb(v)
		m.setWatch(f.merged(v))
		time.Sleep(watchInterval)
	}
}

// pollHeadline keeps the one-line consumer reading current while no panel is open.
//
// This is the NORMAL surface, and it is the architectural proof this milestone exists to
// make: the same value that expands into Watch reduces into one word, so the two can
// never disagree about whether Marco has a question. It runs slowly and it stops while a
// panel is open, because the panel is already fetching the same thing.
func pollHeadline(m *model) {
	for {
		if !directorEnabled() || m.watchMode() != watchOff {
			time.Sleep(headlineInterval)
			continue
		}
		v := fetchPlaybill(false, 0)
		m.setHeadline(v.Normal())
		time.Sleep(headlineInterval)
	}
}
