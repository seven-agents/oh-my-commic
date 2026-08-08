package auth

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newTestService builds a Service backed by a fresh in-memory database with a
// seeded invite code, so each test runs against an isolated schema. The seeded
// invite code is returned for use in Register inputs.
func newTestService(t *testing.T, signupCredits int) (*Service, string) {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	invites := NewInviteRepo(d)
	code, err := invites.Seed("invite01")
	if err != nil {
		t.Fatalf("seed invite: %v", err)
	}

	svc := NewService(NewUserRepo(d), invites, NewSession(nil), signupCredits)
	return svc, code
}

// validRegister returns a RegisterInput with all fields valid, using the given
// invite code and username. Nickname is left empty to exercise the fallback.
func validRegister(code, username string) RegisterInput {
	return RegisterInput{
		Username:   username,
		Password:   "pw123456",
		Email:      username + "@example.com",
		InviteCode: code,
	}
}

func TestRegisterSuccess(t *testing.T) {
	svc, code := newTestService(t, 42)

	tok, u, err := svc.Register(validRegister(code, "xiaoming"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if tok == "" {
		t.Fatal("register should return a non-empty session token")
	}
	if u.Role != "user" {
		t.Fatalf("new user role should be user, got %q", u.Role)
	}
	if u.Credits != 42 {
		t.Fatalf("register should grant 42 credits, got %d", u.Credits)
	}
	// Empty nickname falls back to the username.
	if u.Nickname != "xiaoming" {
		t.Fatalf("nickname should default to username, got %q", u.Nickname)
	}
	if u.Email != "xiaoming@example.com" {
		t.Fatalf("email should be normalized/stored, got %q", u.Email)
	}

	// The issued token resolves to the new user.
	gotID, ok := svc.Sessions().UserID(tok)
	if !ok || gotID != u.ID {
		t.Fatalf("session should resolve to user id %d, got %d ok=%v", u.ID, gotID, ok)
	}

	// Me returns the same user with the live balance.
	me, err := svc.Me(u.ID)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if me.ID != u.ID || me.Credits != 42 {
		t.Fatalf("me mismatch: id=%d credits=%d", me.ID, me.Credits)
	}
}

func TestRegisterKeepsExplicitNickname(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "withname")
	in.Nickname = "小明同学"

	_, u, err := svc.Register(in)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Nickname != "小明同学" {
		t.Fatalf("explicit nickname should be kept, got %q", u.Nickname)
	}
}

func TestRegisterWrongInviteCode(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "baduser")
	in.InviteCode = "totally-wrong"

	_, _, err := svc.Register(in)
	if !errors.Is(err, ErrBadInvite) {
		t.Fatalf("wrong invite code should return ErrBadInvite, got %v", err)
	}
}

func TestRegisterWeakPassword(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "weakpw")
	in.Password = "short" // too short and no digit

	_, _, err := svc.Register(in)
	if !errors.Is(err, ErrBadPassword) {
		t.Fatalf("weak password should return ErrBadPassword, got %v", err)
	}
}

func TestRegisterBadUsername(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "AB") // too short, uppercase

	_, _, err := svc.Register(in)
	if !errors.Is(err, ErrBadUsername) {
		t.Fatalf("bad username should return ErrBadUsername, got %v", err)
	}
}

func TestRegisterBadEmail(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "bademail")
	in.Email = "not-an-email"

	_, _, err := svc.Register(in)
	if !errors.Is(err, ErrBadEmail) {
		t.Fatalf("bad email should return ErrBadEmail, got %v", err)
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	svc, code := newTestService(t, 10)
	if _, _, err := svc.Register(validRegister(code, "dupe")); err != nil {
		t.Fatalf("first register: %v", err)
	}

	// Same username, different email.
	second := validRegister(code, "dupe")
	second.Email = "other@example.com"
	_, _, err := svc.Register(second)
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("duplicate username should return ErrUsernameTaken, got %v", err)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, code := newTestService(t, 10)
	if _, _, err := svc.Register(validRegister(code, "userone")); err != nil {
		t.Fatalf("first register: %v", err)
	}

	second := validRegister(code, "usertwo")
	second.Email = "userone@example.com"
	_, _, err := svc.Register(second)
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email should return ErrEmailTaken, got %v", err)
	}
}

func TestRegisterDoesNotStorePlaintext(t *testing.T) {
	svc, code := newTestService(t, 10)
	in := validRegister(code, "secretpw")
	in.Password = "supersecret1"

	_, u, err := svc.Register(in)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.PasswordHash == "supersecret1" || u.PasswordHash == "" {
		t.Fatalf("password must be hashed, got %q", u.PasswordHash)
	}
}

func TestLoginSuccess(t *testing.T) {
	svc, code := newTestService(t, 10)
	if _, _, err := svc.Register(validRegister(code, "loginok")); err != nil {
		t.Fatalf("register: %v", err)
	}

	tok, u, err := svc.Login("loginok", "pw123456")
	if err != nil || tok == "" {
		t.Fatalf("login should succeed: tok=%q err=%v", tok, err)
	}
	gotID, ok := svc.Sessions().UserID(tok)
	if !ok || gotID != u.ID {
		t.Fatalf("session should resolve to user id %d, got %d ok=%v", u.ID, gotID, ok)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, code := newTestService(t, 10)
	if _, _, err := svc.Register(validRegister(code, "wrongpw")); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, _, err := svc.Login("wrongpw", "nope")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password should return ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUsername(t *testing.T) {
	svc, _ := newTestService(t, 10)
	_, _, err := svc.Login("nobody", "pw123456")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown username should return ErrInvalidCredentials, got %v", err)
	}
}

func TestServiceUpdateProfile(t *testing.T) {
	svc, code := newTestService(t, 10)
	_, u, err := svc.Register(validRegister(code, "profuser"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	updated, err := svc.UpdateProfile(u.ID, "新昵称", 8, "男")
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Nickname != "新昵称" || updated.Age != 8 || updated.Gender != "男" {
		t.Fatalf("profile not updated: %+v", updated)
	}
}

func TestUpdateProfileBadAge(t *testing.T) {
	svc, code := newTestService(t, 10)
	_, u, err := svc.Register(validRegister(code, "badage"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := svc.UpdateProfile(u.ID, "n", 999, "男"); !errors.Is(err, ErrBadAge) {
		t.Fatalf("bad age should return ErrBadAge, got %v", err)
	}
}

func TestUpdateProfileBadGender(t *testing.T) {
	svc, code := newTestService(t, 10)
	_, u, err := svc.Register(validRegister(code, "badgender"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := svc.UpdateProfile(u.ID, "n", 8, "unknown"); !errors.Is(err, ErrBadGender) {
		t.Fatalf("bad gender should return ErrBadGender, got %v", err)
	}
}

func TestServiceSetAvatar(t *testing.T) {
	svc, code := newTestService(t, 10)
	_, u, err := svc.Register(validRegister(code, "avataruser"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	updated, err := svc.SetAvatar(u.ID, "/media/a.png")
	if err != nil {
		t.Fatalf("set avatar: %v", err)
	}
	if updated.AvatarURL != "/media/a.png" {
		t.Fatalf("avatar not set, got %q", updated.AvatarURL)
	}
}

func TestSeedAdminCreatesAdmin(t *testing.T) {
	svc, _ := newTestService(t, 10)
	if err := svc.SeedAdmin("adminuser", "adminpw12", "admin@example.com", 500); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	tok, u, err := svc.Login("adminuser", "adminpw12")
	if err != nil || tok == "" {
		t.Fatalf("admin should be able to log in: %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("seeded user should have role admin, got %q", u.Role)
	}
	if u.Credits != 500 {
		t.Fatalf("admin credits should be 500, got %d", u.Credits)
	}
}

func TestSeedAdminIdempotent(t *testing.T) {
	svc, _ := newTestService(t, 10)
	if err := svc.SeedAdmin("adminuser", "adminpw12", "admin@example.com", 500); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	// Second call with a different password must be a no-op (idempotent).
	if err := svc.SeedAdmin("adminuser", "different9", "admin@example.com", 999); err != nil {
		t.Fatalf("second seed should be a no-op, got %v", err)
	}
	// Original password still works; new one does not (row was not replaced).
	if _, _, err := svc.Login("adminuser", "adminpw12"); err != nil {
		t.Fatalf("original admin password should still work: %v", err)
	}
	if _, _, err := svc.Login("adminuser", "different9"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("new password should not work after idempotent skip, got %v", err)
	}
}

func TestSeedAdminEmptyUsernameSkips(t *testing.T) {
	svc, _ := newTestService(t, 10)
	if err := svc.SeedAdmin("", "adminpw12", "admin@example.com", 500); err != nil {
		t.Fatalf("empty username should be a no-op, got %v", err)
	}
	// No admin was created, so login fails with invalid credentials.
	if _, _, err := svc.Login("admin", "adminpw12"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("no admin should exist, got %v", err)
	}
}

func TestSeedAdminInvalidUsername(t *testing.T) {
	svc, _ := newTestService(t, 10)
	if err := svc.SeedAdmin("AB", "adminpw12", "admin@example.com", 500); !errors.Is(err, ErrBadUsername) {
		t.Fatalf("invalid username should return ErrBadUsername, got %v", err)
	}
}

func TestSeedAdminInvalidPassword(t *testing.T) {
	svc, _ := newTestService(t, 10)
	if err := svc.SeedAdmin("adminuser", "short", "admin@example.com", 500); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("invalid password should return ErrBadPassword, got %v", err)
	}
}

func TestInviteCodeAndRotate(t *testing.T) {
	svc, code := newTestService(t, 10)

	got, err := svc.InviteCode()
	if err != nil {
		t.Fatalf("invite code: %v", err)
	}
	if got != code {
		t.Fatalf("invite code should be %q, got %q", code, got)
	}

	rotated, err := svc.RotateInvite()
	if err != nil {
		t.Fatalf("rotate invite: %v", err)
	}
	if rotated == "" || rotated == code {
		t.Fatalf("rotate should produce a new non-empty code, got %q", rotated)
	}

	// Registration now requires the rotated code; the old one is rejected.
	if _, _, err := svc.Register(validRegister(code, "olddie")); !errors.Is(err, ErrBadInvite) {
		t.Fatalf("old invite code should be rejected after rotate, got %v", err)
	}
	if _, _, err := svc.Register(validRegister(rotated, "newok")); err != nil {
		t.Fatalf("register with rotated code should succeed: %v", err)
	}
}

func TestSeedInviteIdempotent(t *testing.T) {
	svc, code := newTestService(t, 10)
	got, err := svc.SeedInvite("another")
	if err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	if got != code {
		t.Fatalf("seed invite should be idempotent and return %q, got %q", code, got)
	}
}
