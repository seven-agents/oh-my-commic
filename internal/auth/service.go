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
	repo *UserRepo
	sess *Session
}

// NewService wires a Service to its user repository and session store.
func NewService(repo *UserRepo, sess *Session) *Service {
	return &Service{repo: repo, sess: sess}
}

// Sessions exposes the underlying session store so HTTP handlers can resolve
// tokens issued by Login.
func (s *Service) Sessions() *Session { return s.sess }

// Register hashes the password with bcrypt and creates a new user.
//
// It returns ErrNicknameTaken if the nickname is already in use. The taken
// check is done with a ByNickname pre-check: there is a TOCTOU window between
// the check and the insert, but the users.nickname UNIQUE constraint remains
// the authoritative guard — a racing duplicate insert still fails at the DB and
// surfaces as a (non-sentinel) create error rather than a corrupt second row.
func (s *Service) Register(nickname, password string) (models.User, error) {
	if _, err := s.repo.ByNickname(nickname); err == nil {
		return models.User{}, ErrNicknameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.repo.Create(nickname, string(hash))
	if err != nil {
		return models.User{}, fmt.Errorf("register %q: %w", nickname, err)
	}
	return u, nil
}

// Login verifies credentials and, on success, issues a session token bound to
// the user's ID. On unknown nickname or password mismatch it returns
// ErrInvalidCredentials without revealing which check failed.
func (s *Service) Login(nickname, password string) (token string, u models.User, err error) {
	u, err = s.repo.ByNickname(nickname)
	if err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	return s.sess.Issue(u.ID), u, nil
}
