package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// Sentinel errors returned by Service. Callers (e.g. HTTP handlers) match these
// with errors.Is to map to status codes without leaking internal detail.
var (
	// ErrBadInvite is returned by Register when the supplied invite code does
	// not match the current global invite code.
	ErrBadInvite = errors.New("邀请码不正确")
	// ErrInvalidCredentials is returned by Login when the username is unknown or
	// the password does not match. It deliberately does not distinguish between
	// the two so callers cannot probe which usernames exist.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrNicknameTaken is a backwards-compatible alias of ErrUsernameTaken. Login
	// now keys on username (not the display nickname), so registration uniqueness
	// is surfaced via ErrUsernameTaken. This alias is retained so pre-Task-8
	// handler code that still matches ErrNicknameTaken keeps compiling; new code
	// should use ErrUsernameTaken / ErrEmailTaken.
	ErrNicknameTaken = ErrUsernameTaken
)

// Service implements the registration, login, profile and admin/invite seeding
// use cases on top of a user repository, an invite repository and a session
// store.
type Service struct {
	repo          *UserRepo
	invites       *InviteRepo
	sess          *Session
	signupCredits int
}

// NewService wires a Service to its user repository, invite repository and
// session store. signupCredits is the starting image-credit balance granted to
// every newly registered user.
func NewService(repo *UserRepo, invites *InviteRepo, sess *Session, signupCredits int) *Service {
	return &Service{repo: repo, invites: invites, sess: sess, signupCredits: signupCredits}
}

// Sessions exposes the underlying session store so HTTP handlers can resolve
// tokens issued by Login/Register.
func (s *Service) Sessions() *Session { return s.sess }

// RegisterInput carries the fields accepted by Register. Email and Nickname are
// validated/normalized; an empty Nickname falls back to the username.
type RegisterInput struct {
	Username   string
	Password   string
	Email      string
	InviteCode string
	Nickname   string
}

// Register validates the invite code and the incoming profile, hashes the
// password with bcrypt, creates a role="user" account with the configured
// signup credits, and issues a session token so the caller is logged in.
//
// Order: compare invite code (mismatch → ErrBadInvite) → validate username /
// password / email / nickname (each maps to its ErrBad* sentinel) → bcrypt →
// repo.Create (duplicate username/email surface as ErrUsernameTaken /
// ErrEmailTaken) → sess.Issue. The bcrypt hash is never returned in an error or
// logged.
func (s *Service) Register(in RegisterInput) (token string, u models.User, err error) {
	current, err := s.invites.Get()
	if err != nil {
		return "", models.User{}, fmt.Errorf("register: read invite code: %w", err)
	}
	// Defense in depth: an unconfigured invite code ("" from Get) must never
	// open registration. Reject before the equality check so a request carrying
	// InviteCode:"" cannot pass via "" == "" — no invite code means signup is
	// closed, so any input is refused.
	if current == "" || in.InviteCode != current {
		return "", models.User{}, ErrBadInvite
	}

	if err := ValidateUsername(in.Username); err != nil {
		return "", models.User{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return "", models.User{}, err
	}
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return "", models.User{}, err
	}
	nickname, err := NormalizeNickname(in.Nickname, in.Username)
	if err != nil {
		return "", models.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", models.User{}, fmt.Errorf("register: hash password: %w", err)
	}

	u, err = s.repo.Create(NewUser{
		Username:     in.Username,
		Email:        email,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Role:         "user",
		Credits:      s.signupCredits,
	})
	if err != nil {
		// Unique-constraint sentinels are passed through untouched so handlers
		// can map them to field-specific 409s.
		if errors.Is(err, ErrUsernameTaken) || errors.Is(err, ErrEmailTaken) {
			return "", models.User{}, err
		}
		return "", models.User{}, fmt.Errorf("register %q: %w", in.Username, err)
	}

	return s.sess.Issue(u.ID), u, nil
}

// Login verifies credentials by username and, on success, issues a session token
// bound to the user's ID. On unknown username or password mismatch it returns
// ErrInvalidCredentials without revealing which check failed.
func (s *Service) Login(username, password string) (token string, u models.User, err error) {
	u, err = s.repo.ByUsername(username)
	if err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	return s.sess.Issue(u.ID), u, nil
}

// Me returns the current user identified by userID, including the live credit
// balance. It backs the protected GET /api/me endpoint.
func (s *Service) Me(userID int64) (models.User, error) {
	u, err := s.repo.ByID(userID)
	if err != nil {
		return models.User{}, fmt.Errorf("me %d: %w", userID, err)
	}
	return u, nil
}

// UpdateProfile validates the editable profile fields and persists them,
// returning the refreshed user. Nickname falls back to the current username when
// left blank; age and gender must pass their validators.
func (s *Service) UpdateProfile(userID int64, nickname string, age int, gender string) (models.User, error) {
	if err := ValidateAge(age); err != nil {
		return models.User{}, err
	}
	if err := ValidateGender(gender); err != nil {
		return models.User{}, err
	}

	current, err := s.repo.ByID(userID)
	if err != nil {
		return models.User{}, fmt.Errorf("update profile %d: %w", userID, err)
	}
	name, err := NormalizeNickname(nickname, current.Username)
	if err != nil {
		return models.User{}, err
	}

	u, err := s.repo.UpdateProfile(userID, name, age, gender)
	if err != nil {
		return models.User{}, fmt.Errorf("update profile %d: %w", userID, err)
	}
	return u, nil
}

// SetAvatar persists the given avatar URL for the user and returns the refreshed
// user row.
func (s *Service) SetAvatar(userID int64, url string) (models.User, error) {
	u, err := s.repo.SetAvatar(userID, url)
	if err != nil {
		return models.User{}, fmt.Errorf("set avatar %d: %w", userID, err)
	}
	return u, nil
}

// SeedAdmin idempotently ensures an admin account exists. It is meant to run at
// startup: an empty username is a no-op (feature disabled); an invalid username
// or password returns an error so the caller can fatal; an already-present
// username is a no-op (idempotent). Otherwise it bcrypt-hashes the password and
// creates a role="admin" account.
func (s *Service) SeedAdmin(username, password, email string, credits int) error {
	if username == "" {
		return nil
	}
	if err := ValidateUsername(username); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	if _, err := s.repo.ByUsername(username); err == nil {
		// Already seeded on a prior boot; nothing to do.
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("seed admin: hash password: %w", err)
	}

	if _, err := s.repo.Create(NewUser{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Nickname:     username,
		Role:         "admin",
		Credits:      credits,
	}); err != nil {
		return fmt.Errorf("seed admin %q: %w", username, err)
	}
	return nil
}

// InviteCode returns the current global invite code (admin view).
func (s *Service) InviteCode() (string, error) {
	code, err := s.invites.Get()
	if err != nil {
		return "", fmt.Errorf("invite code: %w", err)
	}
	return code, nil
}

// RotateInvite generates and persists a fresh invite code, returning it.
func (s *Service) RotateInvite() (string, error) {
	code, err := s.invites.Rotate()
	if err != nil {
		return "", fmt.Errorf("rotate invite: %w", err)
	}
	return code, nil
}

// SeedInvite ensures an invite code exists and returns the effective value. When
// one already exists it is returned unchanged (idempotent); otherwise preferred
// is used when non-empty, else a random code is generated.
func (s *Service) SeedInvite(preferred string) (string, error) {
	code, err := s.invites.Seed(preferred)
	if err != nil {
		return "", fmt.Errorf("seed invite: %w", err)
	}
	return code, nil
}
