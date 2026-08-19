package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestErrorDetailsShowsFullWrappedMessage(t *testing.T) {
	message := "operation failed: the complete underlying AWS error remains available for inspection"
	model := NewErrorDetails(message, 56, 24)
	view := stripAnsi(model.View())
	for _, fragment := range []string{"operation failed", "underlying AWS error", "inspection"} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("error detail omitted %q:\n%s", fragment, view)
		}
	}
}

func TestErrorDetailsCanCloseWithoutDismissing(t *testing.T) {
	app := App{
		kb:           NewKeyBindings(),
		err:          errors.New("complete error"),
		errorDetails: NewErrorDetails("complete error", 80, 24),
	}

	updatedModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := updatedModel.(App)
	if updated.errorDetails.Active {
		t.Fatal("escape did not close error details")
	}
	if updated.err == nil {
		t.Fatal("closing details unexpectedly dismissed the error")
	}
}

func TestErrorDetailsKeyOpensFullError(t *testing.T) {
	app := App{
		kb:  NewKeyBindings(),
		err: errors.New("complete error"),
	}

	updatedModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	updated := updatedModel.(App)
	if !updated.errorDetails.Active || updated.errorDetails.Message != "complete error" {
		t.Fatal("error details key did not open the complete error")
	}
}

func TestErrorDetailsCanDismissError(t *testing.T) {
	app := App{
		kb:           NewKeyBindings(),
		err:          errors.New("complete error"),
		errorDetails: NewErrorDetails("complete error", 80, 24),
	}

	updatedModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := updatedModel.(App)
	if updated.err != nil || updated.errorDetails.Active {
		t.Fatal("dismiss did not clear the error and close its details")
	}
}

func TestBackClearsStaleError(t *testing.T) {
	app := App{state: viewTaskDetail, err: errors.New("stale error")}
	updated, _ := app.goBack()
	if updated.err != nil {
		t.Fatal("navigation retained a stale error")
	}
}
