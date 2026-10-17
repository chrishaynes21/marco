package directorapi

import (
	"strings"
	"testing"
)

// NAMING A CONTROL WITHOUT QUOTING SOMEBODY'S LIFE BACK AT THEM.
//
// Measured live. Navigating Settings by request, Marco's ordinary verification line read
//
//	COMPLETED: keyboard focus moved from Chris Haynes chris.haynes2112@gmail.com to Settings
//
// Correct, and not diagnostic information: that is somebody's identity, quoted into a status
// message to say a thing that could have been said about "the account control".

func TestAnAccountNamesAControlWithoutQuotingSomebodysLife(t *testing.T) {
	// An address is never repeated, however it is wrapped.
	for _, label := range []string{
		"chris.haynes2112@gmail.com",
		"Chris Haynes chris.haynes2112@gmail.com",
		"signed in as someone@example.co.uk",
	} {
		el := &Element{Role: RoleButton, Label: label}
		got := el.ForAccount()
		if strings.Contains(got, "@") {
			t.Errorf("an address survived into a status message: %q", got)
		}
		// AND THE CONTROL IS STILL IDENTIFIED, which is the whole point: hiding the
		// role would protect nobody and make an account unreadable.
		if !strings.Contains(got, string(RoleButton)) {
			t.Errorf("the control is no longer identifiable: %q", got)
		}
		if _, ok := QuotableLabel(label); ok {
			t.Errorf("%q was judged quotable", label)
		}
	}

	// An ordinary control name is quoted exactly, because that is what a diagnostic is for.
	for _, label := range []string{
		"Bluetooth & devices", "Add device", "New", "Folder", "notes.txt",
		"Scroll inactive windows when hovering over them",
	} {
		text, ok := QuotableLabel(label)
		if !ok || text != label {
			t.Errorf("an ordinary control name was hidden: %q", label)
		}
		if got := (&Element{Role: RoleListItem, Label: label}).ForAccount(); !strings.Contains(got, label) {
			t.Errorf("%q did not survive into its own account: %q", label, got)
		}
	}
}

// AND CONTENT IS DESCRIBED RATHER THAN REPEATED.
//
// A text field's label on a word processor is often the first line of what somebody is writing.
// The length and the role are enough to tell two controls apart; the words are not needed.
func TestLongLabelsAndDocumentContentAreDescribedNotRepeated(t *testing.T) {
	content := "Dear Dr Okonkwo, I am writing to confirm the results of last Tuesday's appointment"
	el := &Element{Role: RoleTextField, Label: content}
	got := el.ForAccount()
	if strings.Contains(got, "Okonkwo") || strings.Contains(got, "appointment") {
		t.Errorf("a document's contents were repeated: %q", got)
	}
	if !strings.Contains(got, "not repeated here") {
		t.Errorf("it hid the label without saying it had: %q", got)
	}
	if !strings.Contains(got, string(RoleTextField)) {
		t.Errorf("the control is no longer identifiable: %q", got)
	}
	// A multi-line label is content whatever its length.
	if _, ok := QuotableLabel("Save\nDon't Save"); ok {
		t.Error("a label spanning lines was judged quotable")
	}
	// And nothing at all is still named by its role.
	if got := (&Element{Role: RoleButton}).ForAccount(); got != string(RoleButton) {
		t.Errorf("an unlabelled control reads as %q", got)
	}
	if got := (*Element)(nil).ForAccount(); got != "nothing" {
		t.Errorf("no element at all reads as %q", got)
	}
}
