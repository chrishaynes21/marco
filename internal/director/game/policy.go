package game

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
)

// What the framework will and will not automate in an application it has been told about.
//
//	Plugins declare: online game, competitive, protected, automation restrictions.
//	The Director may refuse: repeated combat, aim assistance, movement automation,
//	competitive interaction.
//	Supportive automation is preferred: inventory, crafting, menus, organization,
//	accessibility, reminders.
//	The framework should make these policies explicit rather than hiding them.
//
// This file is that paragraph, enforced. It is a policy.Rule, which means it can refuse an
// action or require agreement for one and can do nothing else — the same seam a pack gets,
// with the framework's own rule placed first so its reason is the one the user sees.
//
// # Why the vocabulary does the work
//
// There is no check here for "is this an aimbot". A rule that tried to recognise combat
// automation by inspecting actions would be a filter, and filters are argued with. Instead
// the Automation vocabulary contains no value for combat, aiming, movement or player
// interaction — so a pack cannot permit one, and an action that does not fall under
// something a pack permitted is refused for that reason alone.
//
// The result is that adding a permission is a change to this file, reviewed here, rather
// than a line in a pack nobody reads.

// SafetyRule returns the framework's own policy rule for the registered packs.
func (r *Registry) SafetyRule() policy.Rule { return &safetyRule{reg: r} }

// safetyRule refuses what the detected application does not permit.
type safetyRule struct {
	reg *Registry
	// Detect supplies the current detection. A function rather than a stored Active,
	// because what is in front changes between requests and a rule holding a snapshot
	// would judge one game by another's declaration.
	//
	// Set by the runtime; nil means detection is unavailable, which is treated as "no
	// pack serves this" — the permissive direction, and correct: an application no pack
	// has declared anything about is an ordinary desktop application, and the Director's
	// own policy is what governs it.
	Detect func() Active
}

func (s *safetyRule) Name() string { return "game safety" }

// SetDetect supplies the source of the current detection.
//
// Set by the composition root, which is the one place that knows both the registry and
// where detection is cached. A rule with none treats every application as one no pack
// serves — the Director's ordinary policy, unchanged — which is the correct behaviour for a
// Director whose caller never wired detection.
func (s *safetyRule) SetDetect(f func() Active) { s.Detect = f }

// Evaluate applies the detected pack's declaration to one action.
func (s *safetyRule) Evaluate(_ context.Context, req policy.Request) policy.Verdict {
	if s == nil || s.Detect == nil {
		return policy.Verdict{}
	}
	active := s.Detect()
	if !active.Detected() {
		// No pack serves this. Not a game as far as the framework is concerned, and the
		// Director's ordinary policy is the whole of what applies.
		return policy.Verdict{}
	}
	safety := active.Safety

	// PROTECTED first, and it is absolute. An application that ships measures against
	// automation has said what it wants; the framework's answer is to refuse, and there
	// is deliberately nothing here that could be configured to do otherwise.
	if safety.Protected {
		return policy.Verdict{
			Refuse: true,
			Reason: fmt.Sprintf(
				"%s is declared as protected against automation, so the Director will not "+
					"act in it. This is not something to configure around.",
				describeApp(active)),
		}
	}

	// Nothing permitted is a refusal, not a default-allow. A pack that declared an
	// application and permitted nothing has said "I recognise this and I do not sanction
	// automation here", which is a useful thing to be able to say.
	if len(safety.Permitted) == 0 {
		return policy.Verdict{
			Refuse: true,
			Reason: fmt.Sprintf(
				"%s is recognised, and %s permits no automation in it%s",
				describeApp(active), active.Pack, note(safety)),
		}
	}

	// COMPETITIVE: the supportive kinds are still permitted, and everything is confirmed.
	// A ranked game is where an automated action affects other people, and the framework's
	// position is that the player should be the one deciding to take it — every time,
	// rather than once when they installed a pack.
	if safety.Competitive {
		return policy.Verdict{
			Confirm: true,
			Reason: fmt.Sprintf(
				"%s is declared competitive. Supportive automation is permitted there, and "+
					"each action is confirmed because it affects other players%s",
				describeApp(active), note(safety)),
		}
	}
	return policy.Verdict{}
}

// describeApp names the application in a refusal.
func describeApp(a Active) string {
	if a.Application != "" {
		return a.Application
	}
	return "this application"
}

// note appends the pack's own sentence, when it wrote one.
func note(s Safety) string {
	if strings.TrimSpace(s.Note) == "" {
		return "."
	}
	return ": " + strings.TrimSuffix(s.Note, ".") + "."
}

// ── procedure-level declaration ───────────────────────────────────────────────

// Permits reports whether the detected pack sanctions a kind of automation.
//
// The question a PROCEDURE asks about itself. A pack's crafting procedure calls this to
// declare what it is, and the framework answers from what the pack declared — so a pack
// that permitted only reminders cannot ship a crafting procedure that runs anyway.
func (r *Registry) Permits(active Active, a Automation) (bool, string) {
	switch {
	case !active.Detected():
		return false, "no capability pack serves what is in front"
	case active.Safety.Protected:
		return false, describeApp(active) + " is declared protected against automation"
	case !a.Supportive():
		return false, fmt.Sprintf("%q is not a kind of automation this framework recognises", a)
	case !active.Safety.Permits(a):
		return false, fmt.Sprintf("%s does not permit %s automation in %s",
			active.Pack, a, describeApp(active))
	}
	return true, ""
}

// ensure the rule is usable as one.
var _ policy.Rule = (*safetyRule)(nil)
