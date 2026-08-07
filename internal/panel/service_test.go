package panel

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// TestGetPanel verifies the owner can read a panel by id.
func TestGetPanel(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)

	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{
		{Caption: "hello", CharacterIDs: []int64{9}},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	id := out[0].ID

	got, err := env.panels.GetPanel(1, id)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if got.ID != id || got.Caption != "hello" {
		t.Fatalf("unexpected panel: %+v", got)
	}
	if len(got.CharacterIDs) != 1 || got.CharacterIDs[0] != 9 {
		t.Fatalf("CharacterIDs not loaded: %+v", got)
	}
}

// TestGetPanelCrossUser verifies user 2 cannot read user 1's panel.
func TestGetPanelCrossUser(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)

	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{{Caption: "p"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := env.panels.GetPanel(2, out[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user GetPanel should return ErrNotFound, got %v", err)
	}
}

// TestGetPanelUnknown verifies an unknown panel id returns ErrNotFound.
func TestGetPanelUnknown(t *testing.T) {
	env := newPanelTestEnv(t)
	if _, err := env.panels.GetPanel(1, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown GetPanel should return ErrNotFound, got %v", err)
	}
}
