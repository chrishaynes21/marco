package goal

import (
	"fmt"
	"sort"
	"sync"
)

// Control roles a source outside the Director can contribute.
//
// The built-in table names the controls a desktop has: Rename, Delete, Save, New Folder.
// An application with its own vocabulary — a craft button, a deposit-all button, an
// incubator — has controls the Director has never heard of, and a procedure that named
// them in English would work on one machine and fail on the next.
//
// So a capability pack contributes ROLES, in exactly the form the built-in table uses: a
// name for the meaning, and the ordered list of labels it is known by. From that point on
// a contributed role is an ordinary role — Aliases, RolesForLabel, MatchControl and every
// procedure that names one behave identically, because they read one table.
//
// # Registration is a startup act
//
// Contributed roles are registered by the composition root before the Director serves
// anything, and never after. A role that appeared mid-session would make one request
// resolve a control the next could not, for reasons nothing could show a user — so the
// registry refuses a second, different registration of the same role rather than allowing
// a pack to redefine one under a running system.
//
// The lock is here because "before serving anything" is a convention and the reader is
// entitled to something stronger than a convention.

var contributed = struct {
	mu      sync.RWMutex
	roles   map[ControlRole]ContributedRole
	byOwner map[string][]ControlRole
}{
	roles:   map[ControlRole]ContributedRole{},
	byOwner: map[string][]ControlRole{},
}

// ContributedRole is a control role a capability pack contributes.
type ContributedRole struct {
	// Role is the identifier procedures name it by, and must not collide with a
	// built-in. Conventionally prefixed with the pack ("palworld.craft_command"), which
	// is what makes a collision between two packs impossible in practice as well as
	// refused in principle.
	Role ControlRole
	// Describe is what to call it in a plan and a prompt ("the craft button").
	Describe string
	// Aliases are the labels it is known by, canonical first. The same contract the
	// built-in table has: exact matching, in order.
	Aliases []string
	// Destructive marks a control that loses something when chosen wrongly. A
	// destructive role demands an EXACT label match and is refused rather than
	// approximated — see MatchControl.
	Destructive bool
	// Owner is the pack that contributed it, for diagnostics and for removal.
	Owner string
}

// RegisterControlRole adds a contributed role.
//
// Refuses a role that collides with a built-in or with a different registration of the
// same name. Re-registering the IDENTICAL role is allowed and does nothing, so a process
// that builds two registries does not fail on the second.
func RegisterControlRole(r ContributedRole) error {
	switch {
	case r.Role == "":
		return fmt.Errorf("goal: a contributed control role needs a name")
	case len(r.Aliases) == 0:
		return fmt.Errorf("goal: %q contributes no labels, so nothing could ever match it",
			r.Role)
	case r.Owner == "":
		return fmt.Errorf("goal: %q names no owner, so a refusal could not say who "+
			"contributed it", r.Role)
	}
	if _, builtin := aliases[r.Role]; builtin {
		return fmt.Errorf("goal: %q is a built-in control role and cannot be redefined",
			r.Role)
	}

	contributed.mu.Lock()
	defer contributed.mu.Unlock()
	if existing, ok := contributed.roles[r.Role]; ok {
		if sameRole(existing, r) {
			return nil
		}
		return fmt.Errorf("goal: %q is already contributed by %q with different labels",
			r.Role, existing.Owner)
	}
	contributed.roles[r.Role] = r
	contributed.byOwner[r.Owner] = append(contributed.byOwner[r.Owner], r.Role)
	return nil
}

// sameRole reports whether two registrations are identical.
func sameRole(a, b ContributedRole) bool {
	if a.Owner != b.Owner || a.Describe != b.Describe ||
		a.Destructive != b.Destructive || len(a.Aliases) != len(b.Aliases) {
		return false
	}
	for i := range a.Aliases {
		if a.Aliases[i] != b.Aliases[i] {
			return false
		}
	}
	return true
}

// ContributedRoles lists what has been contributed, by role name.
func ContributedRoles() []ContributedRole {
	contributed.mu.RLock()
	defer contributed.mu.RUnlock()
	out := make([]ContributedRole, 0, len(contributed.roles))
	for _, r := range contributed.roles {
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}

// ContributedRolesOf lists the roles one owner contributed.
func ContributedRolesOf(owner string) []ControlRole {
	contributed.mu.RLock()
	defer contributed.mu.RUnlock()
	out := append([]ControlRole{}, contributed.byOwner[owner]...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// contributedRole looks one up.
func contributedRole(role ControlRole) (ContributedRole, bool) {
	contributed.mu.RLock()
	defer contributed.mu.RUnlock()
	r, ok := contributed.roles[role]
	return r, ok
}

// contributedAliases is the label list for a contributed role, nil for anything else.
func contributedAliases(role ControlRole) []string {
	if r, ok := contributedRole(role); ok {
		return r.Aliases
	}
	return nil
}

// eachContributedRole visits every contributed role under one read lock.
func eachContributedRole(visit func(ContributedRole)) {
	contributed.mu.RLock()
	defer contributed.mu.RUnlock()
	// Sorted, so a label that two packs claim resolves the same way every time. Iterating
	// a Go map here would make RolesForLabel non-deterministic, which is precisely the
	// class of bug the built-in table's fixed order exists to prevent.
	names := make([]ControlRole, 0, len(contributed.roles))
	for name := range contributed.roles {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	for _, name := range names {
		visit(contributed.roles[name])
	}
}

// ForgetContributedRoles removes one owner's roles.
//
// For tests, and for a composition root that rebuilds its registry. Not part of the
// running system's vocabulary management: a pack that could withdraw a role mid-session
// would make a procedure that resolved a moment ago stop resolving.
func ForgetContributedRoles(owner string) {
	contributed.mu.Lock()
	defer contributed.mu.Unlock()
	for _, role := range contributed.byOwner[owner] {
		delete(contributed.roles, role)
	}
	delete(contributed.byOwner, owner)
}
