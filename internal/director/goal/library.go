package goal

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The procedure library: every goal's expansion, written by hand.
//
//	These are typed procedures. Do not generate them with an LLM.
//
// Each is a small, readable sequence of semantic directives, and the reason they are
// hand-written rather than generated is the same reason the ladders in uiact are: a
// procedure is a claim about what an application does, and a generated claim cannot be
// reviewed before it acts. A wrong ladder rung picks a weaker mechanism; a wrong
// procedure presses the wrong thing.
//
// Every directive below is a SEMANTIC action or a focus or an edit. None names a
// coordinate, a keystroke or a pattern — those are chosen further down, by the
// capability ladder, against the control that is actually there.

// low, medium and high are the risk levels, named here so a procedure reads as a
// paragraph rather than as a struct literal full of package qualifiers.
const (
	low    = directorapi.RiskLow
	medium = directorapi.RiskMedium
	high   = directorapi.RiskHigh
)

// library is the built-in procedures.
func library() []Procedure {
	return []Procedure{
		genericRename(), explorerRename(), vscodeRenameSymbol(),
		genericCreateFolder(), explorerCreateFolder(),
		genericSave(), genericSaveAs(), genericPrint(),
		genericCloseWithoutSaving(),
		genericDuplicate(), genericDelete(),
		genericOpenFile(), genericOpenSettings(),
		genericCreateTab(), genericDownload(),
		genericMove(), genericCopy(), genericPaste(),
		genericSort(), genericCraft(),
	}
}

// ── rename ────────────────────────────────────────────────────────────────────

// genericRename is the milestone's worked example.
//
//	Focus target → Invoke Rename → wait until editable → set value → confirm
//
// The wait is NOT a step. It is a precondition on the step that needs it, which the
// program layer evaluates as a semantic wait — so the Director waits for an editable
// control to exist rather than for a duration somebody guessed.
func genericRename() Procedure {
	return Procedure{
		Name: "generic rename", Goal: Rename,
		Requires: []Requirement{RequiresTarget, RequiresName},
		// "rename this file" means a FILE. A folder, an unsaved tab, a text selection
		// or a nearby button is refused rather than renamed.
		Expect: binding.KindFile,
		Safety: Safety{
			Mutations: 1, Risk: medium,
			// A rename is undone by renaming back, so it is neither destructive nor
			// irreversible — but it does change something the user would notice, which
			// is why it is not low.
		},
		Why: "renaming requires an editable field, so the procedure invokes the rename " +
			"control and waits for one to appear rather than typing into whatever has focus",
		Steps: func(g Goal) ([]Directive, error) {
			target, pointed := g.Context.Target, g.Context.TargetIsImplicit
			return []Directive{
				{Focus: true, Target: target, TargetDeictic: pointed, Phrase: "focus " + quoted(target)},
				{Semantic: directorapi.SemanticInvoke, Role: RoleRenameCommand,
					Phrase: "invoke the rename command"},
				{SetText: true, Text: g.Param(ParamName),
					Phrase: fmt.Sprintf("set the name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "an editable field has focus",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the new name"},
			}, nil
		},
	}
}

// explorerRename overrides the generic procedure for File Explorer.
//
// Explorer's editable field is the item's OWN LABEL rather than a separate dialog, and the
// item has to be selected before the Rename command applies to anything. That is what
// justifies an override: the generic procedure focuses and invokes, and in Explorer
// focusing a list item is not the same as selecting it.
//
// # It used to open the context menu, and that was a Windows 10 assumption
//
// The first live run got three steps in and stopped at "choose the rename command". Two
// things were wrong with going through the menu, and the live run showed both:
//
//   - Windows 11 puts Rename in the COMMAND BAR, as an AppBarButton with an Invoke
//     pattern and the accessible name "Rename". There is no need to open anything.
//   - A context menu is its own top-level window. The Director walks the window it is
//     acting in, so the menu's contents were not in the tree at all — the step could not
//     have succeeded, and it correctly asked for clarification rather than pressing
//     something that looked close.
//
// So the menu step is gone. The Rename control is named by ROLE, as before, which is what
// keeps this working when the command bar is localised.
func explorerRename() Procedure {
	return Procedure{
		Name: "explorer rename", Goal: Rename,
		Applications: []string{"explorer", "explorer.exe", "File Explorer"},
		Requires:     []Requirement{RequiresTarget, RequiresName},
		Expect:       binding.KindFile,
		Safety:       Safety{Mutations: 1, Risk: medium},
		Why: "Explorer renames in place, editing the item's own label, so the item is " +
			"selected first and the Rename command is invoked from the command bar",
		Steps: func(g Goal) ([]Directive, error) {
			target, pointed := g.Context.Target, g.Context.TargetIsImplicit
			return []Directive{
				{Semantic: directorapi.SemanticSelect, Target: target, TargetDeictic: pointed,
					Phrase: "select " + quoted(target)},
				{Semantic: directorapi.SemanticInvoke, Role: RoleRenameCommand,
					Phrase: "invoke the rename command"},
				// The EDITOR, not "whatever holds focus". A details view has an Edit
				// control per column per row, and the selected row's Name cell holds the
				// filename — typing into it changes a caption and renames nothing.
				{SetText: true, Text: g.Param(ParamName), TargetEditor: true,
					Phrase: fmt.Sprintf("set the name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the item's label is editable",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, TargetEditor: true,
					Phrase: "confirm the new name"},
			}, nil
		},
	}
}

// vscodeRenameSymbol is a DIFFERENT operation that shares the word.
//
// Renaming a symbol in an editor is a refactor across a project, not a change to a
// file's name. It gets its own procedure so the two never silently substitute for each
// other — and its risk is higher for exactly that reason: it edits files the user is
// not looking at.
func vscodeRenameSymbol() Procedure {
	return Procedure{
		Name: "vscode rename symbol", Goal: Rename,
		Applications: []string{"code", "code.exe", "Visual Studio Code"},
		Requires:     []Requirement{RequiresName},
		Safety: Safety{
			Mutations: 1, Risk: high, RequiresConfirmation: true,
			// It rewrites every reference in the project, including in files that are
			// not open. That is not something to do without asking.
		},
		Why: "renaming a symbol rewrites every reference in the project, including in " +
			"files that are not open, so it is confirmed before it runs",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleRenameSymbolCommand,
					Phrase: "invoke the rename-symbol command"},
				{SetText: true, Text: g.Param(ParamName),
					Phrase: fmt.Sprintf("set the new name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the rename box has focus",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the rename"},
			}, nil
		},
	}
}

// ── folders and files ─────────────────────────────────────────────────────────

func genericCreateFolder() Procedure {
	return Procedure{
		Name: "generic create folder", Goal: CreateFolder,
		Requires: []Requirement{RequiresName},
		Safety:   Safety{Mutations: 1, Risk: low},
		Why:      "a new folder is created empty and named, so nothing existing is touched",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleNewFolderCommand,
					Phrase: "invoke the new-folder command"},
				{SetText: true, Text: g.Param(ParamName),
					Phrase: fmt.Sprintf("set the name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the new folder's name is editable",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the name"},
			}, nil
		},
	}
}

// explorerCreateFolder reaches the same outcome through Explorer's New menu.
func explorerCreateFolder() Procedure {
	return Procedure{
		Name: "explorer create folder", Goal: CreateFolder,
		Applications: []string{"explorer", "explorer.exe", "File Explorer"},
		Requires:     []Requirement{RequiresName},
		Safety:       Safety{Mutations: 1, Risk: low},
		Why:          "Explorer creates a folder from its New menu, then names it in place",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleNewSubmenu, Phrase: "invoke the New menu"},
				{Semantic: directorapi.SemanticChoose, Role: RoleFolderItem, Phrase: "choose the Folder entry"},
				{SetText: true, Text: g.Param(ParamName),
					Phrase: fmt.Sprintf("set the name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the new folder's name is editable",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the name"},
			}, nil
		},
	}
}

func genericOpenFile() Procedure {
	return Procedure{
		Name: "generic open", Goal: OpenFile,
		Requires: []Requirement{RequiresTarget},
		Expect:   binding.KindFile,
		Safety:   Safety{Mutations: 0, Risk: low},
		Why:      "opening shows something; it changes nothing",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticOpen, Target: g.Context.Target, TargetDeictic: g.Context.TargetIsImplicit,
					Phrase: "open " + quoted(g.Context.Target)},
			}, nil
		},
	}
}

func genericDuplicate() Procedure {
	return Procedure{
		Name: "generic duplicate", Goal: Duplicate,
		Requires: []Requirement{RequiresTarget},
		Expect:   binding.KindFile,
		Safety:   Safety{Mutations: 1, Risk: low},
		Why:      "duplicating adds a copy and leaves the original alone",
		Steps: func(g Goal) ([]Directive, error) {
			target, pointed := g.Context.Target, g.Context.TargetIsImplicit
			return []Directive{
				{Semantic: directorapi.SemanticSelect, Target: target, TargetDeictic: pointed,
					Phrase: "select " + quoted(target)},
				{Semantic: directorapi.SemanticCopy, Phrase: "copy it"},
				{Semantic: directorapi.SemanticPaste, Phrase: "paste the copy"},
			}, nil
		},
	}
}

// genericDelete is the one procedure that always asks.
func genericDelete() Procedure {
	return Procedure{
		Name: "generic delete", Goal: Delete,
		Requires: []Requirement{RequiresTarget},
		Expect:   binding.KindFile,
		Safety: Safety{
			Mutations: 1, Destructive: true, Irreversible: true,
			RequiresConfirmation: true, Risk: high,
		},
		Why: "deleting removes something, and whether it is recoverable depends on the " +
			"application rather than on anything the Director can see, so it always asks",
		Steps: func(g Goal) ([]Directive, error) {
			target, pointed := g.Context.Target, g.Context.TargetIsImplicit
			return []Directive{
				{Semantic: directorapi.SemanticSelect, Target: target, TargetDeictic: pointed,
					Phrase: "select " + quoted(target)},
				{Semantic: directorapi.SemanticInvoke, Role: RoleDeleteCommand,
					Phrase: "invoke the delete command"},
				// A delete frequently raises its own confirmation. Best effort because
				// an application that deletes without asking is not a failure.
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the deletion",
					BestEffort: true},
			}, nil
		},
	}
}

func genericMove() Procedure {
	return Procedure{
		Name: "generic move", Goal: Move,
		Requires: []Requirement{RequiresTarget, RequiresDestination},
		Expect:   binding.KindFile,
		Safety:   Safety{Mutations: 1, Risk: medium},
		Why:      "a move is a cut and a paste, so the item is gone from where it was",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticSelect, Target: g.Context.Target, TargetDeictic: g.Context.TargetIsImplicit,
					Phrase: "select " + quoted(g.Context.Target)},
				{Semantic: directorapi.SemanticCut, Phrase: "cut it"},
				{Semantic: directorapi.SemanticOpen, Target: g.Param(ParamDestination),
					Phrase: "open " + quoted(g.Param(ParamDestination))},
				{Semantic: directorapi.SemanticPaste, Phrase: "paste it there"},
			}, nil
		},
	}
}

func genericCopy() Procedure {
	return Procedure{
		Name: "generic copy", Goal: Copy,
		Requires: []Requirement{RequiresTarget},
		Expect:   binding.KindFile,
		Safety:   Safety{Mutations: 0, Risk: low},
		Why:      "copying reads; the clipboard is the only thing that changes",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticSelect, Target: g.Context.Target, TargetDeictic: g.Context.TargetIsImplicit,
					Phrase: "select " + quoted(g.Context.Target)},
				{Semantic: directorapi.SemanticCopy, Phrase: "copy it"},
			}, nil
		},
	}
}

func genericPaste() Procedure {
	return Procedure{
		Name: "generic paste", Goal: Paste,
		Safety: Safety{Mutations: 1, Risk: medium},
		Why:    "pasting inserts whatever is on the clipboard into the focused control",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticPaste, Phrase: "paste the clipboard"},
			}, nil
		},
	}
}

// ── documents ─────────────────────────────────────────────────────────────────

func genericSave() Procedure {
	return Procedure{
		Name: "generic save", Goal: Save,
		Safety: Safety{Mutations: 1, Risk: low},
		Why:    "saving writes what is already on screen; nothing is chosen or discarded",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleSaveCommand, Phrase: "invoke the save command"},
			}, nil
		},
	}
}

func genericSaveAs() Procedure {
	return Procedure{
		Name: "generic save as", Goal: SaveAs,
		Requires: []Requirement{RequiresName},
		Safety: Safety{
			Mutations: 1, Risk: medium,
			// It can overwrite an existing file, and the Director cannot see whether
			// the name is taken until the application says so.
		},
		Why: "saving under a new name opens a dialog, so the name is set there and " +
			"confirmed rather than typed into the document",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleSaveAsCommand,
					Phrase: "invoke the save-as command"},
				{SetText: true, Text: g.Param(ParamName),
					Phrase: fmt.Sprintf("set the file name to %s", quoted(g.Param(ParamName))),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the save dialog's name field has focus",
					}},
				},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the save"},
			}, nil
		},
	}
}

func genericPrint() Procedure {
	return Procedure{
		Name: "generic print", Goal: Print,
		Safety: Safety{
			Mutations: 1, External: true, Irreversible: true,
			RequiresConfirmation: true, Risk: high,
			// Printing leaves the machine. Paper cannot be un-printed, which is the
			// definition of irreversible that matters here.
		},
		Why: "printing produces something outside the computer, so it is confirmed first",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RolePrintCommand, Phrase: "invoke the print command"},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the print",
					BestEffort: true},
			}, nil
		},
	}
}

// genericCloseWithoutSaving is the milestone's other worked example.
//
//	Close window → if a confirmation appears → choose Don't Save → verify closed
//
// The conditional is the interesting part, and it is NOT expressed as a branch: the
// program layer has no branches, deliberately. It is a BEST-EFFORT step instead — the
// dialog either appeared, in which case Don't Save is chosen, or it did not, in which
// case the step verifies inconclusively and the program continues. That is the honest
// shape: the Director cannot know in advance whether the document is dirty.
func genericCloseWithoutSaving() Procedure {
	return Procedure{
		Name: "generic close without saving", Goal: CloseWithoutSaving,
		Safety: Safety{
			Mutations: 1, Destructive: true, Irreversible: true,
			RequiresConfirmation: true, Risk: high,
			// Discarding unsaved work is exactly the thing no ordinary action undoes.
		},
		Why: "the save prompt may or may not appear depending on whether the document " +
			"is dirty, so choosing Don't Save is a best-effort step rather than a branch",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticClose, Phrase: "close the window"},
				// The phrase says WHAT, never "if". The conditionality lives in
				// BestEffort, and wording it as a condition would be rejected by the
				// program validator's control-flow guard — correctly, since it cannot
				// tell a description of a conditional from a request for one.
				{Semantic: directorapi.SemanticChoose, Role: RoleDiscardChanges,
					Phrase: "choose the control that discards changes", BestEffort: true},
			}, nil
		},
	}
}

// ── application shells ────────────────────────────────────────────────────────

func genericOpenSettings() Procedure {
	return Procedure{
		Name: "generic open settings", Goal: OpenSettings,
		Safety: Safety{Mutations: 0, Risk: low},
		Why:    "opening settings shows them; nothing is changed until something is set",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleSettingsCommand,
					Phrase: "invoke the settings command"},
			}, nil
		},
	}
}

func genericCreateTab() Procedure {
	return Procedure{
		Name: "generic create tab", Goal: CreateTab,
		Safety: Safety{Mutations: 1, Risk: low},
		Why:    "a new tab is added beside the others and changes none of them",
		Steps: func(g Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleNewTabCommand,
					Phrase: "invoke the new-tab command"},
			}, nil
		},
	}
}

func genericDownload() Procedure {
	return Procedure{
		Name: "generic download", Goal: Download,
		Requires: []Requirement{RequiresTarget},
		Expect:   binding.KindFile,
		Safety: Safety{
			Mutations: 1, External: true, Risk: medium,
			// It reaches the network and writes a file. Neither is destructive, and
			// together they are more than a local click.
		},
		Why: "downloading is reached from the item's context menu in every application " +
			"that offers it, so the menu is opened first",
		Steps: func(g Goal) ([]Directive, error) {
			target, pointed := g.Context.Target, g.Context.TargetIsImplicit
			return []Directive{
				{Semantic: directorapi.SemanticShowContextMenu, Target: target, TargetDeictic: pointed,
					Phrase: "open the context menu for " + quoted(target)},
				{Semantic: directorapi.SemanticChoose, Role: RoleDownloadCommand,
					Phrase: "choose the download command"},
				{Semantic: directorapi.SemanticConfirm, Phrase: "confirm the download",
					BestEffort: true},
			}, nil
		},
	}
}

// quoted renders a target for a phrase, and reads sensibly when it is empty.
func quoted(s string) string {
	if s == "" {
		return "the focused control"
	}
	return `"` + s + `"`
}

// ── sort ──────────────────────────────────────────────────────────────────────

// genericSort reorders whatever list is in front.
//
// One step, and the target is the SORT COMMAND rather than the list: an application that
// can sort has a control that does it, and reaching for the list itself would be reaching
// for a drag. Arrived with the first capability pack and is not a game concept — a file
// manager, a mail client and a music library all have one.
func genericSort() Procedure {
	return Procedure{
		Name: "generic sort", Goal: Sort,
		Safety: Safety{Mutations: 1, Risk: low},
		Why: "an application that can reorder a list exposes a control that does it, so " +
			"sorting is invoking that control rather than moving anything by hand",
		Steps: func(Goal) ([]Directive, error) {
			return []Directive{
				{Semantic: directorapi.SemanticInvoke, Role: RoleSortCommand,
					Phrase: "sort what is in front"},
			}, nil
		},
	}
}

// ── craft ─────────────────────────────────────────────────────────────────────

// genericCraft makes a new thing of a named kind from a catalogue.
//
//	Choose what to make → start making it
//
// Two steps, because the shape has two: a catalogue of things that CAN be made, and a
// control that starts making the chosen one. That is a workbench, and it is equally a
// build tool's target list and a print dialog's paper size — which is why this is in the
// Director's vocabulary rather than in a game pack.
//
// It does not wait for completion. Whether the thing is finished is a question about the
// application's own queue, which a capability pack answers with a condition and a
// verifier; a generic procedure that waited would be waiting for something it cannot
// recognise.
func genericCraft() Procedure {
	return Procedure{
		Name: "generic craft", Goal: Craft,
		Requires: []Requirement{RequiresName},
		Safety:   Safety{Mutations: 1, Risk: low},
		Why: "making something is choosing it from what can be made and then starting " +
			"the work; whether it finished is a question for whatever runs the queue",
		Steps: func(g Goal) ([]Directive, error) {
			what := strings.TrimSpace(g.Param(ParamName))
			if what == "" {
				return nil, fmt.Errorf("making something needs to know what to make")
			}
			return []Directive{
				{Semantic: directorapi.SemanticChoose, Target: what,
					Phrase: fmt.Sprintf("choose %s from what can be made", quoted(what)),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementVisible,
						Description: "a list of what can be made is on screen",
					}}},
				{Semantic: directorapi.SemanticInvoke, Role: RoleCraftCommand,
					Phrase: "start making it"},
			}, nil
		},
	}
}
