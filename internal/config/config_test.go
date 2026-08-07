package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.TextModel != "qwen-plus" {
		t.Fatalf("default text model wrong: %s", c.TextModel)
	}
	if c.DashScopeKey != "sk-test" {
		t.Fatalf("key not loaded")
	}
}

func TestLoadMissingKey(t *testing.T) {
	os.Unsetenv("DASHSCOPE_API_KEY")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DASHSCOPE_API_KEY is missing")
	}
}

func TestLoadOverridesAndDefaults(t *testing.T) {
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")
	os.Unsetenv("QWEN_IMAGE_MODEL")
	os.Unsetenv("QWEN_EDIT_MODEL")
	os.Unsetenv("QWEN_RENDER_MODEL")
	os.Unsetenv("QWEN_RENDER_MAX_REFS")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9090" {
		t.Fatalf("PORT override not applied: %s", c.Port)
	}
	if c.DBPath != "oh-my-commic.db" {
		t.Fatalf("default DBPath wrong: %s", c.DBPath)
	}
	if c.DataDir != "data" {
		t.Fatalf("default DataDir wrong: %s", c.DataDir)
	}
	if c.ImageModel != "wan2.2-t2i-plus" {
		t.Fatalf("default ImageModel wrong: %s", c.ImageModel)
	}
	if c.EditModel != "qwen-image-edit" {
		t.Fatalf("default EditModel wrong: %s", c.EditModel)
	}
	if c.RenderModel != "qwen-image-edit-plus" {
		t.Fatalf("default RenderModel wrong: %s", c.RenderModel)
	}
	if c.RenderMaxRefs != 3 {
		t.Fatalf("default RenderMaxRefs wrong: %d", c.RenderMaxRefs)
	}
	if c.TextBaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("default TextBaseURL wrong: %s", c.TextBaseURL)
	}
	if c.ImageBaseURL != "https://dashscope.aliyuncs.com/api/v1" {
		t.Fatalf("default ImageBaseURL wrong: %s", c.ImageBaseURL)
	}
}

func TestLoadRenderMaxRefs(t *testing.T) {
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")

	// Valid override is parsed.
	os.Setenv("QWEN_RENDER_MAX_REFS", "6")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.RenderMaxRefs != 6 {
		t.Fatalf("override RenderMaxRefs wrong: %d", c.RenderMaxRefs)
	}

	// Invalid / non-positive values fall back to the default.
	for _, bad := range []string{"abc", "0", "-2", ""} {
		os.Setenv("QWEN_RENDER_MAX_REFS", bad)
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.RenderMaxRefs != 3 {
			t.Fatalf("RenderMaxRefs for %q should fall back to 3, got %d", bad, c.RenderMaxRefs)
		}
	}
	os.Unsetenv("QWEN_RENDER_MAX_REFS")
}
