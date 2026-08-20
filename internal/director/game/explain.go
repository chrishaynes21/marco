package game

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What the diagnostics answer.
//
//	director game            what is detected, and what the pack permits
//	director capabilities    what each registered pack contributes
//	director explain game    why this pack was chosen and the others were not
//	director explain inventory   what the Director can see of what the player holds
//
// The rendering lives here rather than in the CLI for the reason every other diagnostic
// does: the service transports what a layer concluded and has no opinion about it, and a
// second renderer in the client would be a second opinion.

// Report is what `director capabilities` shows.
type Report struct {
	// Packs is what is registered.
	Packs []PackReport `json:"packs"`
	// Active is what the Director believes it is looking at.
	Active Active `json:"active"`
}

// PackReport is one pack's contribution, counted.
type PackReport struct {
	Name        string   `json:"name"`
	Application string   `json:"application"`
	Version     string   `json:"version"`
	Safety      Safety   `json:"safety"`
	Observers   []string `json:"observers,omitempty"`
	Procedures  []string `json:"procedures,omitempty"`
	Conditions  []string `json:"conditions,omitempty"`
	Verifiers   []string `json:"verifiers,omitempty"`
	Roles       []string `json:"control_roles,omitempty"`
	Policies    []string `json:"policies,omitempty"`
	// Automations is what kinds of automation its procedures declare, deduplicated.
	Automations []string `json:"automations,omitempty"`
}

// Describe renders one pack for a person.
func (p PackReport) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)\n", p.Application, p.Name)
	if p.Version != "" {
		fmt.Fprintf(&b, "  version      %s\n", p.Version)
	}
	fmt.Fprintf(&b, "  safety       %s\n", p.Safety.Describe())
	if p.Safety.Note != "" {
		fmt.Fprintf(&b, "               %s\n", p.Safety.Note)
	}
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "  %-12s %s\n", title, items[0])
		for _, item := range items[1:] {
			fmt.Fprintf(&b, "  %-12s %s\n", "", item)
		}
	}
	section("observes", p.Observers)
	section("procedures", p.Procedures)
	section("conditions", p.Conditions)
	section("verifies", p.Verifiers)
	section("controls", p.Roles)
	section("policies", p.Policies)
	return b.String()
}

// Describe renders the whole report.
func (r Report) Describe() string {
	var b strings.Builder
	if len(r.Packs) == 0 {
		return "No capability packs are registered. The Director behaves exactly as it " +
			"does for any other application.\n"
	}
	b.WriteString(r.Active.Describe())
	b.WriteString("\n")
	for i, p := range r.Packs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(p.Describe())
	}
	return b.String()
}

// Describe renders what is detected.
func (a Active) Describe() string {
	var b strings.Builder
	if !a.Detected() {
		b.WriteString("Detected game\n  none — no capability pack serves what is in front\n")
		if len(a.Considered) > 0 {
			b.WriteString("\nConsidered\n")
			for _, c := range a.Considered {
				fmt.Fprintf(&b, "  %-16s %.0f%%\n", c.Pack, c.Confidence*100)
				for _, e := range c.Evidence {
					fmt.Fprintf(&b, "    %s\n", e)
				}
			}
		}
		return b.String()
	}
	fmt.Fprintf(&b, "Detected game\n  %s\n\nCapability pack\n  %s (%.0f%% confident)\n",
		a.Application, a.Pack, a.Detection.Confidence*100)
	if a.Detection.GameVersion != "" {
		fmt.Fprintf(&b, "\nGame version\n  %s\n", a.Detection.GameVersion)
	}
	if a.Detection.Mode != "" {
		fmt.Fprintf(&b, "\nUI mode\n  %s\n", a.Detection.Mode)
	}
	if len(a.Detection.Evidence) > 0 {
		b.WriteString("\nBecause\n")
		for _, e := range a.Detection.Evidence {
			fmt.Fprintf(&b, "  %s\n", e)
		}
	}
	fmt.Fprintf(&b, "\nAutomation\n  %s\n", a.Safety.Describe())
	if a.Safety.Note != "" {
		fmt.Fprintf(&b, "  %s\n", a.Safety.Note)
	}
	return b.String()
}

// Report gathers what every pack contributes.
func (r *Registry) Report(active Active) Report {
	out := Report{Active: active}
	for _, p := range r.Packs() {
		pr := PackReport{
			Name: p.Name(), Application: p.Describe(), Version: p.Version(),
			Safety: p.Safety(),
		}
		for _, in := range p.Interpreters() {
			pr.Observers = append(pr.Observers, in.Name())
		}
		seen := map[Automation]bool{}
		for _, proc := range p.Procedures() {
			pr.Procedures = append(pr.Procedures,
				fmt.Sprintf("%s (%s)", proc.Procedure.Name, proc.Automation))
			if !seen[proc.Automation] {
				seen[proc.Automation] = true
				pr.Automations = append(pr.Automations, string(proc.Automation))
			}
		}
		for _, c := range p.Conditions() {
			pr.Conditions = append(pr.Conditions, c.Description())
		}
		for _, v := range p.Verifiers() {
			pr.Verifiers = append(pr.Verifiers, v.Name())
		}
		for _, role := range p.ControlRoles() {
			pr.Roles = append(pr.Roles, fmt.Sprintf("%s — %s", role.Role, role.Describe))
		}
		for _, pol := range p.Policies() {
			pr.Policies = append(pr.Policies, pol.Name())
		}
		out.Packs = append(out.Packs, pr)
	}
	return out
}

// ── explain inventory ─────────────────────────────────────────────────────────

// InventoryReport is what `director explain inventory` shows.
//
// What the Director can SEE, and — just as importantly — what it cannot. An inventory
// diagnostic that showed only the readable slots would let a user believe "deposit
// everything" covers everything.
type InventoryReport struct {
	// Detected is the pack serving the window, empty when none does.
	Detected string `json:"detected,omitempty"`
	// Inventory is what was read.
	Inventory Inventory `json:"inventory"`
	// Containers are the other containers the world reports.
	Containers []string `json:"containers,omitempty"`
	// Meters are the levels it can see.
	Meters []Meter `json:"meters,omitempty"`
	// Stations are the places work happens.
	Stations []Station `json:"stations,omitempty"`
	// Unavailable explains why there is nothing, when there is nothing.
	Unavailable string `json:"unavailable,omitempty"`
}

// ReadInventoryReport builds the diagnostic.
func (r *Registry) ReadInventoryReport(w directorapi.WorldState, active Active,
	container string) InventoryReport {

	out := InventoryReport{
		Detected:  active.Pack,
		Inventory: ReadInventory(w, container),
		Meters:    Meters(w),
		Stations:  Stations(w),
	}
	seen := map[string]bool{}
	for _, el := range w.Elements {
		e := el.Entity
		if e.Known() && e.Kind == directorapi.EntityContainer && e.Name != "" && !seen[e.Name] {
			seen[e.Name] = true
			out.Containers = append(out.Containers, e.Name)
		}
	}
	if out.Inventory.Slots == 0 && len(out.Meters) == 0 && len(out.Stations) == 0 {
		switch {
		case !active.Detected():
			out.Unavailable = "no capability pack serves what is in front, so nothing " +
				"here is modelled as an inventory"
		default:
			out.Unavailable = fmt.Sprintf(
				"%s serves this window and its observers reported no inventory — the "+
					"interface may not be open, or may not be one it models", active.Pack)
		}
	}
	return out
}

// Describe renders the inventory diagnostic.
func (r InventoryReport) Describe() string {
	var b strings.Builder
	if r.Unavailable != "" {
		return "Inventory\n  " + r.Unavailable + "\n"
	}
	fmt.Fprintf(&b, "Inventory\n  %s\n", r.Inventory.Describe())
	if full, known := r.Inventory.Full(); known {
		fmt.Fprintf(&b, "  full: %v\n", full)
	} else {
		b.WriteString("  full: cannot be established\n")
	}
	if len(r.Inventory.Items) > 0 {
		b.WriteString("\nHolding\n")
		for _, it := range r.Inventory.Items {
			fmt.Fprintf(&b, "  %s\n", it.Describe())
		}
	}
	if len(r.Containers) > 0 {
		fmt.Fprintf(&b, "\nContainers\n  %s\n", strings.Join(r.Containers, ", "))
	}
	if len(r.Meters) > 0 {
		b.WriteString("\nMeters\n")
		for _, m := range r.Meters {
			fmt.Fprintf(&b, "  %-16s %.0f%%\n", m.Name, m.Level*100)
		}
	}
	if len(r.Stations) > 0 {
		b.WriteString("\nStations\n")
		for _, s := range r.Stations {
			line := fmt.Sprintf("  %-16s %s", s.Name, s.State)
			if s.Progress != nil {
				line += fmt.Sprintf(" (%.0f%%)", *s.Progress*100)
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
