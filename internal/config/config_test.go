package config

import (
	"os"
	"testing"
)

// setKeys sets both required API keys so Load() succeeds; individual tests may
// still unset one to exercise the missing-key paths.
func setKeys(t *testing.T) {
	t.Helper()
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")
	os.Setenv("ARK_API_KEY", "ark-test")
}

func TestLoadDefaults(t *testing.T) {
	setKeys(t)
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
	if c.ArkKey != "ark-test" {
		t.Fatalf("ark key not loaded")
	}
}

func TestLoadMissingKey(t *testing.T) {
	os.Setenv("ARK_API_KEY", "ark-test")
	os.Unsetenv("DASHSCOPE_API_KEY")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DASHSCOPE_API_KEY is missing")
	}
}

func TestLoadMissingArkKey(t *testing.T) {
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")
	os.Unsetenv("ARK_API_KEY")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when ARK_API_KEY is missing")
	}
}

func TestLoadOverridesAndDefaults(t *testing.T) {
	setKeys(t)
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")
	os.Unsetenv("SEEDREAM_MODEL")
	os.Unsetenv("SEEDREAM_BASE_URL")
	os.Unsetenv("RENDER_MAX_REFS")
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
	if c.SeedreamModel != "doubao-seedream-4-0-250828" {
		t.Fatalf("default SeedreamModel wrong: %s", c.SeedreamModel)
	}
	if c.SeedreamBaseURL != "https://ark.cn-beijing.volces.com/api/v3" {
		t.Fatalf("default SeedreamBaseURL wrong: %s", c.SeedreamBaseURL)
	}
	if c.RenderMaxRefs != 10 {
		t.Fatalf("default RenderMaxRefs wrong: %d", c.RenderMaxRefs)
	}
	if c.TextBaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("default TextBaseURL wrong: %s", c.TextBaseURL)
	}
}

func TestLoadCreditDefaults(t *testing.T) {
	setKeys(t)
	os.Unsetenv("SIGNUP_CREDITS")
	os.Unsetenv("IMAGE_CREDIT_COST")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.SignupCredits != 100 {
		t.Fatalf("default SignupCredits should be 100, got %d", c.SignupCredits)
	}
	if c.ImageCreditCost != 1 {
		t.Fatalf("default ImageCreditCost should be 1, got %d", c.ImageCreditCost)
	}
}

func TestLoadCreditOverrides(t *testing.T) {
	setKeys(t)
	os.Setenv("SIGNUP_CREDITS", "250")
	os.Setenv("IMAGE_CREDIT_COST", "3")
	defer os.Unsetenv("SIGNUP_CREDITS")
	defer os.Unsetenv("IMAGE_CREDIT_COST")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.SignupCredits != 250 {
		t.Fatalf("SIGNUP_CREDITS override not applied: %d", c.SignupCredits)
	}
	if c.ImageCreditCost != 3 {
		t.Fatalf("IMAGE_CREDIT_COST override not applied: %d", c.ImageCreditCost)
	}

	// Invalid / non-positive values fall back to the defaults (never fatal).
	for _, bad := range []string{"abc", "0", "-5"} {
		os.Setenv("SIGNUP_CREDITS", bad)
		os.Setenv("IMAGE_CREDIT_COST", bad)
		c, err := Load()
		if err != nil {
			t.Fatalf("bad credit env %q must not fail Load: %v", bad, err)
		}
		if c.SignupCredits != 100 || c.ImageCreditCost != 1 {
			t.Fatalf("bad credit env %q should fall back to defaults, got signup=%d cost=%d", bad, c.SignupCredits, c.ImageCreditCost)
		}
	}
}

func TestLoadRenderMaxRefs(t *testing.T) {
	setKeys(t)
	defer os.Unsetenv("RENDER_MAX_REFS")
	defer os.Unsetenv("QWEN_RENDER_MAX_REFS")

	// New RENDER_MAX_REFS override is parsed.
	os.Unsetenv("QWEN_RENDER_MAX_REFS")
	os.Setenv("RENDER_MAX_REFS", "6")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.RenderMaxRefs != 6 {
		t.Fatalf("override RenderMaxRefs wrong: %d", c.RenderMaxRefs)
	}

	// RENDER_MAX_REFS takes precedence over the legacy name.
	os.Setenv("QWEN_RENDER_MAX_REFS", "3")
	os.Setenv("RENDER_MAX_REFS", "8")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.RenderMaxRefs != 8 {
		t.Fatalf("RENDER_MAX_REFS should win over QWEN_RENDER_MAX_REFS, got %d", c.RenderMaxRefs)
	}

	// Legacy QWEN_RENDER_MAX_REFS is honored when RENDER_MAX_REFS is unset.
	os.Unsetenv("RENDER_MAX_REFS")
	os.Setenv("QWEN_RENDER_MAX_REFS", "5")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.RenderMaxRefs != 5 {
		t.Fatalf("legacy QWEN_RENDER_MAX_REFS fallback wrong: %d", c.RenderMaxRefs)
	}

	// Invalid / non-positive values fall back to the default (10).
	os.Unsetenv("QWEN_RENDER_MAX_REFS")
	for _, bad := range []string{"abc", "0", "-2"} {
		os.Setenv("RENDER_MAX_REFS", bad)
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.RenderMaxRefs != 10 {
			t.Fatalf("RenderMaxRefs for %q should fall back to 10, got %d", bad, c.RenderMaxRefs)
		}
	}
	os.Unsetenv("RENDER_MAX_REFS")
}
