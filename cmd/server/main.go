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
	"github.com/seven-agents/oh-my-commic/internal/config"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/httpx"
	"github.com/seven-agents/oh-my-commic/internal/panel"
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
	sess := auth.NewSession()

	authSvc := auth.NewService(auth.NewUserRepo(d), sess)
	authHandler := auth.NewHandler(authSvc)

	bookRepo := book.NewRepo(d)
	bookSvc := book.NewService(bookRepo)
	bookHandler := book.NewHandler(bookSvc)

	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	assetHandler := asset.NewHandler(assetSvc, media)

	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	chapterHandler := chapter.NewHandler(chapterSvc)

	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)
	panelHandler := panel.NewHandler(panelSvc)

	aiClient := &ai.Client{
		Key:          cfg.DashScopeKey,
		TextBaseURL:  cfg.TextBaseURL,
		ImageBaseURL: cfg.ImageBaseURL,
		TextModel:    cfg.TextModel,
		ImageModel:   cfg.ImageModel,
		HTTP:         &http.Client{Timeout: 60 * time.Second},
	}
	storySvc := story.NewService(aiClient, assetSvc, chapterSvc, panelSvc)
	storyHandler := story.NewHandler(storySvc)

	router := httpx.NewRouter(httpx.Deps{
		Session: sess,
		Auth:    authHandler,
		Book:    bookHandler,
		Asset:   assetHandler,
		Chapter: chapterHandler,
		Panel:   panelHandler,
		Story:   storyHandler,
		Media:   media.Handler(),
	})

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
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
