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
	// ErrNicknameTaken is returned by Register when the nickname already exists.
	ErrNicknameTaken = errors.New("nickname already taken")
	// ErrInvalidCredentials is returned by Login when the nickname is unknown or
	// the password does not match. It deliberately does not distinguish between
	// the two so callers cannot probe which nicknames exist.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Service implements the registration and login use cases on top of a user
// repository and a session store.
type Service struct {
	repo          *UserRepo
	sess          *Session
	signupCredits int
}

// NewService wires a Service to its user repository and session store.
// signupCredits is the starting image-credit balance granted to every newly
// registered user.
func NewService(repo *UserRepo, sess *Session, signupCredits int) *Service {
	return &Service{repo: repo, sess: sess, signupCredits: signupCredits}
}

// Sessions exposes the underlying session store so HTTP handlers can resolve
// tokens issued by Login.
func (s *Service) Sessions() *Session { return s.sess }

// Register hashes the password with bcrypt and creates a new user.
//
// It returns ErrNicknameTaken if the login name is already in use. The taken
// check is done with a ByUsername pre-check: there is a TOCTOU window between
// the check and the insert, but the users.username unique index remains the
// authoritative guard — a racing duplicate insert still fails at the DB and
// surfaces as a (non-sentinel) create error rather than a corrupt second row.
//
// NOTE (Task 4 stub): until Task 6 splits login username from display nickname,
// the single incoming name is used as both the account's username (login key)
// and its nickname. The signature is unchanged so service/handler tests and
// wiring keep compiling; Task 6 will introduce the real username/email flow.
func (s *Service) Register(nickname, password string) (models.User, error) {
	if _, err := s.repo.ByUsername(nickname); err == nil {
		return models.User{}, ErrNicknameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.repo.Create(NewUser{
		Username:     nickname,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Role:         "user",
		Credits:      s.signupCredits,
	})
	if err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			return models.User{}, ErrNicknameTaken
		}
		return models.User{}, fmt.Errorf("register %q: %w", nickname, err)
	}
	return u, nil
}

// Me returns the current user identified by userID, including the live credit
// balance. It is used by the protected GET /api/me endpoint so the frontend can
// display and refresh the header credit count.
func (s *Service) Me(userID int64) (models.User, error) {
	u, err := s.repo.ByID(userID)
	if err != nil {
		return models.User{}, fmt.Errorf("me %d: %w", userID, err)
	}
	return u, nil
}

// Login verifies credentials and, on success, issues a session token bound to
// the user's ID. On unknown login name or password mismatch it returns
// ErrInvalidCredentials without revealing which check failed.
//
// NOTE (Task 4 stub): the incoming name is treated as the account username
// (login key); Task 6 will formalize the username/email login flow.
func (s *Service) Login(nickname, password string) (token string, u models.User, err error) {
	u, err = s.repo.ByUsername(nickname)
	if err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	return s.sess.Issue(u.ID), u, nil
}
