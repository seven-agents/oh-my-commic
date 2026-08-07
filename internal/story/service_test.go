package story

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/panel"
)

// storyTestEnv bundles a story.Service wired to real asset/chapter/panel
// services over an in-memory database, plus a fake DashScope server.
type storyTestEnv struct {
	svc      *Service
	chapters *chapter.Service
	assets   *asset.Service
	books    *book.Repo
	db       *sql.DB
}

// newStoryTestEnv opens an in-memory DB, seeds two users, and wires the full
// stack. content is the raw string returned as the single chat completion.
func newStoryTestEnv(t *testing.T, content string) *storyTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedUsers(t, d, 2)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`))
	}))
	t.Cleanup(ts.Close)

	client := &ai.Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}

	bookRepo := book.NewRepo(d)
	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)

	return &storyTestEnv{
		svc:      NewService(client, assetSvc, chapterSvc, panelSvc),
		chapters: chapterSvc,
		assets:   assetSvc,
		books:    bookRepo,
		db:       d,
	}
}

// newChapter seeds a book owned by userID and a chapter under it.
func (e *storyTestEnv) newChapter(t *testing.T, userID int64) models.Chapter {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := e.chapters.CreateChapter(userID, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	return c
}

// seedUsers inserts n users so books referencing user ids 1..n satisfy the FK.
func seedUsers(t *testing.T, d *sql.DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := d.Exec(
			`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
			"user"+string(rune('0'+i)), "hash",
		); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

func TestConverseReturnsReply(t *testing.T) {
	env := newStoryTestEnv(t, "我们一起想想开头吧！")
	ch := env.newChapter(t, 1)

	reply, err := env.svc.Converse(1, ch.ID, []ai.Msg{{Role: "user", Content: "帮我"}})
	if err != nil {
		t.Fatalf("converse: %v", err)
	}
	if reply != "我们一起想想开头吧！" {
		t.Fatalf("reply 错: %q", reply)
	}
}

func TestConverseCrossUserNotFound(t *testing.T) {
	env := newStoryTestEnv(t, "hi")
	ch := env.newChapter(t, 1)

	_, err := env.svc.Converse(2, ch.ID, []ai.Msg{{Role: "user", Content: "帮我"}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户应 ErrNotFound: %v", err)
	}
}

func TestGenerateStoryboardPersistsPanels(t *testing.T) {
	body := "分镜：[{\"caption\":\"出发\",\"characterIds\":[1],\"sceneId\":2,\"imagePrompt\":\"fox\"},{\"caption\":\"回家\",\"characterIds\":[],\"sceneId\":0,\"imagePrompt\":\"home\"}]"
	env := newStoryTestEnv(t, body)
	ch := env.newChapter(t, 1)

	panels, err := env.svc.GenerateStoryboard(1, ch.ID, nil, 2)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(panels) != 2 {
		t.Fatalf("应有2个分镜, got %d", len(panels))
	}
	if panels[0].Caption != "出发" || panels[0].Status != "pending" {
		t.Fatalf("首个分镜错: %+v", panels[0])
	}
	if len(panels[0].CharacterIDs) != 1 || panels[0].CharacterIDs[0] != 1 || panels[0].SceneID != 2 {
		t.Fatalf("索引解析错: %+v", panels[0])
	}

	// Chapter status must advance to storyboarding.
	got, err := env.chapters.GetChapter(1, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Status != "storyboarding" {
		t.Fatalf("章节状态应为 storyboarding, got %q", got.Status)
	}
}

func TestGenerateStoryboardCrossUserNotFound(t *testing.T) {
	env := newStoryTestEnv(t, "[]")
	ch := env.newChapter(t, 1)

	_, err := env.svc.GenerateStoryboard(2, ch.ID, nil, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户应 ErrNotFound: %v", err)
	}
}

func TestGenerateStoryboardBadJSONErrors(t *testing.T) {
	env := newStoryTestEnv(t, "抱歉无法生成")
	ch := env.newChapter(t, 1)

	_, err := env.svc.GenerateStoryboard(1, ch.ID, nil, 1)
	if err == nil {
		t.Fatal("无 JSON 应报错")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("AI 解析错误不应是 ErrNotFound: %v", err)
	}
}
