package auth

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newUser is a small helper to build a NewUser with sensible defaults for tests.
func newUser(username, email, nickname string, credits int) NewUser {
	return NewUser{
		Username:     username,
		Email:        email,
		PasswordHash: "hash",
		Nickname:     nickname,
		Role:         "user",
		Credits:      credits,
	}
}

func TestCreateAndFetch(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	u, err := repo.Create(newUser("xiaoming", "xiaoming@example.com", "小明", 100))
	if err != nil || u.ID == 0 {
		t.Fatalf("create failed: %v", err)
	}
	if u.Username != "xiaoming" || u.Email != "xiaoming@example.com" {
		t.Fatalf("create returned unexpected username/email: %+v", u)
	}
	if u.Nickname != "小明" || u.PasswordHash != "hash" || u.CreatedAt == "" {
		t.Fatalf("create returned unexpected user: %+v", u)
	}
	if u.Role != "user" {
		t.Fatalf("create should persist role, got %q", u.Role)
	}
	if u.Credits != 100 {
		t.Fatalf("create should grant 100 credits, got %d", u.Credits)
	}

	// A second, distinct user also succeeds.
	u2, err := repo.Create(newUser("xiaohong", "xiaohong@example.com", "小红", 50))
	if err != nil || u2.ID == 0 || u2.ID == u.ID {
		t.Fatalf("second create failed: err=%v u2=%+v", err, u2)
	}

	got, err := repo.ByUsername("xiaoming")
	if err != nil || got.ID != u.ID {
		t.Fatalf("fetch mismatch: err=%v got=%+v want=%+v", err, got, u)
	}
	if got.PasswordHash != "hash" {
		t.Fatalf("fetched password_hash mismatch: %q", got.PasswordHash)
	}

	// Duplicate username maps to ErrUsernameTaken.
	if _, err := repo.Create(newUser("xiaoming", "other@example.com", "另一个", 100)); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("重复 username 应返回 ErrUsernameTaken, got %v", err)
	}

	// Duplicate email maps to ErrEmailTaken.
	if _, err := repo.Create(newUser("otheruser", "xiaoming@example.com", "另一个", 100)); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("重复 email 应返回 ErrEmailTaken, got %v", err)
	}
}

func TestByUsernameNotFound(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	if _, err := repo.ByUsername("nobody"); err == nil {
		t.Fatal("查询不存在的用户名应报错")
	}
}

func TestUpdateProfile(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)
	u, err := repo.Create(newUser("profileuser", "p@example.com", "旧昵称", 10))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.UpdateProfile(u.ID, "新昵称", 8, "female")
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Nickname != "新昵称" || updated.Age != 8 || updated.Gender != "female" {
		t.Fatalf("profile not updated: %+v", updated)
	}
	// Username/email/credits are untouched by profile update.
	if updated.Username != "profileuser" || updated.Email != "p@example.com" || updated.Credits != 10 {
		t.Fatalf("profile update clobbered immutable fields: %+v", updated)
	}

	// Re-read to confirm persistence.
	got, err := repo.ByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.Nickname != "新昵称" || got.Age != 8 || got.Gender != "female" {
		t.Fatalf("profile not persisted: %+v", got)
	}
}

func TestSetAvatar(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)
	u, err := repo.Create(newUser("avataruser", "a@example.com", "头像", 10))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.SetAvatar(u.ID, "/media/avatars/1.png")
	if err != nil {
		t.Fatalf("set avatar: %v", err)
	}
	if updated.AvatarURL != "/media/avatars/1.png" {
		t.Fatalf("avatar not set: %+v", updated)
	}

	got, err := repo.ByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.AvatarURL != "/media/avatars/1.png" {
		t.Fatalf("avatar not persisted: %+v", got)
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
	u, err := repo.Create(newUser("huahua", "hua@example.com", "花花", 2))
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
	u, err := repo.Create(newUser("xiaoman", "man@example.com", "小满", 1))
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
