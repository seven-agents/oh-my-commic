package community

const (
	defaultLimit = 20
	maxLimit     = 50
)

// Service applies input clamping at the boundary and delegates to Repo.
type Service struct {
	repo *Repo
}

// NewService wires a Service to its Repo.
func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// clampPaging normalizes caller paging into safe bounds (never errors).
func clampPaging(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ListPublic returns the public feed. sort is passed through to the repo, which
// whitelists it (unknown -> newest-first). limit/offset are clamped.
func (s *Service) ListPublic(viewerKey, sort string, limit, offset int) ([]CommunityBook, error) {
	limit, offset = clampPaging(limit, offset)
	return s.repo.ListPublic(viewerKey, sort, limit, offset)
}

// GetPublicDetail returns one public book's full read payload, or ErrNotFound.
func (s *Service) GetPublicDetail(viewerKey string, bookID int64) (CommunityBookDetail, error) {
	return s.repo.GetPublicDetail(viewerKey, bookID)
}

// Like / Unlike / RecordView delegate to the repo.
func (s *Service) Like(userID, bookID int64) (LikeResult, error)   { return s.repo.Like(userID, bookID) }
func (s *Service) Unlike(userID, bookID int64) (LikeResult, error) { return s.repo.Unlike(userID, bookID) }
func (s *Service) RecordView(bookID int64, viewerKey string) error {
	return s.repo.RecordView(bookID, viewerKey)
}
