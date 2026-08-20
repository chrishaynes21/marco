// Package clipboard lets the Director BORROW the clipboard.
//
// Pasting is often the right way to put text into a control — it is atomic, it is
// layout-independent, and it does not fire an autocomplete on every character. But the
// clipboard belongs to the user, and it usually holds something: the URL they were
// about to paste, the paragraph they just cut. Automation that quietly destroys it has
// taken something that was not offered.
//
// So the rule here is absolute: never destroy clipboard contents permanently. Every
// borrow saves first and restores afterwards, the restore runs even when the paste
// failed, and whether the restore SUCCEEDED is recorded rather than assumed. A restore
// that silently failed would be worse than no restore at all, because the user would
// have no reason to look.
package clipboard

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Borrower takes the clipboard temporarily and gives it back.
type Borrower struct {
	board directorapi.Clipboard
}

// New wraps a clipboard.
func New(board directorapi.Clipboard) *Borrower { return &Borrower{board: board} }

// Loan is an in-progress borrow. Restore must be called, and the caller must record
// what it reports.
type Loan struct {
	borrower *Borrower

	// saved is what was on the clipboard, and savedText whether it was text at all.
	saved     string
	savedText bool
	// wasEmpty records that there was nothing to give back, so Restore writes "".
	wasEmpty bool
}

// ErrNonText reports that the clipboard held something this package cannot preserve.
//
// Returned INSTEAD of borrowing. The alternative — paste anyway and restore "" — would
// silently destroy an image the user had copied, and no amount of care afterwards
// brings it back. Refusing to borrow costs one fallback to typing; borrowing anyway
// costs the user their data.
var ErrNonText = fmt.Errorf("the clipboard holds something that is not text, so it cannot be borrowed without destroying it")

// Borrow saves the clipboard, writes text, and returns the loan to restore later.
func (b *Borrower) Borrow(ctx context.Context, text string) (*Loan, error) {
	if b == nil || b.board == nil {
		return nil, fmt.Errorf("clipboard: no clipboard is available")
	}
	held, err := b.board.Read(ctx)
	if err != nil {
		// Could not read it, so cannot promise to give it back. Do not proceed:
		// borrowing without being able to restore is just taking.
		return nil, fmt.Errorf("clipboard: could not save the current contents, so it will not be used: %w", err)
	}
	if !held.IsText && !held.Empty {
		// An image, a file list, a spreadsheet range. Nothing here can save it or put
		// it back, and pasting over it would destroy it while reporting success. The
		// cost of refusing is one fallback to typing; the cost of proceeding is the
		// user's data.
		return nil, ErrNonText
	}
	if err := b.board.Write(ctx, text); err != nil {
		return nil, fmt.Errorf("clipboard: could not write: %w", err)
	}
	// An empty clipboard is restored by writing "" back. Not byte-identical — the
	// clipboard afterwards holds an empty string rather than nothing at all — but
	// nothing the user can retrieve is different, which is the standard that matters.
	return &Loan{borrower: b, saved: held.Text, savedText: held.IsText, wasEmpty: held.Empty}, nil
}

// Restore puts back what was there.
//
// Returns whether the clipboard is genuinely back to its original state. A caller must
// surface a false, not swallow it: the user's clipboard being wrong is a thing they
// need to be told about, and the Director is the only one who knows.
func (l *Loan) Restore(ctx context.Context) (bool, error) {
	if l == nil || l.borrower == nil {
		return false, nil
	}
	if err := l.borrower.board.Write(ctx, l.saved); err != nil {
		return false, fmt.Errorf("clipboard: could not restore the previous contents: %w", err)
	}
	return true, nil
}

// WasEmpty reports that there was nothing on the clipboard to begin with.
func (l *Loan) WasEmpty() bool { return l != nil && l.wasEmpty }

// Saved reports what the loan is holding, for diagnostics. The text itself is not
// logged anywhere by this package — a clipboard routinely holds passwords.
func (l *Loan) Saved() (length int, wasText bool) {
	if l == nil {
		return 0, false
	}
	return len(l.saved), l.savedText
}
