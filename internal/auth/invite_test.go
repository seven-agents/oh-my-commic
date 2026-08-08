package auth

import (
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newInviteRepo opens an in-memory, migrated DB and returns an InviteRepo bound
// to it. The DB is closed on test cleanup.
func newInviteRepo(t *testing.T) *InviteRepo {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return NewInviteRepo(d)
}

func TestInviteGetEmptyDB(t *testing.T) {
	repo := newInviteRepo(t)
	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "" {
		t.Fatalf("Get on empty DB = %q, want \"\"", got)
	}
}

func TestInviteSetAndGet(t *testing.T) {
	repo := newInviteRepo(t)
	if err := repo.Set("welcome"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "welcome" {
		t.Fatalf("Get after Set = %q, want %q", got, "welcome")
	}
	// Set again upserts the same row (no duplicate-key error).
	if err := repo.Set("changed"); err != nil {
		t.Fatalf("Set (upsert): %v", err)
	}
	got, err = repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "changed" {
		t.Fatalf("Get after upsert = %q, want %q", got, "changed")
	}
}

func TestInviteSeedIdempotent(t *testing.T) {
	repo := newInviteRepo(t)

	first, err := repo.Seed("welcome")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if first != "welcome" {
		t.Fatalf("first Seed = %q, want %q", first, "welcome")
	}

	// Seeding again must not overwrite the existing value.
	second, err := repo.Seed("other")
	if err != nil {
		t.Fatalf("Seed (repeat): %v", err)
	}
	if second != "welcome" {
		t.Fatalf("second Seed = %q, want %q (idempotent)", second, "welcome")
	}

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "welcome" {
		t.Fatalf("Get after repeat Seed = %q, want %q", got, "welcome")
	}
}

func TestInviteSeedEmptyPreferredGeneratesCode(t *testing.T) {
	repo := newInviteRepo(t)

	code, err := repo.Seed("")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	assertCode(t, code)

	// A second Seed with an empty preferred returns the persisted value.
	again, err := repo.Seed("")
	if err != nil {
		t.Fatalf("Seed (repeat): %v", err)
	}
	if again != code {
		t.Fatalf("second Seed = %q, want %q (idempotent)", again, code)
	}
}

func TestInviteRotate(t *testing.T) {
	repo := newInviteRepo(t)

	old, err := repo.Seed("welcome")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	rotated, err := repo.Rotate()
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	assertCode(t, rotated)
	if rotated == old {
		t.Fatalf("Rotate returned unchanged code %q", rotated)
	}

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != rotated {
		t.Fatalf("Get after Rotate = %q, want %q", got, rotated)
	}
}

func TestInviteRotateOnEmptyDB(t *testing.T) {
	repo := newInviteRepo(t)

	code, err := repo.Rotate()
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	assertCode(t, code)

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != code {
		t.Fatalf("Get after Rotate = %q, want %q", got, code)
	}
}

func TestRandomCode(t *testing.T) {
	const allowed = "abcdefghijklmnopqrstuvwxyz0123456789"
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := randomCode()
		if len(code) != 10 {
			t.Fatalf("randomCode() length = %d, want 10 (%q)", len(code), code)
		}
		for _, c := range code {
			if !containsRune(allowed, c) {
				t.Fatalf("randomCode() = %q contains illegal char %q", code, c)
			}
		}
		seen[code] = true
	}
	// With a 36^10 space, 100 draws being all-identical is effectively
	// impossible; a single collision would still pass, but all-equal signals a
	// broken generator.
	if len(seen) == 1 {
		t.Fatalf("randomCode() produced the same value 100 times")
	}
}

// assertCode fails the test unless code is a valid 10-char [a-z0-9] string.
func assertCode(t *testing.T, code string) {
	t.Helper()
	const allowed = "abcdefghijklmnopqrstuvwxyz0123456789"
	if len(code) != 10 {
		t.Fatalf("code length = %d, want 10 (%q)", len(code), code)
	}
	for _, c := range code {
		if !containsRune(allowed, c) {
			t.Fatalf("code %q contains illegal char %q", code, c)
		}
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
