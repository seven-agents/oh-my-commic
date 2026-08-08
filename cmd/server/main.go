// Command server assembles all backend layers and starts the HTTP server.
package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/comicify"
	"github.com/seven-agents/oh-my-commic/internal/config"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/httpx"
	"github.com/seven-agents/oh-my-commic/internal/panel"
	"github.com/seven-agents/oh-my-commic/internal/render"
	"github.com/seven-agents/oh-my-commic/internal/storage"
	"github.com/seven-agents/oh-my-commic/internal/story"
)

func main() {
	// config.Load reads only process env, so seed it from .env first. Existing
	// environment variables always win, so this never overrides real deployment
	// configuration.
	loadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	media := storage.Local{Root: cfg.DataDir}
	sess := auth.NewSession(d)

	// userRepo doubles as the render/comicify credit ledger (Spend/Refund).
	userRepo := auth.NewUserRepo(d)
	authSvc := auth.NewService(userRepo, sess, cfg.SignupCredits)
	authHandler := auth.NewHandler(authSvc)

	bookRepo := book.NewRepo(d)
	bookSvc := book.NewService(bookRepo)
	bookHandler := book.NewHandler(bookSvc)

	aiClient := &ai.Client{
		// Text (Qwen chat / storyboard) via DashScope.
		Key:         cfg.DashScopeKey,
		TextBaseURL: cfg.TextBaseURL,
		TextModel:   cfg.TextModel,
		// Images (comicify + panel render) via Volcano Ark Seedream 4.0.
		ArkKey:          cfg.ArkKey,
		SeedreamModel:   cfg.SeedreamModel,
		SeedreamBaseURL: cfg.SeedreamBaseURL,
		// Seedream 2048x2048 generation can take ~20-40s (more with references),
		// so allow a generous 120s request timeout for the AI client.
		HTTP: &http.Client{Timeout: 120 * time.Second},
	}

	// Comic-ification downloads the produced image from a remote URL and can take
	// 15-30s per asset; give it a generous, independent timeout.
	comicSvc := comicify.NewService(aiClient, userRepo, cfg.ImageCreditCost, media, &http.Client{Timeout: 120 * time.Second})

	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	assetHandler := asset.NewHandler(assetSvc, media, comicSvc)

	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	chapterHandler := chapter.NewHandler(chapterSvc)

	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)
	panelHandler := panel.NewHandler(panelSvc)

	storySvc := story.NewService(aiClient, assetSvc, chapterSvc, panelSvc)
	storyHandler := story.NewHandler(storySvc)

	// The render flow downloads the generated image from a remote URL; give it a
	// generous timeout independent of the AI client's request timeout.
	renderSvc := render.NewService(
		aiClient, panelSvc, chapterSvc, assetSvc, bookRepo,
		userRepo, cfg.ImageCreditCost, media,
		&http.Client{Timeout: 120 * time.Second},
		cfg.RenderMaxRefs,
	)
	renderHandler := render.NewHandler(renderSvc)

	// Serve the built SPA when a dist directory is present. In dev the frontend
	// runs under Vite (with a proxy to this server), so a missing dist simply
	// leaves the backend in API-only mode.
	var static http.Handler
	webDist := get("WEB_DIST", "web/dist")
	if spa, err := httpx.NewSPAHandler(webDist); err != nil {
		log.Printf("SPA 未启用（%s 缺少 index.html，API-only 模式）", webDist)
	} else {
		static = spa
		log.Printf("SPA 已启用，从 %s 提供前端", webDist)
	}

	router := httpx.NewRouter(httpx.Deps{
		Session: sess,
		Auth:    authHandler,
		Book:    bookHandler,
		Asset:   assetHandler,
		Chapter: chapterHandler,
		Panel:   panelHandler,
		Story:   storyHandler,
		Render:  renderHandler,
		Media:   media.Handler(),
		Static:  static,
	})

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

// get returns the environment value for key k, or def when it is unset/empty.
func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadDotEnv reads a minimal .env file at path and sets any keys that are not
// already present in the process environment. It is intentionally dependency
// free: blank lines and lines beginning with '#' are skipped, and each entry is
// split on the first '='. Missing files are ignored. Values are never logged.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env is a valid configuration (env may be set externally)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
