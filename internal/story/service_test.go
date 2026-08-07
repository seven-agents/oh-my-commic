package story

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
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

	mu      sync.Mutex
	content string
}

// setContent updates the raw string the fake DashScope server returns as the
// single chat completion. Guarded by a mutex so it is race-safe.
func (e *storyTestEnv) setContent(content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.content = content
}

func (e *storyTestEnv) getContent() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.content
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

	env := &storyTestEnv{content: content}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":` + strconv.Quote(env.getContent()) + `}}]}`))
	}))
	t.Cleanup(ts.Close)

	client := &ai.Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}

	bookRepo := book.NewRepo(d)
	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)

	env.svc = NewService(client, assetSvc, chapterSvc, panelSvc)
	env.chapters = chapterSvc
	env.assets = assetSvc
	env.books = bookRepo
	env.db = d
	return env
}

// newChapter seeds a book owned by userID and a chapter under it.
func (e *storyTestEnv) newChapter(t *testing.T, userID int64) models.Chapter {
	t.Helper()
	ch, _ := e.newChapterWithBook(t, userID)
	return ch
}

// newChapterWithBook seeds a book owned by userID and a chapter under it,
// returning both so tests can also seed book-scoped assets.
func (e *storyTestEnv) newChapterWithBook(t *testing.T, userID int64) (models.Chapter, models.Book) {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := e.chapters.CreateChapter(userID, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	return c, b
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

func TestStoryboardChatReturnsReply(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"我们一起想想开头吧！","panels":[]}`)
	ch := env.newChapter(t, 1)

	reply, panels, err := env.svc.StoryboardChat(1, ch.ID, []ai.Msg{{Role: "user", Content: "帮我"}}, 0)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if reply != "我们一起想想开头吧！" {
		t.Fatalf("reply 错: %q", reply)
	}
	if len(panels) != 0 {
		t.Fatalf("空 panels 应返回空, got %d", len(panels))
	}
}

func TestStoryboardChatCrossUserNotFound(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"hi","panels":[]}`)
	ch := env.newChapter(t, 1)

	_, _, err := env.svc.StoryboardChat(2, ch.ID, []ai.Msg{{Role: "user", Content: "帮我"}}, 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户应 ErrNotFound: %v", err)
	}
}

func TestStoryboardChatPersistsPanels(t *testing.T) {
	env := newStoryTestEnv(t, "placeholder")
	ch, b := env.newChapterWithBook(t, 1)

	char, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "小狐狸", Type: "person"})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	scene, err := env.assets.CreateScene(1, b.ID, models.Scene{Name: "森林"})
	if err != nil {
		t.Fatalf("create scene: %v", err)
	}
	env.setContent(`分镜：{"reply":"好的","panels":[` +
		`{"location":"森林边","sceneId":` + strconv.FormatInt(scene.ID, 10) +
		`,"characters":[{"id":` + strconv.FormatInt(char.ID, 10) + `,"expression":"好奇"}],` +
		`"event":"出发探险","caption":"出发","imagePrompt":"fox"},` +
		`{"location":"家里","sceneId":0,"characters":[],"event":"回到家","caption":"回家","imagePrompt":"home"}]}`)

	reply, panels, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if reply != "好的" {
		t.Fatalf("reply 错: %q", reply)
	}
	if len(panels) != 2 {
		t.Fatalf("应有2个分镜, got %d", len(panels))
	}
	if panels[0].Caption != "出发" || panels[0].Status != "pending" {
		t.Fatalf("首个分镜错: %+v", panels[0])
	}
	if panels[0].Location != "森林边" || panels[0].Event != "出发探险" {
		t.Fatalf("结构化字段错: %+v", panels[0])
	}
	if len(panels[0].CharacterIDs) != 1 || panels[0].CharacterIDs[0] != char.ID || panels[0].SceneID != scene.ID {
		t.Fatalf("引用解析错: %+v", panels[0])
	}
	if panels[0].CharExpressions[char.ID] != "好奇" {
		t.Fatalf("表情持久化错: %+v", panels[0].CharExpressions)
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

func TestStoryboardChatBadJSONErrors(t *testing.T) {
	env := newStoryTestEnv(t, "抱歉无法生成")
	ch := env.newChapter(t, 1)

	_, _, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err == nil {
		t.Fatal("无 JSON 应报错")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("AI 解析错误不应是 ErrNotFound: %v", err)
	}
}

// TestStoryboardChatRegenerateSucceeds guards against the state-machine bug:
// a second turn on a chapter already in "storyboarding" must succeed (no
// self-transition exists) rather than fail after the panels were replaced.
func TestStoryboardChatRegenerateSucceeds(t *testing.T) {
	body := `{"reply":"ok","panels":[{"location":"L","sceneId":0,"characters":[],"event":"E","caption":"v","imagePrompt":"x"}]}`
	env := newStoryTestEnv(t, body)
	ch := env.newChapter(t, 1)

	_, first, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(first) != 1 {
		t.Fatalf("首次应成功: %v (%d)", err, len(first))
	}

	// Second turn on the same chapter (already storyboarding) must NOT error.
	_, second, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil {
		t.Fatalf("重生成不应报错: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("重生成应替换出1个分镜, got %d", len(second))
	}

	got, err := env.chapters.GetChapter(1, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Status != "storyboarding" {
		t.Fatalf("章节状态应仍为 storyboarding, got %q", got.Status)
	}
}

// TestStoryboardChatFiltersForeignIDs verifies hallucinated character ids and
// foreign scene ids are dropped before persistence.
func TestStoryboardChatFiltersForeignIDs(t *testing.T) {
	// The model returns character id 999 (nonexistent) and scene id 888 (foreign);
	// only the valid character id (created below) and sceneId==0 must survive.
	body := `{"reply":"ok","panels":[{"location":"L","sceneId":888,` +
		`"characters":[{"id":VALID,"expression":"笑"},{"id":999,"expression":"哭"}],` +
		`"event":"E","caption":"c","imagePrompt":"x"}]}`
	env := newStoryTestEnv(t, "placeholder")
	ch, b := env.newChapterWithBook(t, 1)

	char, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "小狐狸", Type: "person"})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	env.setContent(strings.Replace(body, "VALID", strconv.FormatInt(char.ID, 10), 1))

	_, panels, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(panels) != 1 {
		t.Fatalf("应有1个分镜, got %d", len(panels))
	}
	if len(panels[0].CharacterIDs) != 1 || panels[0].CharacterIDs[0] != char.ID {
		t.Fatalf("应只保留合法角色 id: %+v", panels[0].CharacterIDs)
	}
	if panels[0].SceneID != 0 {
		t.Fatalf("外部 sceneId 应被清零, got %d", panels[0].SceneID)
	}
}
