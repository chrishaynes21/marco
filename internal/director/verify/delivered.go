package verify

import "github.com/chaynes-simpleclouds/marco/pkg/directorapi"

// The evidence kinds that report DELIVERY — the action reached the control Marco aimed at —
// rather than EFFECT, which is what the application then did about it.
//
// Named constants rather than literals at each site, because two different consumers now read
// them for two different purposes and a typo in either would be silent:
//
//	the verdict      weighs them at WeightDelivered, so they corroborate and never conclude
//	the retry guard  refuses to repeat an action one of them says already arrived
//
// The second is the safety one. See [Delivered].
const (
	// EvidenceFocusOnTarget is keyboard focus sitting on the element that was clicked.
	EvidenceFocusOnTarget = "focus_on_target"
	// EvidenceSelectionState is a control's own word about whether it is selected. Measured
	// against a live provider that said yes while the application held no selection at all.
	EvidenceSelectionState = "selection_state"
)

// Delivered reports whether a verification result holds evidence that the action REACHED its
// target, whatever the application did about it afterwards.
//
// # Why a retry needs this and "did the world change?" is not enough
//
// The retry guard permits a second attempt only when the world is byte-for-byte what it was,
// reasoning that nothing can have happened and so nothing can be done twice. That inference is
// unsound, and the guard's own note says so in capitals: FAILURE TO CONFIRM IS NOT PROOF THAT
// NOTHING HAPPENED. An application may act on a press while moving nothing a screen can show —
// a counter nobody displays, a message sent, a state it was already in.
//
// Measured: the acceptance fixture's Save button counts presses and writes a status label.
// Pressed a second time, the label already held the value it would be set to, so not one pixel
// and not one element differed. The guard allowed a retry, Marco pressed Save again, and the
// application counted both.
//
// Delivery evidence answers the question the retry actually turns on. [WeightDelivered]'s note
// puts it exactly: this evidence "is what separates 'the button did nothing' from 'Marco clicked
// the wrong thing', and those send a reader to two different places". A retry is for the second.
// If the click reached the control, re-observing finds the same control in the same place and
// re-delivers the same action — all of the risk and none of the recovery.
//
// It does NOT report whether the action worked. An action can be delivered and inert, which is
// the whole reason these kinds weigh below MinConfidence.
func Delivered(r directorapi.VerificationResult) bool {
	for _, e := range r.Evidence {
		if !e.Observed {
			continue
		}
		switch e.Kind {
		case EvidenceFocusOnTarget, EvidenceSelectionState:
			return true
		}
	}
	return false
}
