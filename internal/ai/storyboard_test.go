package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// fakeChatServer returns a DashScope-compatible server whose single choice's
// content is the given raw string.
func fakeChatServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`))
	}))
}

func TestGenStoryboardParsesJSON(t *testing.T) {
	body := "好的，这是分镜：\n[{\"caption\":\"小狐狸出发\",\"characterIds\":[1],\"sceneId\":2,\"imagePrompt\":\"fox in forest\"}]\n希望满意"
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	drafts, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 1)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("应解析出1个: %v", err)
	}
	if drafts[0].CharacterIDs[0] != 1 || drafts[0].SceneID != 2 {
		t.Fatalf("字段解析错: %+v", drafts[0])
	}
}

func TestGenStoryboardParsesFencedJSON(t *testing.T) {
	body := "```json\n[{\"caption\":\"日出\",\"characterIds\":[],\"sceneId\":0,\"imagePrompt\":\"sunrise\"}]\n```"
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	drafts, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 1)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("代码块 JSON 应解析: %v", err)
	}
	if drafts[0].Caption != "日出" || drafts[0].SceneID != 0 {
		t.Fatalf("字段解析错: %+v", drafts[0])
	}
}

func TestGenStoryboardParsesMultipleObjects(t *testing.T) {
	body := "分镜如下[{\"caption\":\"a\",\"characterIds\":[1,2],\"sceneId\":3,\"imagePrompt\":\"x\"},{\"caption\":\"b\",\"characterIds\":[],\"sceneId\":0,\"imagePrompt\":\"y\"}]end"
	ts := fakeChatServer(body)
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	drafts, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 2)
	if err != nil || len(drafts) != 2 {
		t.Fatalf("应解析出2个: %v (%+v)", err, drafts)
	}
	if len(drafts[0].CharacterIDs) != 2 || drafts[0].CharacterIDs[1] != 2 {
		t.Fatalf("多角色解析错: %+v", drafts[0])
	}
}

func TestGenStoryboardNoArrayReturnsError(t *testing.T) {
	ts := fakeChatServer("抱歉，我暂时无法生成分镜。")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	_, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 1)
	if err == nil {
		t.Fatal("无 JSON 数组时应报错")
	}
}

func TestGenStoryboardMalformedReturnsError(t *testing.T) {
	ts := fakeChatServer("[{not valid json}]")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	_, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 1)
	if err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

func TestConversePrependsSystemPrompt(t *testing.T) {
	ts := fakeChatServer("我们一起想想吧！")
	defer ts.Close()

	c := &Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	reply, err := Converse(context.Background(), c, []Msg{{Role: "user", Content: "帮我想开头"}}, AssetContext{})
	if err != nil || reply != "我们一起想想吧！" {
		t.Fatalf("对话失败: %q %v", reply, err)
	}
}
