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
	panels   *panel.Service
	books    *book.Repo
	db       *sql.DB

	mu      sync.Mutex
	content string
}

// panelsSvc exposes the wired panel service for tests that seed structured
// fields / images before a merge turn.
func (e *storyTestEnv) panelsSvc() *panel.Service { return e.panels }

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
	env.panels = panelSvc
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

	reply, panels, err := env.svc.StoryboardChat(1, ch.ID, []models.ConversationMsg{{Role: "user", Content: "帮我"}}, 0)
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

	_, _, err := env.svc.StoryboardChat(2, ch.ID, []models.ConversationMsg{{Role: "user", Content: "帮我"}}, 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户应 ErrNotFound: %v", err)
	}
}

func TestStoryboardChatPersistsContentPanels(t *testing.T) {
	env := newStoryTestEnv(t, `分镜：{"reply":"好的","panels":[`+
		`{"content":"小狐狸在森林边出发探险"},`+
		`{"content":"小狐狸回到家里"}]}`)
	ch := env.newChapter(t, 1)

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
	// Stage-1 panels carry ONLY content; structured fields stay empty and status
	// is pending until stage-2 ProcessPanel runs.
	if panels[0].Content != "小狐狸在森林边出发探险" || panels[0].Status != "pending" {
		t.Fatalf("首个分镜错: %+v", panels[0])
	}
	if panels[0].Location != "" || panels[0].Event != "" || len(panels[0].CharacterIDs) != 0 {
		t.Fatalf("stage-1 分镜不应有结构化字段: %+v", panels[0])
	}
	if panels[1].Content != "小狐狸回到家里" {
		t.Fatalf("第二格 content 错: %+v", panels[1])
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

// TestStoryboardChatMergePreservesUnchanged verifies the content merge: a second
// turn whose first frame content is UNCHANGED preserves that frame's structured
// fields + image, while a CHANGED frame is reset to a pending content-only frame.
func TestStoryboardChatMergePreservesUnchanged(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"r","panels":[{"content":"第一格"},{"content":"第二格"}]}`)
	ch := env.newChapter(t, 1)

	// First turn: seed two content-only frames.
	_, first, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(first) != 2 {
		t.Fatalf("首轮应产出2格: %v (%d)", err, len(first))
	}

	// Simulate stage-2 processing + rendering on frame 0: give it structured
	// fields and a rendered image (via a direct UpdatePanel + a manual image set).
	firstID := first[0].ID
	upd := first[0]
	upd.Location = "森林"
	upd.Event = "出发"
	upd.Caption = "出发探险"
	if _, err := env.panelsSvc().UpdatePanel(1, firstID, upd); err != nil {
		t.Fatalf("update panel: %v", err)
	}
	if _, err := env.panelsSvc().SetPanelImage(1, firstID, "http://img/1.png"); err != nil {
		t.Fatalf("set image: %v", err)
	}

	// Second turn: frame 0 content unchanged, frame 1 content changed.
	env.setContent(`{"reply":"r2","panels":[{"content":"第一格"},{"content":"第二格改了"}]}`)
	_, second, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(second) != 2 {
		t.Fatalf("二轮应产出2格: %v (%d)", err, len(second))
	}

	// Frame 0 unchanged → structured fields + image + done status preserved.
	if second[0].Location != "森林" || second[0].Event != "出发" || second[0].Caption != "出发探险" {
		t.Fatalf("未变格应保留结构化字段: %+v", second[0])
	}
	if second[0].ImageURL != "http://img/1.png" || second[0].Status != "done" {
		t.Fatalf("未变格应保留图片与状态: %+v", second[0])
	}
	// Frame 1 changed → reset to content-only pending frame.
	if second[1].Content != "第二格改了" || second[1].Status != "pending" || second[1].Location != "" {
		t.Fatalf("变化格应被清空为 pending content-only: %+v", second[1])
	}
}

// TestStoryboardChatPersistsConversation verifies the conversation history (input
// messages + this turn's assistant reply) and panel_count are stored on the
// chapter.
func TestStoryboardChatPersistsConversation(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"好的呀","panels":[{"content":"一格"}]}`)
	ch := env.newChapter(t, 1)

	history := []models.ConversationMsg{{Role: "user", Content: "帮我讲个故事"}}
	if _, _, err := env.svc.StoryboardChat(1, ch.ID, history, 5); err != nil {
		t.Fatalf("chat: %v", err)
	}

	got, err := env.chapters.GetChapter(1, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.PanelCount != 5 {
		t.Fatalf("panelCount 应持久化为 5, got %d", got.PanelCount)
	}
	if len(got.Conversation) != 2 {
		t.Fatalf("对话应含 user + assistant 两条, got %d: %+v", len(got.Conversation), got.Conversation)
	}
	if got.Conversation[0].Role != "user" || got.Conversation[0].Content != "帮我讲个故事" {
		t.Fatalf("首条应为用户消息: %+v", got.Conversation[0])
	}
	if got.Conversation[1].Role != "assistant" || got.Conversation[1].Content != "好的呀" {
		t.Fatalf("次条应为 assistant reply: %+v", got.Conversation[1])
	}
}

// TestStoryboardChatPersistsSummary verifies the AI-produced summary is stored
// on the chapter (readable via GetChapter) as a side-output of the turn.
func TestStoryboardChatPersistsSummary(t *testing.T) {
	const summary = "小狐狸出发探险，最后温暖地回到家。"
	env := newStoryTestEnv(t, `{"reply":"好的","summary":"`+summary+`","panels":[`+
		`{"content":"小狐狸回到家里"}]}`)
	ch := env.newChapter(t, 1)

	if _, _, err := env.svc.StoryboardChat(1, ch.ID, nil, 0); err != nil {
		t.Fatalf("chat: %v", err)
	}

	got, err := env.chapters.GetChapter(1, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Summary != summary {
		t.Fatalf("章节 summary 应被持久化, got %q", got.Summary)
	}
}

// TestStoryboardChatEmptySummaryDoesNotWipe verifies a later turn that omits the
// summary does NOT overwrite a summary stored by an earlier turn.
func TestStoryboardChatEmptySummaryDoesNotWipe(t *testing.T) {
	const summary = "第一轮产出的温暖概述。"
	const panels = `"panels":[{"content":"一格内容"}]`
	env := newStoryTestEnv(t, `{"reply":"好的","summary":"`+summary+`",`+panels+`}`)
	ch := env.newChapter(t, 1)

	if _, _, err := env.svc.StoryboardChat(1, ch.ID, nil, 0); err != nil {
		t.Fatalf("first turn: %v", err)
	}

	// Second turn omits summary entirely — the stored summary must survive.
	env.setContent(`{"reply":"再来",` + panels + `}`)
	if _, _, err := env.svc.StoryboardChat(1, ch.ID, nil, 0); err != nil {
		t.Fatalf("second turn: %v", err)
	}

	got, err := env.chapters.GetChapter(1, ch.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Summary != summary {
		t.Fatalf("空 summary 不应覆盖已存概述, got %q", got.Summary)
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
	body := `{"reply":"ok","panels":[{"content":"一格内容"}]}`
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

// TestProcessPanelDecomposesAndFiltersForeignIDs verifies the stage-2 flow:
// ProcessPanel decomposes a panel's content into structured fields, dropping
// hallucinated character ids and foreign scene ids, while preserving Content.
func TestProcessPanelDecomposesAndFiltersForeignIDs(t *testing.T) {
	// The model returns character id 999 (nonexistent) and scene id 888 (foreign);
	// only the valid character id (created below) and sceneId==0 must survive.
	body := `{"location":"L","sceneId":888,` +
		`"characters":[{"id":VALID,"expression":"笑"},{"id":999,"expression":"哭"}],` +
		`"event":"E","caption":"c","imagePrompt":"x"}`
	env := newStoryTestEnv(t, "placeholder")
	ch, b := env.newChapterWithBook(t, 1)

	char, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "小狐狸", Type: "person"})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}

	// Seed a content-only panel via a stage-1 turn.
	env.setContent(`{"reply":"ok","panels":[{"content":"小狐狸开心地笑了"}]}`)
	_, seeded, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed panel: %v (%d)", err, len(seeded))
	}
	panelID := seeded[0].ID

	// Now stage-2 process it.
	env.setContent(strings.Replace(body, "VALID", strconv.FormatInt(char.ID, 10), 1))
	out, err := env.svc.ProcessPanel(1, panelID)
	if err != nil {
		t.Fatalf("process panel: %v", err)
	}
	if out.Content != "小狐狸开心地笑了" {
		t.Fatalf("content 应保留: %q", out.Content)
	}
	if out.Location != "L" || out.Event != "E" || out.Caption != "c" {
		t.Fatalf("结构化字段应写入: %+v", out)
	}
	if len(out.CharacterIDs) != 1 || out.CharacterIDs[0] != char.ID {
		t.Fatalf("应只保留合法角色 id: %+v", out.CharacterIDs)
	}
	if out.SceneID != 0 {
		t.Fatalf("外部 sceneId 应被清零, got %d", out.SceneID)
	}
	if out.CharExpressions[char.ID] != "笑" {
		t.Fatalf("表情应持久化: %+v", out.CharExpressions)
	}
}

// TestProcessPanelCrossUserNotFound verifies a user cannot process another
// user's panel.
func TestProcessPanelCrossUserNotFound(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"ok","panels":[{"content":"一格"}]}`)
	ch := env.newChapter(t, 1)
	_, seeded, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed panel: %v (%d)", err, len(seeded))
	}

	if _, err := env.svc.ProcessPanel(2, seeded[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户 process 应 ErrNotFound: %v", err)
	}
}

// TestProcessPanelAIErrorNotFound verifies an AI parse failure is a non-ErrNotFound
// error (mapped to 502 by the handler).
func TestProcessPanelAIErrorNotFound(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"ok","panels":[{"content":"一格"}]}`)
	ch := env.newChapter(t, 1)
	_, seeded, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed panel: %v (%d)", err, len(seeded))
	}

	env.setContent("抱歉无法解析") // no JSON object
	_, err = env.svc.ProcessPanel(1, seeded[0].ID)
	if err == nil {
		t.Fatal("AI 解析失败应报错")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("AI 解析错误不应是 ErrNotFound: %v", err)
	}
}
