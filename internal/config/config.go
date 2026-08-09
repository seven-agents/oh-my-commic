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

	// SignupCredits is the starting image-credit balance granted to a newly
	// registered user (env SIGNUP_CREDITS, default 100).
	SignupCredits int
	// ImageCreditCost is how many credits a single image call (panel render or
	// asset comicify) charges up front (env IMAGE_CREDIT_COST, default 1).
	ImageCreditCost int

	// Admin seed credentials (env ADMIN_USERNAME / ADMIN_PASSWORD / ADMIN_EMAIL):
	// when all present, an initial admin user is seeded at startup. All optional
	// and never fatal when unset.
	AdminUsername string
	AdminPassword string
	AdminEmail    string
	// InviteCode gates self-registration (env INVITE_CODE). Optional; empty means
	// no invite gate configured. Never fatal when unset.
	InviteCode string
	// InviteMaxUses caps how many successful registrations a single invite code
	// permits before it is exhausted (env INVITE_MAX_USES, default 10). Rotating
	// the code resets the counter. 0 means unlimited (the pre-limit behavior).
	InviteMaxUses int
}

// defaultSignupCredits is the starting balance granted at registration.
const defaultSignupCredits = 100

// defaultImageCreditCost is the credit cost of a single image generation.
const defaultImageCreditCost = 1

// defaultRenderMaxRefs is the fallback cap on reference images per render.
// Seedream 4.0 accepts up to 10 reference images per request, so the default
// matches that ceiling.
const defaultRenderMaxRefs = 10

// defaultInviteMaxUses caps registrations per invite code by default.
const defaultInviteMaxUses = 10

// getIntNonNeg is like getInt but accepts 0 as a valid, meaningful value (used
// where 0 carries semantics, e.g. INVITE_MAX_USES=0 meaning "unlimited"). A
// negative or non-numeric value falls back to def.
func getIntNonNeg(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

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

		SignupCredits:   getInt("SIGNUP_CREDITS", defaultSignupCredits),
		ImageCreditCost: getInt("IMAGE_CREDIT_COST", defaultImageCreditCost),

		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		InviteCode:    os.Getenv("INVITE_CODE"),
		InviteMaxUses: getIntNonNeg("INVITE_MAX_USES", defaultInviteMaxUses),
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
