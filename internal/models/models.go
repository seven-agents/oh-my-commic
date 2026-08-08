// Package models defines the core domain data structures shared across the
// oh-my-commic backend. Field names and JSON tags are part of the public API
// contract consumed by other packages and the frontend, so they must remain
// stable.
package models

// User is an application account. PasswordHash is never serialized to JSON.
// Credits is the user's remaining image-generation balance (panel render + asset
// comicify each cost some credits, charged up front and refunded on failure).
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Age          int    `json:"age"`
	Gender       string `json:"gender"`
	AvatarURL    string `json:"avatarUrl"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"createdAt"`
	Credits      int    `json:"credits"`
}

// Book is a comic book owned by a User.
type Book struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	Title       string `json:"title"`
	CoverURL    string `json:"coverUrl"`
	Style       string `json:"style"`
	Summary     string `json:"summary"`
	IsPublic    bool   `json:"isPublic"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	LikeCount   int    `json:"likeCount"`
	ViewCount   int    `json:"viewCount"`
	PublishedAt string `json:"publishedAt"`
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

// ConversationMsg is one turn of the persisted storyboard-chat conversation: a
// role ("user" / "assistant") and its text content. The full ordered slice is
// stored on the chapter (as a JSON TEXT column) so re-entering the chapter
// editor can restore the chat history and the user can keep refining.
type ConversationMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chapter is an ordered section of a Book.
//
// Summary is an AI-polished, warm Chinese overview (2-4 sentences) of the
// chapter's story, produced as a side-output of the storyboard-chat turn and
// shown on the book reader's per-chapter page.
//
// Conversation is the persisted stage-1 storyboard-chat history and PanelCount
// is the target number of frames chosen for the chapter; together they let the
// chat editor restore state across sessions.
type Chapter struct {
	ID           int64             `json:"id"`
	BookID       int64             `json:"bookId"`
	Order        int               `json:"order"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Summary      string            `json:"summary"`
	IsCover      bool              `json:"isCover"`
	CreatedAt    string            `json:"createdAt"`
	Conversation []ConversationMsg `json:"conversation"`
	PanelCount   int               `json:"panelCount"`
}

// Panel is a single comic frame within a Chapter.
//
// Content is the stage-1 basic Chinese description of the frame ("what happens
// here") produced by the storyboard chat; it is the source the stage-2
// ProcessPanel step decomposes into the structured fields below.
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
	Content         string           `json:"content"`
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
