package auth

import (
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestCreateAndFetch(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	u, err := repo.Create("小明", "hash", 100)
	if err != nil || u.ID == 0 {
		t.Fatalf("create failed: %v", err)
	}
	if u.Nickname != "小明" || u.PasswordHash != "hash" || u.CreatedAt == "" {
		t.Fatalf("create returned unexpected user: %+v", u)
	}
	if u.Credits != 100 {
		t.Fatalf("create should grant 100 credits, got %d", u.Credits)
	}

	got, err := repo.ByNickname("小明")
	if err != nil || got.ID != u.ID {
		t.Fatalf("fetch mismatch: err=%v got=%+v want=%+v", err, got, u)
	}
	if got.PasswordHash != "hash" {
		t.Fatalf("fetched password_hash mismatch: %q", got.PasswordHash)
	}

	if _, err := repo.Create("小明", "hash2", 100); err == nil {
		t.Fatal("重复昵称应报错")
	}
}

// TestSpendAndRefund exercises the credit ledger: a spend within balance
// succeeds and deducts, a spend that would overdraw is rejected without change,
// a spend of the exact remaining balance (down to 0) succeeds, a spend at 0
// balance is rejected, and refund restores credits.
func TestSpendAndRefund(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)
	u, err := repo.Create("花花", "hash", 2)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ok, err := repo.Spend(u.ID, 1)
	if err != nil || !ok {
		t.Fatalf("first spend should succeed: ok=%v err=%v", ok, err)
	}
	if got, _ := repo.Credits(u.ID); got != 1 {
		t.Fatalf("balance after spend should be 1, got %d", got)
	}

	// Spend the exact remaining balance (1 → 0) succeeds.
	ok, err = repo.Spend(u.ID, 1)
	if err != nil || !ok {
		t.Fatalf("spending exact remaining balance should succeed: ok=%v err=%v", ok, err)
	}
	if got, _ := repo.Credits(u.ID); got != 0 {
		t.Fatalf("balance should be 0, got %d", got)
	}

	// Spending at 0 balance is rejected and leaves the balance untouched.
	ok, err = repo.Spend(u.ID, 1)
	if err != nil {
		t.Fatalf("spend error: %v", err)
	}
	if ok {
		t.Fatal("spend at 0 balance should return false")
	}
	if got, _ := repo.Credits(u.ID); got != 0 {
		t.Fatalf("rejected spend must not change balance, got %d", got)
	}

	// Refund restores credits.
	if err := repo.Refund(u.ID, 3); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if got, _ := repo.Credits(u.ID); got != 3 {
		t.Fatalf("balance after refund should be 3, got %d", got)
	}
}

// TestSpendRejectsWhenCostExceedsBalance verifies a single over-budget spend is
// rejected as a whole (no partial deduction).
func TestSpendRejectsWhenCostExceedsBalance(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)
	u, err := repo.Create("小满", "hash", 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ok, err := repo.Spend(u.ID, 5)
	if err != nil {
		t.Fatalf("spend error: %v", err)
	}
	if ok {
		t.Fatal("spend exceeding balance should return false")
	}
	if got, _ := repo.Credits(u.ID); got != 1 {
		t.Fatalf("rejected spend must not deduct, balance=%d", got)
	}
}

func TestByNicknameNotFound(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	if _, err := repo.ByNickname("不存在"); err == nil {
		t.Fatal("查询不存在的昵称应报错")
	}
}
