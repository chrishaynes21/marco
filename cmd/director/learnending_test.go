package main

import "testing"

// One word, two of Marco's own surfaces, opposite consequences.
//
// `director learn --stop` sent Cancel — abandon, keep nothing. The Stop button on Marco's control
// centre sent Stop, which the service reads as FINISH — keep everything and run the pipeline.
// Somebody who learned the word in one place lost their demonstration in the other, and the loss
// would have looked like success.
//
// The settlement is that "stop" is the ABORT word, globally: a bare stop during a Learn episode
// means cancel, never finish. ADR-066's two operations both survive unchanged; what is fixed is
// that the ambiguous word is bound to exactly one of them, and the other gets an honest name.

// STOP ON THE COMMAND LINE STILL MEANS CANCEL.
//
// The alias is the compatibility promise: `--stop` has meant abandon on this command line since it
// shipped, so a script that says it must keep abandoning. Removing the alias, or quietly
// repointing it at Finish to "match the button", both fail here.
func TestStopOnTheCommandLineStillMeansCancel(t *testing.T) {
	abandon, keep := learnEnding(false, false, true)
	if !abandon {
		t.Error("--stop no longer cancels. Every script and every finger that learned this " +
			"flag now does something else, and what it does is keep a demonstration " +
			"the person asked to throw away.")
	}
	if keep {
		t.Error("--stop asked to FINISH the demonstration. \"stop\" is the abort word.")
	}
}

// THE TWO ENDINGS ARE STILL TWO.
//
// ADR-066: Cancel throws the attempt away and keeps nothing, Finish is the reason the attempt
// exists. Collapsing them — in either direction — is the failure the ADR names, and the honest
// names exist so nobody has to remember which is which.
func TestCancelAndFinishStayApart(t *testing.T) {
	if abandon, keep := learnEnding(true, false, false); !abandon || keep {
		t.Errorf("--cancel produced abandon=%v keep=%v, want abandon only", abandon, keep)
	}
	if abandon, keep := learnEnding(false, true, false); abandon || !keep {
		t.Errorf("--finish produced abandon=%v keep=%v, want keep only", abandon, keep)
	}
	if abandon, keep := learnEnding(false, false, false); abandon || keep {
		t.Errorf("no ending flag produced abandon=%v keep=%v, want neither", abandon, keep)
	}
}

// A CONTRADICTION IS READ AS THE ENDING THAT CANNOT BE UNDONE WRONGLY.
//
// Somebody who typed the abort word and the keep word together has said two opposite things. The
// safe reading is the one that acts least: nothing is kept, and they can demonstrate again. Keeping
// a demonstration that may have been meant for the bin writes a play, registers it, and makes it
// askable — none of which the person can be assumed to have wanted.
func TestAskingToCancelAndFinishAtOnceCancels(t *testing.T) {
	abandon, keep := learnEnding(true, true, false)
	if !abandon || keep {
		t.Errorf("--cancel --finish together produced abandon=%v keep=%v, want cancel to win",
			abandon, keep)
	}
}
