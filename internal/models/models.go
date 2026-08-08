// Package models defines the core domain data structures shared across the
// oh-my-commic backend. Field names and JSON tags are part of the public API
// contract consumed by other packages and the frontend, so they must remain
// stable.
package models

// User is an application account. PasswordHash is never serialized to JSON.
type User struct {
	ID           int64  `json:"id"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"createdAt"`
}

// Book is a comic book owned by a User.
type Book struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"userId"`
	Title     string `json:"title"`
	CoverURL  string `json:"coverUrl"`
	Style     string `json:"style"`
	Summary   string `json:"summary"`
	IsPublic  bool   `json:"isPublic"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Character is a cast member (person, creature, etc.) within a Book.
type Character struct {
	ID          int64  `json:"id"`
	BookID      int64  `json:"bookId"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Age         string `json:"age"`
	Personality string `json:"personality"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
}

// Scene is a reusable setting/background within a Book.
type Scene struct {
	ID          int64  `json:"id"`
	BookID      int64  `json:"bookId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
}

// Chapter is an ordered section of a Book.
//
// Summary is an AI-polished, warm Chinese overview (2-4 sentences) of the
// chapter's story, produced as a side-output of the storyboard-chat turn and
// shown on the book reader's per-chapter page.
type Chapter struct {
	ID        int64  `json:"id"`
	BookID    int64  `json:"bookId"`
	Order     int    `json:"order"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"createdAt"`
}

// Panel is a single comic frame within a Chapter.
//
// Location, Event and CharExpressions carry the structured storyboard fields: a
// panel's setting, the action, and each present character's facial expression
// keyed by character id. CharacterIDs remains the flat list of present character
// ids (derived from the characters the model referenced) and is what the render
// layer uses to pick reference images.
type Panel struct {
	ID              int64            `json:"id"`
	ChapterID       int64            `json:"chapterId"`
	Order           int              `json:"order"`
	Caption         string           `json:"caption"`
	CharacterIDs    []int64          `json:"characterIds"`
	SceneID         int64            `json:"sceneId"`
	ImagePrompt     string           `json:"imagePrompt"`
	ImageURL        string           `json:"imageUrl"`
	Status          string           `json:"status"`
	Location        string           `json:"location"`
	Event           string           `json:"event"`
	CharExpressions map[int64]string `json:"charExpressions"`
}
