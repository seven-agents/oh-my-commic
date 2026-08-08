// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the oh-my-commic server.
type Config struct {
	Port    string
	DBPath  string
	DataDir string

	// DashScope text (Qwen chat / storyboard) config.
	DashScopeKey string
	TextModel    string
	TextBaseURL  string

	// Volcano Ark Seedream 4.0 image-generation config.
	ArkKey          string
	SeedreamModel   string
	SeedreamBaseURL string

	// RenderMaxRefs caps how many reference images are forwarded to the image
	// model when rendering a panel.
	RenderMaxRefs int
}

// defaultRenderMaxRefs is the fallback cap on reference images per render.
// Seedream 4.0 accepts up to 10 reference images per request, so the default
// matches that ceiling.
const defaultRenderMaxRefs = 10

// getInt returns the integer value of env var k, or def when unset/empty or not
// a positive integer (a safe fallback so a bad env value never breaks startup).
func getInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// get returns the environment value for key k, or def when it is unset/empty.
func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Load reads configuration from the environment, applying defaults.
// It returns an error when a required API key (DASHSCOPE_API_KEY for text,
// ARK_API_KEY for Seedream images) is not set.
func Load() (Config, error) {
	c := Config{
		Port:         get("PORT", "8080"),
		DBPath:       get("DB_PATH", "oh-my-commic.db"),
		DataDir:      get("DATA_DIR", "data"),
		DashScopeKey: os.Getenv("DASHSCOPE_API_KEY"),
		TextModel:    get("QWEN_TEXT_MODEL", "qwen-plus"),
		TextBaseURL:  get("QWEN_TEXT_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),

		ArkKey:          os.Getenv("ARK_API_KEY"),
		SeedreamModel:   get("SEEDREAM_MODEL", "doubao-seedream-4-0-250828"),
		SeedreamBaseURL: get("SEEDREAM_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),

		RenderMaxRefs: renderMaxRefs(),
	}
	if c.DashScopeKey == "" {
		return c, errors.New("DASHSCOPE_API_KEY 未设置")
	}
	if c.ArkKey == "" {
		return c, errors.New("ARK_API_KEY 未设置")
	}
	return c, nil
}

// renderMaxRefs resolves the reference-image cap, preferring the new
// RENDER_MAX_REFS env var and falling back to the legacy QWEN_RENDER_MAX_REFS
// for backward compatibility, then the default.
func renderMaxRefs() int {
	if v := os.Getenv("RENDER_MAX_REFS"); v != "" {
		return getInt("RENDER_MAX_REFS", defaultRenderMaxRefs)
	}
	return getInt("QWEN_RENDER_MAX_REFS", defaultRenderMaxRefs)
}
