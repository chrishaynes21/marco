package goal

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Control intent: naming a control by what it MEANS, not by what it says.
//
//	The procedure must describe the semantic role of the target, not a single
//	visible string.
//
// The library used to say `Target: "Don't Save"`. That is an English string, and on a
// German Windows the button says "Nicht speichern" — so the procedure resolved nothing,
// the step failed, and the document stayed open. Worse is the near miss: a dialog with
// "Save", "Don't Save" and "Cancel" where only "Cancel" happens to match some fallback
// would answer the prompt the wrong way, and the user loses their work.
//
// So a procedure names a ROLE — "the control that discards changes" — and carries an
// ORDERED set of aliases as evidence for finding it. English stays as one alias among
// several rather than as the identity.
//
// What this deliberately is not: a translation service. There is no lookup of arbitrary
// strings, no locale negotiation and no network. It is a fixed, reviewable table of the
// handful of words the built-in procedures need, in the handful of languages it is
// honest to claim — plus a hook for a platform adapter that knows better.

// ControlRole is what a control MEANS, independent of what it says.
//
// A closed vocabulary. A free-form role would let a procedure invent one nothing knows
// how to recognise, which is how a step ends up matching by luck.
type ControlRole string

const (
	// RoleDiscardChanges is the button that closes without saving. The one that matters
	// most: choosing wrongly here destroys work or fails to.
	RoleDiscardChanges ControlRole = "discard_changes"
	// RoleSaveChanges is its opposite, named so it can be positively EXCLUDED.
	RoleSaveChanges ControlRole = "save_changes"
	// RoleCancelAction backs out without answering either way.
	RoleCancelAction ControlRole = "cancel_action"
	// RoleRenameCommand is the command that begins a rename.
	RoleRenameCommand ControlRole = "rename_command"
	// RoleDeleteCommand removes something.
	RoleDeleteCommand ControlRole = "delete_command"
	// RoleNewFolderCommand creates a folder.
	RoleNewFolderCommand ControlRole = "new_folder_command"
	// RoleSaveCommand and RoleSaveAsCommand are the two save commands.
	RoleSaveCommand   ControlRole = "save_command"
	RoleSaveAsCommand ControlRole = "save_as_command"
	// RolePrintCommand prints.
	RolePrintCommand ControlRole = "print_command"
	// RoleSettingsCommand opens settings.
	RoleSettingsCommand ControlRole = "settings_command"
	// RoleNewTabCommand opens a tab.
	RoleNewTabCommand ControlRole = "new_tab_command"
	// RoleRenameSymbolCommand is the editor refactor, deliberately distinct from
	// RoleRenameCommand: they are different operations that share a word.
	RoleRenameSymbolCommand ControlRole = "rename_symbol_command"
	// RoleDownloadCommand saves a resource locally.
	RoleDownloadCommand ControlRole = "download_command"
	// RoleNewSubmenu is the "New" menu a folder command lives under.
	RoleNewSubmenu ControlRole = "new_submenu"
	// RoleFolderItem is the "Folder" entry inside it.
	RoleFolderItem ControlRole = "folder_item"
	// RoleSortCommand reorders a list.
	RoleSortCommand ControlRole = "sort_command"
	// RoleCraftCommand starts making the chosen thing.
	RoleCraftCommand ControlRole = "craft_command"
)

// Describe renders a role for a person.
func (r ControlRole) Describe() string {
	if s, ok := roleDescriptions[r]; ok {
		return s
	}
	// A role a capability pack contributed describes itself. Falling through to the
	// underscore-stripped identifier would put "palworld.craft command" in a prompt.
	if c, ok := contributedRole(r); ok && c.Describe != "" {
		return c.Describe
	}
	return strings.ReplaceAll(string(r), "_", " ")
}

var roleDescriptions = map[ControlRole]string{
	RoleDiscardChanges:      "the control that discards unsaved changes",
	RoleSaveChanges:         "the control that saves changes",
	RoleCancelAction:        "the control that cancels",
	RoleRenameCommand:       "the rename command",
	RoleDeleteCommand:       "the delete command",
	RoleNewFolderCommand:    "the new-folder command",
	RoleSaveCommand:         "the save command",
	RoleSaveAsCommand:       "the save-as command",
	RolePrintCommand:        "the print command",
	RoleSettingsCommand:     "the settings command",
	RoleNewTabCommand:       "the new-tab command",
	RoleRenameSymbolCommand: "the rename-symbol command",
	RoleDownloadCommand:     "the download command",
	RoleNewSubmenu:          "the New menu",
	RoleFolderItem:          "the Folder entry",
	RoleSortCommand:         "the sort command",
	RoleCraftCommand:        "the make command",
}

// Destructive reports whether choosing this control wrongly loses something.
//
// Drives the refusal rule below: for a destructive role, a merely plausible match is not
// good enough.
func (r ControlRole) Destructive() bool {
	if r == RoleDiscardChanges || r == RoleDeleteCommand {
		return true
	}
	// A contributed role declares its own destructiveness, and it is honoured for the
	// same reason the built-ins are: an exact label match is then required, and a merely
	// plausible one is refused rather than pressed.
	c, ok := contributedRole(r)
	return ok && c.Destructive
}

// aliases is the reviewable table of what each role is called.
//
// Ordered: the first alias is the canonical English one, and the rest are the languages
// it is honest to claim. This is NOT a translation service — it is a fixed list of the
// words the built-in procedures need, and adding a language means adding entries here
// where they can be reviewed.
//
// Matching is case-insensitive and ignores the accelerator markers ("&Don't Save") and
// the punctuation that varies by keyboard ("Don't" / "Don't").
var aliases = map[ControlRole][]string{
	RoleDiscardChanges: {
		"Don't Save", "Do not save", "Discard", "Discard changes", "No",
		"Nicht speichern", "Ne pas enregistrer", "No guardar", "Non salvare",
		"Não salvar", "Niet opslaan", "Inte spara", "Ikke gem", "Älä tallenna",
		"Не сохранять", "保存しない", "不保存", "저장 안 함",
	},
	RoleSaveChanges: {
		"Save", "Save changes", "Yes",
		"Speichern", "Enregistrer", "Guardar", "Salva", "Salvar", "Opslaan",
		"Spara", "Gem", "Tallenna", "Сохранить", "保存", "저장",
	},
	RoleCancelAction: {
		"Cancel", "Abbrechen", "Annuler", "Cancelar", "Annulla", "Annuleren",
		"Avbryt", "Annuller", "Peruuta", "Отмена", "キャンセル", "取消", "취소",
	},
	RoleRenameCommand: {
		"Rename", "Umbenennen", "Renommer", "Cambiar nombre", "Rinomina",
		"Renomear", "Naam wijzigen", "Byt namn", "Omdøb", "Nimeä uudelleen",
		"Переименовать", "名前の変更", "重命名", "이름 바꾸기",
	},
	RoleDeleteCommand: {
		"Delete", "Löschen", "Supprimer", "Eliminar", "Elimina", "Excluir",
		"Verwijderen", "Ta bort", "Slet", "Poista", "Удалить", "削除", "删除", "삭제",
	},
	RoleNewFolderCommand: {
		"New folder", "New Folder", "Neuer Ordner", "Nouveau dossier",
		"Nueva carpeta", "Nuova cartella", "Nova pasta", "Nieuwe map",
		"Ny mapp", "Ny mappe", "Uusi kansio", "Создать папку", "新しいフォルダー",
		"新建文件夹", "새 폴더",
	},
	RoleSaveCommand: {
		"Save", "Speichern", "Enregistrer", "Guardar", "Salva", "Salvar",
		"Opslaan", "Spara", "Gem", "Tallenna", "Сохранить", "保存", "저장",
	},
	RoleSaveAsCommand: {
		"Save As", "Save as...", "Speichern unter", "Enregistrer sous",
		"Guardar como", "Salva con nome", "Salvar como", "Opslaan als",
		"Spara som", "Gem som", "Tallenna nimellä", "Сохранить как",
		"名前を付けて保存", "另存为", "다른 이름으로 저장",
	},
	RolePrintCommand: {
		"Print", "Drucken", "Imprimer", "Imprimir", "Stampa", "Afdrukken",
		"Skriv ut", "Udskriv", "Tulosta", "Печать", "印刷", "打印", "인쇄",
	},
	RoleSettingsCommand: {
		"Settings", "Preferences", "Options", "Einstellungen", "Paramètres",
		"Configuración", "Impostazioni", "Configurações", "Instellingen",
		"Inställningar", "Indstillinger", "Asetukset", "Параметры", "設定",
		"设置", "설정",
	},
	RoleNewTabCommand: {
		"New tab", "New Tab", "Neuer Tab", "Nouvel onglet", "Nueva pestaña",
		"Nuova scheda", "Nova guia", "Nieuw tabblad", "Ny flik", "Ny fane",
		"Uusi välilehti", "Новая вкладка", "新しいタブ", "新建标签页", "새 탭",
	},
	RoleRenameSymbolCommand: {
		"Rename Symbol", "Rename symbol", "Symbol umbenennen",
		"Renommer le symbole", "Cambiar nombre del símbolo",
	},
	RoleDownloadCommand: {
		"Save image as", "Save image as...", "Download", "Save link as",
		"Bild speichern unter", "Enregistrer l'image sous", "Guardar imagen como",
		"Salva immagine con nome", "Afbeelding opslaan als", "画像を保存",
	},
	RoleNewSubmenu: {
		"New", "Neu", "Nouveau", "Nuevo", "Nuovo", "Novo", "Nieuw", "Ny",
		"Uusi", "Создать", "新規", "新建", "새로 만들기",
	},
	RoleFolderItem: {
		"Folder", "Ordner", "Dossier", "Carpeta", "Cartella", "Pasta", "Map",
		"Mapp", "Mappe", "Kansio", "Папку", "Папка", "フォルダー", "文件夹", "폴더",
	},
	RoleSortCommand: {
		"Sort", "Sort by", "Sortieren", "Trier", "Ordenar", "Ordina", "Sorteren",
		"Sortera", "Sortér", "Lajittele", "Сортировка", "並べ替え", "排序", "정렬",
	},
	RoleCraftCommand: {
		"Craft", "Make", "Build", "Create", "Produce",
		"Herstellen", "Fabriquer", "Fabricar", "Crea", "Criar", "Maken",
		"Tillverka", "Lav", "Valmista", "Создать", "作成", "制作", "제작",
	},
}

// Aliases returns the ordered candidate labels for a role.
//
// Built-in first, then contributed. One lookup, so a role a capability pack contributed
// resolves by exactly the path a built-in does — see contributed.go.
func Aliases(role ControlRole) []string {
	if a, ok := aliases[role]; ok {
		return a
	}
	return contributedAliases(role)
}

// ControlRoles is every role that has an alias table, in a stable order.
//
// Ordered by hand rather than sorted, because the order is the DISAMBIGUATION order for
// RolesForLabel: "Save" is an alias of both the save command and the save-changes button
// in a discard prompt, and a caller asking "what does this label mean?" needs the same
// answer every time. The command comes first because a control called Save is one, unless
// it is sitting in a prompt — which is a question about context, not about the label, and
// is answered by the caller that has the context.
var ControlRoles = []ControlRole{
	RoleRenameCommand, RoleNewFolderCommand, RoleDeleteCommand,
	RoleSaveCommand, RoleSaveAsCommand, RolePrintCommand,
	RoleSettingsCommand, RoleNewTabCommand, RoleRenameSymbolCommand,
	RoleDownloadCommand, RoleNewSubmenu, RoleFolderItem,
	RoleSaveChanges, RoleDiscardChanges, RoleCancelAction,
	RoleSortCommand, RoleCraftCommand,
}

// RolesForLabel reports which semantic roles a control's label could be, most likely
// first.
//
// The inverse of MatchControl, and it exists because reading a demonstration runs the
// question the other way round: a user pressed a control called "Umbenennen", and what has
// to be recovered is that they invoked the RENAME command — semantically, in a form that
// survives being replayed on an English machine.
//
// EXACT matches only. MatchControl's substring rung is a way to find a control when the
// role is already known and the alternatives are on screen together; used here it would
// read "Rename Symbol" as the rename command, which is a different operation that shares
// the word. When a demonstration cannot be read, the honest answer is that it cannot.
func RolesForLabel(label string) []ControlRole {
	if strings.TrimSpace(label) == "" {
		return nil
	}
	var out []ControlRole
	for _, role := range ControlRoles {
		for _, alias := range aliases[role] {
			if normalizeLabel(label) == normalizeLabel(alias) {
				out = append(out, role)
				break
			}
		}
	}
	// Contributed roles AFTER the built-ins, so a pack cannot change what an ordinary
	// desktop label means. A pack that called something "Save" adds a second reading of
	// it; it does not replace the first.
	eachContributedRole(func(r ContributedRole) {
		for _, alias := range r.Aliases {
			if normalizeLabel(label) == normalizeLabel(alias) {
				out = append(out, r.Role)
				return
			}
		}
	})
	return out
}

// RoleForLabel is RolesForLabel's single best answer, empty when the label means nothing
// to this build.
func RoleForLabel(label string) ControlRole {
	if roles := RolesForLabel(label); len(roles) > 0 {
		return roles[0]
	}
	return ""
}

// ControlMatch is which alias matched, and how.
//
//	Record which alias, accessibility property, or semantic role matched.
//
// Kept as evidence rather than discarded once a control is found: "the Director chose
// Don't Save" and "the Director chose the third button because its label matched the
// German alias for discard" are different claims, and only the second can be checked.
type ControlMatch struct {
	Role ControlRole `json:"role"`
	// Alias is the entry that matched, empty when the platform supplied the label.
	Alias string `json:"alias,omitempty"`
	// Label is the control's actual label, as observed.
	Label string `json:"label,omitempty"`
	// Source is where the match came from: "alias", "platform" or "automation_id".
	Source string `json:"source"`
	// Exact reports whether the label equalled the alias rather than merely containing
	// it. A destructive role requires an exact match — see Resolve.
	Exact bool `json:"exact"`
}

// LocalizedControls is a platform adapter that knows the real labels.
//
// The hook the milestone asks for: a provider that can read a system's own resource
// strings, or an application that exposes an automation id, answers better than any
// fixed table. Optional — without one the aliases carry it.
type LocalizedControls interface {
	// Candidates returns the labels this platform uses for a role, best first. An empty
	// result means "no opinion", not "none exist".
	Candidates(role ControlRole, application string) []string
}

// MatchControl picks the control for a role from the labels actually on screen.
//
// The refusal rule is the point:
//
//	If the destructive choice cannot be identified confidently, refuse rather than
//	selecting a nearby button.
//
// For a destructive role an EXACT alias match is required, and a label that matches the
// OPPOSITE role is excluded first. A save prompt reading "Save / Don't Save / Cancel"
// contains "Save" inside "Don't Save", so a substring match would pick the wrong button
// and lose the work the user asked to discard.
func MatchControl(role ControlRole, labels []string, platform LocalizedControls,
	application string) (ControlMatch, bool) {

	candidates := Aliases(role)
	source := "alias"
	if platform != nil {
		if given := platform.Candidates(role, application); len(given) > 0 {
			// The platform's own answer goes FIRST and keeps the aliases behind it: a
			// system that knows its strings is better evidence than a table, and the
			// table is still there when the system says nothing useful.
			candidates = append(append([]string{}, given...), candidates...)
			source = "platform"
		}
	}

	// Exact first, over the whole candidate list, before any substring is considered.
	// Otherwise "Save" as a substring of "Don't Save" beats the exact "Don't Save"
	// simply by appearing earlier in the list.
	for _, alias := range candidates {
		for _, label := range labels {
			if normalizeLabel(label) == normalizeLabel(alias) {
				return ControlMatch{
					Role: role, Alias: alias, Label: label,
					Source: sourceFor(source, alias, platform, role, application), Exact: true,
				}, true
			}
		}
	}

	if role.Destructive() {
		// No exact match, and this is the choice that loses work. Refused rather than
		// approximated: a substring match here is how "Don't Save" becomes "Save".
		return ControlMatch{Role: role}, false
	}

	for _, alias := range candidates {
		for _, label := range labels {
			if containsWord(label, alias) && !matchesOppositeRole(role, label) {
				return ControlMatch{
					Role: role, Alias: alias, Label: label,
					Source: sourceFor(source, alias, platform, role, application), Exact: false,
				}, true
			}
		}
	}
	return ControlMatch{Role: role}, false
}

// sourceFor reports whether the winning alias came from the platform or the table.
func sourceFor(source, alias string, platform LocalizedControls, role ControlRole,
	application string) string {

	if source != "platform" || platform == nil {
		return "alias"
	}
	for _, given := range platform.Candidates(role, application) {
		if normalizeLabel(given) == normalizeLabel(alias) {
			return "platform"
		}
	}
	return "alias"
}

// matchesOppositeRole reports whether a label is a better match for the role's opposite.
//
// The guard that keeps "Save" from answering a request for "discard": the two live in
// the same dialog, and a loose match on either is a wrong answer to the same question.
func matchesOppositeRole(role ControlRole, label string) bool {
	opposite, has := opposites[role]
	if !has {
		return false
	}
	for _, alias := range aliases[opposite] {
		if normalizeLabel(label) == normalizeLabel(alias) {
			return true
		}
	}
	return false
}

var opposites = map[ControlRole]ControlRole{
	RoleDiscardChanges: RoleSaveChanges,
	RoleSaveChanges:    RoleDiscardChanges,
}

// normalizeLabel makes two labels comparable.
//
// Accelerator markers ("&Don't Save"), the ellipsis a menu entry carries ("Save As..."),
// the apostrophe the keyboard produced, and surrounding whitespace all vary without the
// control meaning anything different.
func normalizeLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "&", "")
	s = strings.ReplaceAll(s, "’", "'") // right single quote → apostrophe
	s = strings.TrimSuffix(s, "...")
	s = strings.TrimSuffix(s, "…") // ellipsis
	s = strings.TrimSpace(s)
	return strings.Join(strings.Fields(s), " ")
}

// containsWord reports whether a label contains an alias as a whole run of words.
func containsWord(label, alias string) bool {
	l, a := normalizeLabel(label), normalizeLabel(alias)
	if l == a {
		return true
	}
	return strings.Contains(l, a)
}

// ElementLabels pulls the labels out of a world, for MatchControl.
//
// Here rather than in the caller so the one definition of "the labels on screen" is
// shared by the procedures and the tests.
func ElementLabels(w *directorapi.WorldState) []string {
	if w == nil {
		return nil
	}
	out := make([]string, 0, len(w.Elements))
	for _, el := range w.Elements {
		if el.Label != "" {
			out = append(out, el.Label)
		}
	}
	return out
}
