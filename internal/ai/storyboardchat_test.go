package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		`{"reply":"我们来讲棉花糖的故事吧！","panels":[` +
		`{"location":"黄昏的森林餐桌旁","sceneId":5,` +
		`"characters":[{"id":1,"expression":"歪着头、好奇"},{"id":2,"expression":"微笑"},{"id":3,"expression":"生气"},{"id":4,"expression":"哭"}],` +
		`"event":"棉花糖听到门开的声音回头张望","caption":"棉花糖好奇地回头。","imagePrompt":"fox at dusk"}` +
		"]}\n希望你喜欢~"

	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, twoCharOneScene())
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	if res.Reply != "我们来讲棉花糖的故事吧！" {
		t.Fatalf("reply 解析错: %q", res.Reply)
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

// TestStoryboardChatDropsSceneWhenThreeCharacters verifies the character-first
// ≤3 rule: three valid characters plus a scene must drop the scene.
func TestStoryboardChatDropsSceneWhenThreeCharacters(t *testing.T) {
	assets := AssetContext{
		Characters: []models.Character{{ID: 1}, {ID: 2}, {ID: 3}},
		Scenes:     []models.Scene{{ID: 5}},
	}
	body := `{"reply":"ok","panels":[{"location":"L","sceneId":5,` +
		`"characters":[{"id":1,"expression":"a"},{"id":2,"expression":"b"},{"id":3,"expression":"c"}],` +
		`"event":"E","caption":"C","imagePrompt":"P"}]}`
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	res, err := StoryboardChat(context.Background(), c, nil, assets)
	if err != nil {
		t.Fatalf("storyboard chat: %v", err)
	}
	p := res.Panels[0]
	if len(p.Characters) != 3 {
		t.Fatalf("应保留 3 个角色, got %d", len(p.Characters))
	}
	if p.SceneID != 0 {
		t.Fatalf("已有 3 个角色时场景应被丢弃, got sceneId=%d", p.SceneID)
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
	res, err := StoryboardChat(context.Background(), c, nil, AssetContext{})
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

// TestStoryboardChatNoObjectErrors verifies a reply without a JSON object errors.
func TestStoryboardChatNoObjectErrors(t *testing.T) {
	ts := fakeChatServer("抱歉，我暂时无法生成分镜。")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}); err == nil {
		t.Fatal("无 JSON 对象时应报错")
	}
}

// TestStoryboardChatMalformedErrors verifies a malformed object errors.
func TestStoryboardChatMalformedErrors(t *testing.T) {
	ts := fakeChatServer(`{"reply": not valid}`)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	if _, err := StoryboardChat(context.Background(), c, nil, AssetContext{}); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}
