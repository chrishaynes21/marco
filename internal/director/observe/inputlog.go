package observe

// What the person actually did, retained whether or not Marco understood it.
//
// # The invariant this file exists to hold
//
// Attributed human input is never discarded because a LATER layer could not interpret the
// world around it. A click Marco could place inside the watched window is evidence that the
// person clicked there — whatever the screen state was, whether a checkpoint formed, whether
// a transition became durable, and whether any semantic target ever resolved. Interpretation
// failure is not capture failure.
//
// The rest of the substrate keeps its own rules on purpose: `pendingInputs` is an ATTRIBUTION
// buffer and expires by design (a keypress from a minute ago must not be credited to a change
// it did not precede), a capture banks only what its own lifecycle admits, and a transition
// keeps only the runs seen before it. Each of those is a claim about what an input MEANT.
// This log claims nothing — it is the record that the input HAPPENED, in order, with whatever
// context was known at the time.
//
// # What it may contain
//
// Exactly what already crosses the navigation boundary: closed-vocabulary intents, a
// session-relative timestamp, a window-relative pointer position, and the session-local
// screen-state counter that was current when the event was banked. No key, no text, no label,
// no absolute coordinate — the log is a different retention policy over the same admitted
// record, not a wider one.
//
// Session-scoped like everything else on ShadowTotals: it is evidence about one watched
// session and nothing here is written to the durable store.

// AttributedInput is one admitted input event, with the context known at banking time.
type AttributedInput struct {
	Event InputEvent `json:"event"`
	// Inference is how many valid inferences had run when the event was banked. Zero means
	// the person acted before anything had been observed at all — which is still evidence,
	// and is the case the log most exists for.
	Inference int `json:"inference,omitempty"`
	// State is the session-local screen state that was current when the event was banked,
	// empty when no valid inference had placed the screen yet. It is the state the person
	// was ON when they acted, as far as anything knew at the time; the state their action
	// produced is the next entry's context, or a transition's.
	State ScreenStateID `json:"state,omitempty"`
}

// MaxInputLog bounds the retained events for one session.
//
// Generous against the demonstration bound (sixty semantic events) and small against a hook
// stream: the log answers "what did the person do during this pass", not "everything since
// boot". Overflow drops the OLDEST and is counted, so a capped log never reads as complete.
const MaxInputLog = 256

// InputLog is the ordered, bounded record of every admitted input event in one session.
type InputLog struct {
	Events []AttributedInput `json:"events,omitempty"`
	// Dropped counts events the bound pushed out. Non-zero means the log holds the most
	// recent MaxInputLog events, not the whole session.
	Dropped int `json:"dropped,omitempty"`
}

// Empty reports whether nothing was ever banked.
func (l InputLog) Empty() bool { return len(l.Events) == 0 && l.Dropped == 0 }

// bank appends admitted events with the context current at the time.
//
// Called BEFORE any structural gate, on every fold — including slots nothing looked at and
// samples nothing could place. That placement is the whole guarantee, and it is why this is a
// separate call rather than a read of `pendingInputs`: the attribution buffer expires and is
// consumed, and the log must survive both.
func (l *InputLog) bank(events []InputEvent, inference int, state ScreenStateID) {
	for _, e := range events {
		if !e.Intent.Known() {
			continue
		}
		l.Events = append(l.Events, AttributedInput{
			Event: e, Inference: inference, State: state,
		})
	}
	if over := len(l.Events) - MaxInputLog; over > 0 {
		l.Dropped += over
		l.Events = append(l.Events[:0:0], l.Events[over:]...)
	}
}
