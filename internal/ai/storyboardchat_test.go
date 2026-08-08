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

// twoCharOneScene is an asset context with characters 1 & 2 and scene 5. It is
// the reference set every stage-2 sanitize assertion validates against.
func twoCharOneScene() AssetContext {
	return AssetContext{
		Characters: []models.Character{{ID: 1, Name: "棉花糖"}, {ID: 2, Name: "小狐狸"}},
		Scenes:     []models.Scene{{ID: 5, Name: "森林"}},
	}
}

// --- Stage 1: StoryboardChat (content-only) ---------------------------------

// TestStoryboardChatParsesContentPanels covers the stage-1 happy path: prose
// around a {reply, summary, panels:[{content}]} object is parsed and each frame's
// content-only field survives.
func TestStoryboardChatParsesContentPanels(t *testing.T) {
	body := "好的，这是分镜：\n" +
		`{"reply":"我们来讲棉花糖的故事吧！","summary":"棉花糖在黄昏的森林里听见门声，好奇地回头张望。","panels":[` +
		`{"content":"黄昏的森林餐桌旁，棉花糖听到门开的声音好奇地回头。"},` +
		`{"content":"小狐狸推门走进来，两人相视一笑。"}` +
		"]}\n希望你喜欢~"

	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, twoCharOneScene(), 0, nil)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if res.Reply != "我们来讲棉花糖的故事吧！" {
		t.Fatalf("reply 解析错: %q", res.Reply)
	}
	if res.Summary != "棉花糖在黄昏的森林里听见门声，好奇地回头张望。" {
		t.Fatalf("summary 解析错: %q", res.Summary)
	}
	if len(res.Panels) != 2 {
		t.Fatalf("应有 2 格分镜, got %d", len(res.Panels))
	}
	if res.Panels[0].Content != "黄昏的森林餐桌旁，棉花糖听到门开的声音好奇地回头。" {
		t.Fatalf("首格 content 解析错: %q", res.Panels[0].Content)
	}
	if res.Panels[1].Content != "小狐狸推门走进来，两人相视一笑。" {
		t.Fatalf("第二格 content 解析错: %q", res.Panels[1].Content)
	}
}

// TestStoryboardChatFencedJSON verifies a ```json fenced object is parsed.
func TestStoryboardChatFencedJSON(t *testing.T) {
	body := "```json\n" +
		`{"reply":"你好","panels":[{"content":"一个安静的清晨"}]}` +
		"\n```"
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0, nil)
	if err != nil {
		t.Fatalf("代码块 JSON 应解析: %v", err)
	}
	if res.Reply != "你好" || len(res.Panels) != 1 {
		t.Fatalf("解析错: %+v", res)
	}
	if res.Panels[0].Content != "一个安静的清晨" {
		t.Fatalf("content 解析错: %+v", res.Panels[0])
	}
}

// TestStoryboardChatSummaryOptional verifies a response that OMITS "summary"
// still parses (summary degrades to the empty string) and does not fail.
func TestStoryboardChatSummaryOptional(t *testing.T) {
	body := `{"reply":"你好","panels":[{"content":"C"}]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0, nil)
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

// TestStoryboardChatPanelsOptional verifies a response that OMITS "panels" still
// parses (panels degrades to nil) and does not fail.
func TestStoryboardChatPanelsOptional(t *testing.T) {
	body := `{"reply":"你好","summary":"一段概述"}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0, nil)
	if err != nil {
		t.Fatalf("缺失 panels 不应报错: %v", err)
	}
	if len(res.Panels) != 0 {
		t.Fatalf("缺失 panels 应为空, got %d", len(res.Panels))
	}
}

// TestStoryboardChatNoObjectErrors verifies a reply without a JSON object errors.
func TestStoryboardChatNoObjectErrors(t *testing.T) {
	ts := fakeChatServer("抱歉，我暂时无法生成分镜。")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0, nil); err == nil {
		t.Fatal("无 JSON 对象时应报错")
	}
}

// TestStoryboardChatMalformedErrors verifies a malformed object errors.
func TestStoryboardChatMalformedErrors(t *testing.T) {
	ts := fakeChatServer(`{"reply": not valid}`)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}, 0, nil); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

// --- Stage 2: ProcessPanel (structured decomposition + sanitize) ------------

// TestProcessPanelParsesAndSanitizes covers the core stage-2 path: prose around a
// single {location, sceneId, characters, event, caption, imagePrompt} object is
// parsed, structured fields survive, and an invalid character id is filtered so
// the panel keeps only valid references.
func TestProcessPanelParsesAndSanitizes(t *testing.T) {
	body := "解析结果：\n" +
		`{"location":"黄昏的森林餐桌旁","sceneId":5,` +
		`"characters":[{"id":1,"expression":"歪着头、好奇"},{"id":2,"expression":"微笑"},{"id":3,"expression":"生气"}],` +
		`"event":"棉花糖听到门开的声音回头张望","caption":"棉花糖好奇地回头。","imagePrompt":"fox at dusk"}` +
		"\n完成~"

	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	draft, err := ProcessPanel(context.Background(), c, "棉花糖回头张望", twoCharOneScene())
	if err != nil {
		t.Fatalf("process panel: %v", err)
	}
	if draft.Location != "黄昏的森林餐桌旁" {
		t.Fatalf("location 解析错: %q", draft.Location)
	}
	if draft.Event != "棉花糖听到门开的声音回头张望" {
		t.Fatalf("event 解析错: %q", draft.Event)
	}
	// Invalid id 3 dropped → only characters 1 & 2 survive.
	if len(draft.Characters) != 2 {
		t.Fatalf("非法角色应被过滤，应剩 2 个, got %d: %+v", len(draft.Characters), draft.Characters)
	}
	if draft.Characters[0].ID != 1 || draft.Characters[0].Expression != "歪着头、好奇" {
		t.Fatalf("首个角色/表情解析错: %+v", draft.Characters[0])
	}
	if draft.SceneID != 5 {
		t.Fatalf("合法 sceneId 应保留, got %d", draft.SceneID)
	}
}

// TestProcessPanelToleratesStringIDs guards the flexID fix: the model may emit
// ids as JSON STRINGS ("1") instead of numbers (1). They must still resolve.
func TestProcessPanelToleratesStringIDs(t *testing.T) {
	body := `{"location":"森林","sceneId":"5",` +
		`"characters":[{"id":"1","expression":"开心"}],"event":"玩耍",` +
		`"caption":"一起玩","imagePrompt":"forest play"}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	draft, err := ProcessPanel(context.Background(), c, "玩耍", twoCharOneScene())
	if err != nil {
		t.Fatalf("string ids should parse, got error: %v", err)
	}
	if draft.SceneID != 5 {
		t.Fatalf("sceneId string \"5\" should parse to 5, got %d", draft.SceneID)
	}
	if len(draft.Characters) != 1 || draft.Characters[0].ID != 1 {
		t.Fatalf("character id string \"1\" should parse and survive, got %+v", draft.Characters)
	}
}

// TestProcessPanelToleratesNameAsID guards the second flexID degradation: an
// unresolvable NAME quoted as an id ("小狐狸") must degrade to "no id" (dropped),
// not fail the whole parse.
func TestProcessPanelToleratesNameAsID(t *testing.T) {
	body := `{"location":"森林","sceneId":0,` +
		`"characters":[{"id":"小狐狸","expression":"开心"}],"event":"玩耍",` +
		`"caption":"一起玩","imagePrompt":"forest play"}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	// Empty asset context — the name-id must be dropped.
	draft, err := ProcessPanel(context.Background(), c, "玩耍", AssetContext{})
	if err != nil {
		t.Fatalf("a name-as-id must not fail the parse, got error: %v", err)
	}
	if len(draft.Characters) != 0 {
		t.Fatalf("unresolvable name-id should be dropped, got %+v", draft.Characters)
	}
}

// TestProcessPanelDropsSceneWhenTenCharacters verifies the character-first ≤10
// rule: ten valid characters plus a scene must drop the scene.
func TestProcessPanelDropsSceneWhenTenCharacters(t *testing.T) {
	chars := make([]models.Character, 0, 10)
	crefs := make([]string, 0, 10)
	for i := 1; i <= 10; i++ {
		chars = append(chars, models.Character{ID: int64(i)})
		crefs = append(crefs, `{"id":`+strconv.Itoa(i)+`,"expression":"e"}`)
	}
	assets := AssetContext{Characters: chars, Scenes: []models.Scene{{ID: 99}}}
	body := `{"location":"L","sceneId":99,` +
		`"characters":[` + strings.Join(crefs, ",") + `],` +
		`"event":"E","caption":"C","imagePrompt":"P"}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	draft, err := ProcessPanel(context.Background(), c, "热闹的聚会", assets)
	if err != nil {
		t.Fatalf("process panel: %v", err)
	}
	if len(draft.Characters) != 10 {
		t.Fatalf("应保留 10 个角色, got %d", len(draft.Characters))
	}
	if draft.SceneID != 0 {
		t.Fatalf("已有 10 个角色时场景应被丢弃, got sceneId=%d", draft.SceneID)
	}
}

// TestProcessPanelNoObjectErrors verifies a reply without a JSON object errors.
func TestProcessPanelNoObjectErrors(t *testing.T) {
	ts := fakeChatServer("抱歉，无法解析。")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := ProcessPanel(context.Background(), c, "x", AssetContext{}); err == nil {
		t.Fatal("无 JSON 对象时应报错")
	}
}
