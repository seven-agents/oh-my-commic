package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// fakeChatServer returns a DashScope-compatible server whose single choice's
// content is the given raw string.
func fakeChatServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`))
	}))
}

// TestStoryboardChatToleratesStringIDs guards the flexID fix: LLMs frequently
// emit ids as JSON STRINGS ("1") instead of numbers (1). A rigid int64 field
// made the whole storyboard parse fail (→ 502). Here ids arrive quoted and must
// still resolve to the real assets.
func TestStoryboardChatToleratesStringIDs(t *testing.T) {
	content := `{"reply":"好的","panels":[{"location":"森林","sceneId":"5",` +
		`"characters":[{"id":"1","expression":"开心"}],"event":"玩耍",` +
		`"caption":"一起玩","imagePrompt":"forest play"}]}`
	ts := fakeChatServer(content)
	defer ts.Close()
	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}

	res, err := StoryboardChat(context.Background(), c, []Msg{{Role: "user", Content: "讲故事"}}, twoCharOneScene(), 1)
	if err != nil {
		t.Fatalf("string ids should parse, got error: %v", err)
	}
	if len(res.Panels) != 1 {
		t.Fatalf("want 1 panel, got %d", len(res.Panels))
	}
	p := res.Panels[0]
	if p.SceneID != 5 {
		t.Fatalf("sceneId string \"5\" should parse to 5, got %d", p.SceneID)
	}
	if len(p.Characters) != 1 || p.Characters[0].ID != 1 {
		t.Fatalf("character id string \"1\" should parse to 1 and survive, got %+v", p.Characters)
	}
}

// TestStoryboardChatToleratesNameAsID guards the second flexID degradation: when
// the book has no matching assets, the model sometimes quotes a NAME as the id
// ("小狐狸"). That must degrade to "no id" (dropped by sanitize), NOT fail the
// whole parse with a 502.
func TestStoryboardChatToleratesNameAsID(t *testing.T) {
	content := `{"reply":"好的","panels":[{"location":"森林","sceneId":0,` +
		`"characters":[{"id":"小狐狸","expression":"开心"}],"event":"玩耍",` +
		`"caption":"一起玩","imagePrompt":"forest play"}]}`
	ts := fakeChatServer(content)
	defer ts.Close()
	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}

	// Empty asset context (no characters) — the name-id must be dropped.
	res, err := StoryboardChat(context.Background(), c, []Msg{{Role: "user", Content: "讲故事"}}, AssetContext{}, 1)
	if err != nil {
		t.Fatalf("a name-as-id must not fail the parse, got error: %v", err)
	}
	if len(res.Panels) != 1 {
		t.Fatalf("want 1 panel, got %d", len(res.Panels))
	}
	if len(res.Panels[0].Characters) != 0 {
		t.Fatalf("unresolvable name-id should be dropped, got %+v", res.Panels[0].Characters)
	}
}

// twoCharOneScene is an asset context with characters 1 & 2 and scene 5. It is
// the reference set every sanitize assertion validates against.
func twoCharOneScene() AssetContext {
	return AssetContext{
		Characters: []models.Character{{ID: 1, Name: "棉花糖"}, {ID: 2, Name: "小狐狸"}},
		Scenes:     []models.Scene{{ID: 5, Name: "森林"}},
	}
}

// TestStoryboardChatParsesAndSanitizes covers the core path: prose around a
// {reply, panels:[...]} object is parsed, structured fields survive, and an
// invalid character id plus a 4th reference are filtered so the panel keeps ≤3
// valid references.
func TestStoryboardChatParsesAndSanitizes(t *testing.T) {
	// characters: 1 (valid), 2 (valid), 3 (invalid → dropped), 4 (invalid →
	// dropped). scene 5 is valid, but with 2 valid characters + scene = 3 refs it
	// stays. Total valid refs must be ≤3.
	body := "好的，这是分镜：\n" +
		`{"reply":"我们来讲棉花糖的故事吧！","summary":"棉花糖在黄昏的森林里听见门声，好奇地回头张望，温暖的故事就此开始。","panels":[` +
		`{"location":"黄昏的森林餐桌旁","sceneId":5,` +
		`"characters":[{"id":1,"expression":"歪着头、好奇"},{"id":2,"expression":"微笑"},{"id":3,"expression":"生气"},{"id":4,"expression":"哭"}],` +
		`"event":"棉花糖听到门开的声音回头张望","caption":"棉花糖好奇地回头。","imagePrompt":"fox at dusk"}` +
		"]}\n希望你喜欢~"

	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, twoCharOneScene(), 0)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if res.Reply != "我们来讲棉花糖的故事吧！" {
		t.Fatalf("reply 解析错: %q", res.Reply)
	}
	if res.Summary != "棉花糖在黄昏的森林里听见门声，好奇地回头张望，温暖的故事就此开始。" {
		t.Fatalf("summary 解析错: %q", res.Summary)
	}
	if len(res.Panels) != 1 {
		t.Fatalf("应有 1 格分镜, got %d", len(res.Panels))
	}
	p := res.Panels[0]
	if p.Location != "黄昏的森林餐桌旁" {
		t.Fatalf("location 解析错: %q", p.Location)
	}
	if p.Event != "棉花糖听到门开的声音回头张望" {
		t.Fatalf("event 解析错: %q", p.Event)
	}
	// Invalid ids 3 & 4 dropped → only characters 1 & 2 survive.
	if len(p.Characters) != 2 {
		t.Fatalf("非法角色应被过滤，应剩 2 个, got %d: %+v", len(p.Characters), p.Characters)
	}
	if p.Characters[0].ID != 1 || p.Characters[0].Expression != "歪着头、好奇" {
		t.Fatalf("首个角色/表情解析错: %+v", p.Characters[0])
	}
	// 2 characters + scene = 3 refs, at the limit and all valid.
	refs := len(p.Characters)
	if p.SceneID != 0 {
		refs++
	}
	if refs > maxPanelRefs {
		t.Fatalf("引用数应 ≤%d, got %d", maxPanelRefs, refs)
	}
	if p.SceneID != 5 {
		t.Fatalf("合法 sceneId 应保留, got %d", p.SceneID)
	}
}

// TestStoryboardChatDropsSceneWhenTenCharacters verifies the character-first
// ≤10 rule: ten valid characters plus a scene must drop the scene (characters
// already fill the 10-reference budget).
func TestStoryboardChatDropsSceneWhenTenCharacters(t *testing.T) {
	chars := make([]models.Character, 0, 10)
	crefs := make([]string, 0, 10)
	for i := 1; i <= 10; i++ {
		chars = append(chars, models.Character{ID: int64(i)})
		crefs = append(crefs, `{"id":`+strconv.Itoa(i)+`,"expression":"e"}`)
	}
	assets := AssetContext{
		Characters: chars,
		Scenes:     []models.Scene{{ID: 99}},
	}
	body := `{"reply":"ok","panels":[{"location":"L","sceneId":99,` +
		`"characters":[` + strings.Join(crefs, ",") + `],` +
		`"event":"E","caption":"C","imagePrompt":"P"}]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, assets, 0)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	p := res.Panels[0]
	if len(p.Characters) != 10 {
		t.Fatalf("应保留 10 个角色, got %d", len(p.Characters))
	}
	if p.SceneID != 0 {
		t.Fatalf("已有 10 个角色时场景应被丢弃, got sceneId=%d", p.SceneID)
	}
}

// TestStoryboardChatCapsElevenCharactersToTen verifies characters beyond the
// 10-reference budget are dropped (11 valid characters → capped to 10).
func TestStoryboardChatCapsElevenCharactersToTen(t *testing.T) {
	chars := make([]models.Character, 0, 11)
	crefs := make([]string, 0, 11)
	for i := 1; i <= 11; i++ {
		chars = append(chars, models.Character{ID: int64(i)})
		crefs = append(crefs, `{"id":`+strconv.Itoa(i)+`,"expression":"e"}`)
	}
	assets := AssetContext{Characters: chars}
	body := `{"reply":"ok","panels":[{"location":"L","sceneId":0,` +
		`"characters":[` + strings.Join(crefs, ",") + `],` +
		`"event":"E","caption":"C","imagePrompt":"P"}]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, assets, 0)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if got := len(res.Panels[0].Characters); got != 10 {
		t.Fatalf("11 个角色应被截断到 10, got %d", got)
	}
}

// TestStoryboardChatFencedJSON verifies a ```json fenced object is parsed.
func TestStoryboardChatFencedJSON(t *testing.T) {
	body := "```json\n" +
		`{"reply":"你好","panels":[{"location":"L","sceneId":0,"characters":[],"event":"E","caption":"C","imagePrompt":"P"}]}` +
		"\n```"
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0)
	if err != nil {
		t.Fatalf("代码块 JSON 应解析: %v", err)
	}
	if res.Reply != "你好" || len(res.Panels) != 1 {
		t.Fatalf("解析错: %+v", res)
	}
	if res.Panels[0].Location != "L" || res.Panels[0].SceneID != 0 {
		t.Fatalf("字段解析错: %+v", res.Panels[0])
	}
}

// TestStoryboardChatSummaryOptional verifies a response that OMITS "summary"
// still parses (summary degrades to the empty string) and does not fail.
func TestStoryboardChatSummaryOptional(t *testing.T) {
	body := `{"reply":"你好","panels":[{"location":"L","sceneId":0,"characters":[],"event":"E","caption":"C","imagePrompt":"P"}]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0)
	if err != nil {
		t.Fatalf("缺失 summary 不应报错: %v", err)
	}
	if res.Summary != "" {
		t.Fatalf("缺失 summary 应为空串, got %q", res.Summary)
	}
	if res.Reply != "你好" || len(res.Panels) != 1 {
		t.Fatalf("其余字段应正常解析: %+v", res)
	}
}

// TestStoryboardChatParsesSummary verifies a present "summary" is parsed.
func TestStoryboardChatParsesSummary(t *testing.T) {
	const summary = "一段温暖的中文故事概述。"
	body := `{"reply":"你好","summary":"` + summary + `","panels":[]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if res.Summary != summary {
		t.Fatalf("summary 应被解析, got %q", res.Summary)
	}
}

// TestStoryboardChatNoObjectErrors verifies a reply without a JSON object errors.
func TestStoryboardChatNoObjectErrors(t *testing.T) {
	ts := fakeChatServer("抱歉，我暂时无法生成分镜。")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0); err == nil {
		t.Fatal("无 JSON 对象时应报错")
	}
}

// TestStoryboardChatMalformedErrors verifies a malformed object errors.
func TestStoryboardChatMalformedErrors(t *testing.T) {
	ts := fakeChatServer(`{"reply": not valid}`)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}
