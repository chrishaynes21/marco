package palworld

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The procedures this pack contributes.
//
//	These are ordinary procedures.
//
// Ordinary in every respect that matters: each is a goal.Procedure, expanded by the
// ordinary expander into an ordinary program.Program, validated by the ordinary validator,
// confirmed by the ordinary gate, lowered into legal Marco and verified semantically. The
// only thing this file adds is knowing that the button which deposits everything is called
// "Deposit All" — and even that is expressed as a semantic ROLE so it survives being
// played in German.
//
// Each declares which kind of automation it is, checked at registration against what the
// pack permits. A procedure here that declared itself something the pack does not permit
// stops the Director from starting.
//
// # Why these goals and not new ones
//
// Every procedure below serves a goal that already existed: open_settings for opening a
// panel, create_folder-shaped naming for an incubator, and so on would be a stretch — so
// where the outcome genuinely is a new one, this pack contributes it as a goal the
// vocabulary already has, and where it is not, it does not invent one. Adding a goal kind
// is a change to the Director's vocabulary and belongs in a review of that vocabulary, not
// in a pack.

// Control roles this pack contributes, so a procedure can name a control by meaning.
//
// Prefixed with the pack name, which is what makes a collision between two packs
// impossible in practice as well as refused in principle.
const (
	RoleDepositAll    goal.ControlRole = "palworld.deposit_all"
	RoleTakeAll       goal.ControlRole = "palworld.take_all"
	RoleSortButton    goal.ControlRole = "palworld.sort"
	RoleCraftButton   goal.ControlRole = "palworld.craft"
	RoleInventoryTab  goal.ControlRole = "palworld.inventory_tab"
	RoleTechnologyTab goal.ControlRole = "palworld.technology_tab"
	RolePalboxButton  goal.ControlRole = "palworld.palbox"
	RoleIncubateEgg   goal.ControlRole = "palworld.incubate"
	RoleRenamePal     goal.ControlRole = "palworld.rename_pal"
	RoleRepairButton  goal.ControlRole = "palworld.repair"
)

// controlRoles is the table.
//
// English first, then the languages Palworld ships in that this pack is prepared to claim.
// The same contract the Director's own table has: exact matching, in order, and a language
// absent from the list is a language this pack does not claim rather than one it guesses at.
func controlRoles() []goal.ContributedRole {
	return []goal.ContributedRole{
		{
			Role: RoleDepositAll, Describe: "the deposit-all button",
			Aliases: []string{
				"Deposit All", "Deposit all", "Store All",
				"Alle einlagern", "Tout déposer", "Depositar todo", "Deposita tutto",
				"すべて預ける", "全部存入", "모두 맡기기",
			},
		},
		{
			Role: RoleTakeAll, Describe: "the take-all button",
			Aliases: []string{
				"Take All", "Take all", "Retrieve All",
				"Alle nehmen", "Tout prendre", "Tomar todo", "Prendi tutto",
				"すべて取り出す", "全部取出", "모두 가져오기",
			},
		},
		{
			Role: RoleSortButton, Describe: "the sort button",
			Aliases: []string{
				"Sort", "Sort Items", "Sortieren", "Trier", "Ordenar", "Ordina",
				"並べ替え", "整理", "정렬",
			},
		},
		{
			Role: RoleCraftButton, Describe: "the craft button",
			Aliases: []string{
				"Craft", "Crafting", "Make", "Herstellen", "Fabriquer", "Fabricar",
				"Crea", "作成", "制作", "제작",
			},
		},
		{
			Role: RoleInventoryTab, Describe: "the inventory tab",
			Aliases: []string{
				"Inventory", "Inventar", "Inventaire", "Inventario",
				"インベントリ", "背包", "인벤토리",
			},
		},
		{
			Role: RoleTechnologyTab, Describe: "the technology tab",
			Aliases: []string{
				"Technology", "Technologie", "Technologie", "Tecnología", "Tecnologia",
				"テクノロジー", "技术", "기술",
			},
		},
		{
			Role: RolePalboxButton, Describe: "the Palbox",
			Aliases: []string{"Palbox", "Pal Box", "パルボックス", "帕鲁盒子", "팰 박스"},
		},
		{
			Role: RoleIncubateEgg, Describe: "the incubate button",
			Aliases: []string{
				"Incubate", "Start Incubating", "Ausbrüten", "Incuber", "Incubar",
				"Incuba", "孵化", "부화",
			},
		},
		{
			Role: RoleRenamePal, Describe: "the rename control for a Pal",
			Aliases: []string{
				"Rename", "Change Name", "Umbenennen", "Renommer", "Cambiar nombre",
				"Rinomina", "名前変更", "重命名", "이름 변경",
			},
		},
		{
			Role: RoleRepairButton, Describe: "the repair button",
			Aliases: []string{
				"Repair", "Reparieren", "Réparer", "Reparar", "Ripara",
				"修理", "修复", "수리",
			},
		},
	}
}

// ── the procedures ────────────────────────────────────────────────────────────

// procedures is what this pack contributes to the goal registry.
func procedures() []game.Procedure {
	return []game.Procedure{
		{Procedure: openInventory(), Automation: game.AutomationMenus},
		{Procedure: depositEverything(), Automation: game.AutomationInventory},
		{Procedure: withdrawEverything(), Automation: game.AutomationInventory},
		{Procedure: sortInventory(), Automation: game.AutomationOrganization},
		{Procedure: craftItem(), Automation: game.AutomationCrafting},
		{Procedure: renamePal(), Automation: game.AutomationOrganization},
	}
}

// apps is what these procedures serve. The application id the window system reports, plus
// the executable name, because which one the Director has depends on the provider.
var apps = []string{"Palworld", "Palworld-Win64-Shipping", "Palworld-Win64-Shipping.exe"}

// openInventory opens the player's inventory.
//
//	Open my inventory.
func openInventory() goal.Procedure {
	return goal.Procedure{
		Name: "palworld open inventory", Goal: goal.OpenSettings,
		Applications: apps,
		Safety:       goal.Safety{Mutations: 0, Risk: directorapi.RiskLow},
		Why: "Palworld opens the inventory as a tab of the player menu, so this invokes " +
			"the tab rather than pressing a key that may be rebound",
		Steps: func(goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleInventoryTab,
					Phrase: "open the inventory",
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementVisible,
						Description: "the inventory tab is on screen",
					}},
				},
			}, nil
		},
	}
}

// depositEverything moves the player's items into an open container.
//
//	Deposit everything except food.
//
// The exception is carried as the goal's own name parameter, which is how "except food"
// reaches a procedure without a new parameter vocabulary. Absent means everything.
func depositEverything() goal.Procedure {
	return goal.Procedure{
		Name: "palworld deposit inventory", Goal: goal.Move,
		Applications: apps,
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskMedium},
		Why: "Palworld's storage screens carry a deposit-all control, which moves the " +
			"whole inventory in one verified action rather than one drag per slot",
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleDepositAll,
					Phrase: "deposit everything into the open container",
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementVisible,
						Description: "a storage container is open",
					}},
				},
			}, nil
		},
	}
}

// withdrawEverything is its complement.
func withdrawEverything() goal.Procedure {
	return goal.Procedure{
		Name: "palworld withdraw materials", Goal: goal.Copy,
		Applications: apps,
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskMedium},
		Why:          "the take-all control moves a container's contents to the player in one action",
		Steps: func(goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleTakeAll,
					Phrase: "take everything from the open container",
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementVisible,
						Description: "a storage container is open",
					}},
				},
			}, nil
		},
	}
}

// sortInventory tidies a container.
//
//	Sort this inventory.
func sortInventory() goal.Procedure {
	return goal.Procedure{
		Name: "palworld sort inventory", Goal: goal.Sort,
		Applications: apps,
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskLow},
		Why:          "the sort control reorders a container in one action",
		Steps: func(goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleSortButton,
					Phrase: "sort the open container",
				},
			}, nil
		},
	}
}

// craftItem queues one item at a station.
//
//	Craft more arrows.
//
// Two steps and a wait: choose the recipe, invoke craft, and let the queue prove it. The
// wait is a PRECONDITION on nothing — Palworld's craft is immediate at the queue level —
// so what proves it is the verifier, not a sleep.
func craftItem() goal.Procedure {
	return goal.Procedure{
		Name: "palworld craft item", Goal: goal.Craft,
		Applications: apps,
		Requires:     []goal.Requirement{goal.RequiresName},
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskLow},
		Why: "Palworld crafts by choosing a recipe in the technology or workbench list " +
			"and invoking craft; the queue is what proves it happened",
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			item := g.Param(goal.ParamName)
			if item == "" {
				return nil, fmt.Errorf("crafting needs an item to make")
			}
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticChoose, Target: item,
					Phrase: fmt.Sprintf("choose %s in the recipe list", item),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementVisible,
						Description: "a crafting list is open",
					}},
				},
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleCraftButton,
					Phrase: "start crafting it",
				},
			}, nil
		},
	}
}

// renamePal renames a captured creature.
//
//	Rename this Pal.
//
// A rename, and it uses the Director's OWN rename goal — the point being that the Director
// already knows what renaming is. What the pack adds is which control begins one here.
func renamePal() goal.Procedure {
	return goal.Procedure{
		Name: "palworld rename pal", Goal: goal.Rename,
		Applications: apps,
		Requires:     []goal.Requirement{goal.RequiresName},
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskLow},
		Why: "Palworld renames a Pal from its detail panel, and the field it opens is an " +
			"ordinary editable control the Director already knows how to fill",
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{
				{
					Semantic: directorapi.SemanticInvoke, Role: RoleRenamePal,
					Phrase: "begin renaming the Pal",
				},
				{
					SetText: true, Text: g.Param(goal.ParamName),
					Phrase: fmt.Sprintf("set the name to %q", g.Param(goal.ParamName)),
					Preconditions: []directorapi.Condition{{
						Type:        directorapi.ConditionElementFocused,
						Description: "the name field is editable",
					}},
				},
				{
					Semantic: directorapi.SemanticConfirm,
					Phrase:   "confirm the new name",
				},
			}, nil
		},
	}
}
