package auth

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newTestService builds a Service backed by a fresh in-memory database so each
// test runs against an isolated schema.
func newTestService(t *testing.T) *Service {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	return NewService(NewUserRepo(d), NewSession(nil))
}

func TestRegisterLogin(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Register("小明", "pw123456"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login("小明", "wrong"); err == nil {
		t.Fatal("错误密码应失败")
	}
	tok, u, err := svc.Login("小明", "pw123456")
	if err != nil || tok == "" || u.Nickname != "小明" {
		t.Fatalf("登录应成功: %v", err)
	}
}

func TestRegisterDuplicateNickname(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Register("小红", "pw123456"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Register("小红", "another")
	if !errors.Is(err, ErrNicknameTaken) {
		t.Fatalf("重复昵称应返回 ErrNicknameTaken, got %v", err)
	}
}

func TestLoginUnknownNicknameIsInvalidCredentials(t *testing.T) {
	svc := newTestService(t)
	_, _, err := svc.Login("不存在", "pw123456")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("未知昵称应返回 ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginWrongPasswordIsInvalidCredentials(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Register("阿强", "pw123456"); err != nil {
		t.Fatal(err)
	}
	_, _, err := svc.Login("阿强", "nope")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("错误密码应返回 ErrInvalidCredentials, got %v", err)
	}
}

func TestRegisterDoesNotStorePlaintext(t *testing.T) {
	svc := newTestService(t)
	u, err := svc.Register("保密", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash == "supersecret" || u.PasswordHash == "" {
		t.Fatalf("password must be hashed, got %q", u.PasswordHash)
	}
}

func TestLoginIssuesResolvableSession(t *testing.T) {
	svc := newTestService(t)
	u, err := svc.Register("会话", "pw123456")
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := svc.Login("会话", "pw123456")
	if err != nil {
		t.Fatal(err)
	}
	gotID, ok := svc.Sessions().UserID(tok)
	if !ok || gotID != u.ID {
		t.Fatalf("session should resolve to user id %d, got %d ok=%v", u.ID, gotID, ok)
	}
}
