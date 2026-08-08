// Package community serves the public, read-mostly community surface: a feed of
// books their owners have made public, a full read-only view of one public book,
// and per-book like / unique-view counters. Every read is filtered to
// is_public=1 and leaks no owner account fields beyond nickname + avatar.
package community

import "github.com/seven-agents/oh-my-commic/internal/models"

// Author is the minimal, privacy-safe attribution shown on community surfaces.
// It deliberately excludes username, email, and every other account field.
type Author struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

// CommunityBook is one feed list item: a public book plus its author, counters,
// and whether the requesting user has liked it (always false for anonymous).
type CommunityBook struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	CoverURL    string `json:"coverUrl"`
	Summary     string `json:"summary"`
	Author      Author `json:"author"`
	LikeCount   int    `json:"likeCount"`
	ViewCount   int    `json:"viewCount"`
	Liked       bool   `json:"liked"`
	PublishedAt string `json:"publishedAt"`
}

// ReaderChapter is a public, read-only chapter: only the fields the reader
// renders. It omits conversation and panelCount (storyboard-chat internals).
type ReaderChapter struct {
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	Order   int            `json:"order"`
	IsCover bool           `json:"isCover"`
	Panels  []models.Panel `json:"panels"`
}

// CommunityBookDetail is the full public read payload for one book: book meta +
// author + counters + liked + all chapters (each carrying its panels).
type CommunityBookDetail struct {
	ID        int64           `json:"id"`
	Title     string          `json:"title"`
	CoverURL  string          `json:"coverUrl"`
	Summary   string          `json:"summary"`
	Style     string          `json:"style"`
	Author    Author          `json:"author"`
	LikeCount int             `json:"likeCount"`
	ViewCount int             `json:"viewCount"`
	Liked     bool            `json:"liked"`
	Chapters  []ReaderChapter `json:"chapters"`
}

// LikeResult is returned by like / unlike: the fresh count and the new state.
type LikeResult struct {
	LikeCount int  `json:"likeCount"`
	Liked     bool `json:"liked"`
}
