// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the oh-my-commic server.
type Config struct {
	Port         string
	DBPath       string
	DataDir      string
	DashScopeKey string
	TextModel    string
	ImageModel   string
	EditModel    string
	RenderModel  string
	TextBaseURL  string
	ImageBaseURL string

	// RenderMaxRefs caps how many reference images are forwarded to the
	// multi-image edit model when rendering a panel.
	RenderMaxRefs int
}

// defaultRenderMaxRefs is the fallback cap on reference images per render.
const defaultRenderMaxRefs = 4

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
// It returns an error when the required DASHSCOPE_API_KEY is not set.
func Load() (Config, error) {
	c := Config{
		Port:         get("PORT", "8080"),
		DBPath:       get("DB_PATH", "oh-my-commic.db"),
		DataDir:      get("DATA_DIR", "data"),
		DashScopeKey: os.Getenv("DASHSCOPE_API_KEY"),
		TextModel:    get("QWEN_TEXT_MODEL", "qwen-plus"),
		ImageModel:   get("QWEN_IMAGE_MODEL", "wan2.2-t2i-plus"),
		EditModel:    get("QWEN_EDIT_MODEL", "qwen-image-edit"),
		RenderModel:  get("QWEN_RENDER_MODEL", "qwen-image-edit-plus"),
		TextBaseURL:  get("QWEN_TEXT_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		ImageBaseURL: get("QWEN_IMAGE_BASE_URL", "https://dashscope.aliyuncs.com/api/v1"),

		RenderMaxRefs: getInt("QWEN_RENDER_MAX_REFS", defaultRenderMaxRefs),
	}
	if c.DashScopeKey == "" {
		return c, errors.New("DASHSCOPE_API_KEY 未设置")
	}
	return c, nil
}
