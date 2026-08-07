// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"os"
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
	TextBaseURL  string
	ImageBaseURL string
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
		TextBaseURL:  get("QWEN_TEXT_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		ImageBaseURL: get("QWEN_IMAGE_BASE_URL", "https://dashscope.aliyuncs.com/api/v1"),
	}
	if c.DashScopeKey == "" {
		return c, errors.New("DASHSCOPE_API_KEY 未设置")
	}
	return c, nil
}
