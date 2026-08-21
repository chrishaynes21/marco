package main

import (
	"net/http"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The control centre's ACTIVITY surface: what Marco has done, and how each one ended.
//
// # Why this reads two accounts and writes none
//
// There was no Activity surface at all. The nearest thing was the HUD's log, which is in-memory,
// capped, and gone the moment the overlay restarts — so "what did Marco do this morning" had no
// answer anywhere in the product, while the Director had been writing every action to a durable
// action graph the whole time.
//
// The tempting fix is a store. It would be wrong: it would be a SECOND durable record of what
// Marco has done, kept by a web page, drifting away from the graph the moment either changed —
// and this campaign has spent four phases removing exactly that shape (two intakes, two stops,
// two accounts of one run). So this endpoint owns nothing. It asks the two places that already
// know:
//
//	the Director's action graph   what `marco director history` reads; durable across restarts
//	this session's runAccount     what became of the Runs clicked on this page, bounded to 5min
//
// and merges them into one list, newest first.
//
// # What is kept in process, and why it is allowed
//
// Nothing new. `runAccount` already existed for the Run button (see runaccount.go), it is already
// bounded by time rather than by count, and it is already explicitly not a history. Reading it
// here adds no storage; it makes the runs a person started from this page visible beside the ones
// the Director recorded, which is the only reason this page can answer honestly on a machine where
// the Director is not running at all.
//
// # It never starts anything
//
// `directorConnect(false)`. Opening a tab is a read, and a read may not pay for a service start —
// the reasoning is written out at cmd/marco/intake.go's `pendingQuestion`, and the Learn surface's
// wake button is the one place on this page where a person can choose to pay it.

// activityRow is one thing Marco did, as the page is shown it.
//
// Every word is settled here rather than in the page: the six outcome words come out of
// [outcome], so this surface and the HUD cannot describe one ending differently.
type activityRow struct {
	// When is a short local clock time — the page shows a list, not a forensic log.
	When string `json:"when"`
	// What is the person's own words for it where there are any, and the goal otherwise.
	What string `json:"what"`
	// Outcome is one of the six, always. Never empty, never a seventh word.
	Outcome string `json:"outcome"`
	// Detail is the reason Marco gave, when it gave one.
	Detail string `json:"detail,omitempty"`
	// From says which account the row came out of: "marco" for the Director's own record,
	// "here" for a Run clicked on this page. A person who just pressed Run and saw the row
	// twice would otherwise think it had happened twice.
	From string `json:"from"`
	// at is the sort key. Unexported: the page sorts nothing, it renders the order it is given.
	at time.Time
}

// directorHistoryEntries reads the Director's account of what it has done.
//
// A package variable for the same reason `pendingQuestion` and `runSpawn` are: a test must be able
// to exercise this surface without a socket, a service, or any chance of reaching the Director
// running on the developer's own desktop. Production never reassigns it.
var directorHistoryEntries = readDirectorHistory

// activityLimit is how far back the surface looks. Twenty is what `marco director history` shows
// without an argument, and matching it means the tab and the command agree about "recent".
const activityLimit = 20

func readDirectorHistory(limit int) ([]service.HistoryEntry, error) {
	c, err := directorConnect(false)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	resp, err := c.History(limit)
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

// handleActivity answers "what has Marco done".
//
// A read, and it says which of the two empties it is when there is nothing: "Marco has not done
// anything yet" and "Marco is not watching, so it has no record to show" are different facts, and
// a surface that rendered one blank list for both would be guessing on the person's behalf.
//
// Deleting the runAccount arm must fail TestActivityShowsRunsStartedFromThisPage.
func (e *editor) handleActivity(w http.ResponseWriter, _ *http.Request) {
	rows := []activityRow{}
	entries, err := directorHistoryEntries(activityLimit)
	for _, en := range entries {
		rows = append(rows, activityRow{
			When:    clockOf(en.Timestamp),
			What:    activityWhat(en),
			Outcome: string(activityOutcome(en)),
			Detail:  en.Reason,
			From:    "marco",
			at:      en.Timestamp,
		})
	}
	for _, r := range e.runs.recent() {
		// A run still going has no ending yet, and inventing one would be the exact defect
		// runaccount.go exists to have fixed. It is simply not a row until it finishes.
		if !r.Done {
			continue
		}
		rows = append(rows, activityRow{
			When:    clockOf(r.Started),
			What:    prettyRun(r),
			Outcome: string(r.Outcome),
			Detail:  r.Detail,
			From:    "here",
			at:      r.Started,
		})
	}
	// Newest first, and stable: two rows in the same second keep the order they arrived in.
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].at.After(rows[j].at) })

	why := ""
	if len(rows) == 0 {
		why = "Marco has not done anything yet."
		if err != nil {
			why = "Marco is not watching right now, so there is nothing it can tell you about " +
				"earlier. Runs you start from this page will show up here."
		}
	}
	writeJSON(w, map[string]any{"entries": rows, "why": why})
}

// recent is the Activity surface's read of the runs this control centre started.
//
// It lives here rather than beside the rest of runAccount because this is its only caller and
// because the account itself is deliberately not a history — runaccount.go says so at length.
// Reading it does not make it one: it is still bounded to five minutes, still swept, and still
// never written to disk.
func (a *runAccount) recent() []runRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]runRecord, 0, len(a.runs))
	for _, r := range a.runs {
		out = append(out, *r)
	}
	return out
}

// clockOf is a timestamp as a person reads one: the time of day, local.
//
// Not a duration ("4m ago") because the page does not tick, and a relative time that is only
// correct at the instant it was rendered is worse than an absolute one that is always correct.
func clockOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("15:04:05")
}

// activityWhat is what to call one of the Director's actions.
//
// The person's own phrase first, because that is what they would recognise; the goal next, because
// a replay or a hotkey has no phrase; and the label of whatever was acted on last, because a row
// with no words at all is a row that answers nothing.
func activityWhat(en service.HistoryEntry) string {
	for _, s := range []string{en.Phrase, en.Goal, en.Label} {
		if s != "" {
			return s
		}
	}
	return "something Marco did"
}

// activityOutcome reads one action's ending into the ONE vocabulary.
//
// # Why `unverified` is `failed` here
//
// Because that is already the engine's reading of it, and a second reading would be a second
// account. `cmd/marco/director.go` returns `exitNotVerified` for anything that is not
// `CommandCompleted`, and `outcomeOfDirectorExit` in intake.go maps that exit to `failed`. So a
// person who ran a phrase from the command line and a person looking at this list see the same
// word for the same action. Calling it `performed` here would be this surface deciding, on its
// own, that an unconfirmed action counts as done — which is the claim outcome.Performed exists to
// refuse.
//
// A status this does not recognise falls back to the entry's own Success flag rather than to a
// guess, so a status added upstream reads as failed rather than as a blank.
//
// Deleting the ActionBlocked arm must fail TestActivityRendersTheSixOutcomeWords.
func activityOutcome(en service.HistoryEntry) outcome.Outcome {
	switch directorapi.ActionStatus(en.Status) {
	case directorapi.ActionSucceeded:
		return outcome.Performed
	case directorapi.ActionCancelled:
		return outcome.Cancelled
	case directorapi.ActionBlocked:
		// Refused by policy. A door saying no is not a thing going wrong, and the whole reason
		// the vocabulary separates `refused` from `failed` is so a person can tell them apart.
		return outcome.Refused
	case directorapi.ActionPending, directorapi.ActionRunning:
		// It never reached an ending. Nothing delivered a verdict, which is what `unavailable`
		// means — not that it went wrong.
		return outcome.Unavailable
	case directorapi.ActionFailed, directorapi.ActionUnverified:
		return outcome.Failed
	}
	if en.Success {
		return outcome.Performed
	}
	return outcome.Failed
}
