# oh-my-commic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个亲子向 AI 漫画书 Web 应用——多用户隔离，用户建书、建角色/场景、对话生成分镜、逐格生图、拼版成书。

**Architecture:** Go 单体后端（chi 路由，分层 handler→service→repository，SQLite 存元数据，本地文件按 book_id 存图片），React SPA 前端（Vite + TS + Tailwind），AI 走通义千问 DashScope（compatible-mode 文本 + 经典异步生图）。

**Tech Stack:** Go 1.22+, chi/v5, modernc.org/sqlite（纯 Go 免 cgo）, golang.org/x/crypto/bcrypt, React 18, Vite, TypeScript, Tailwind, react-router。

## Global Constraints

- **语言/风格**：默认吉卜力/宫崎骏风格；UI 面向小朋友，暖色、圆润、鼓励式。
- **隔离**：除 `users` 外每次数据访问都必须能追溯并校验 `user_id`；越权一律返回 404。
- **密钥**：`DASHSCOPE_API_KEY` 只存 `.env`，永不进 git（已在 `.gitignore`）。
- **图片存储**：本地文件系统，按 `data/{book_id}/` 分目录，文件名随机 id。
- **不可变**：service 层返回新对象，不原地修改入参。
- **AI 模型可配**：`QWEN_TEXT_MODEL`（默认 `qwen-plus`）、`QWEN_IMAGE_MODEL`（默认 `wan2.2-t2i-plus`，运行时按实际可用模型调整）。
- **提交**：每个 Task 末尾提交；commit 信息用中文，遵循 `type: 描述`。

---

## 数据模型（全局类型，各 Task 共用，务必字段名一致）

Go struct（`internal/models`）：

```go
type User struct {
    ID           int64  `json:"id"`
    Nickname     string `json:"nickname"`
    PasswordHash string `json:"-"`
    CreatedAt    string `json:"createdAt"`
}

type Book struct {
    ID        int64  `json:"id"`
    UserID    int64  `json:"userId"`
    Title     string `json:"title"`
    CoverURL  string `json:"coverUrl"`
    Style     string `json:"style"`   // 默认 "ghibli"
    Summary   string `json:"summary"`
    IsPublic  bool   `json:"isPublic"` // 分享/点赞后做, 先预留
    CreatedAt string `json:"createdAt"`
    UpdatedAt string `json:"updatedAt"`
}

type Character struct {
    ID          int64  `json:"id"`
    BookID      int64  `json:"bookId"`
    Type        string `json:"type"` // "character" | "pet"
    Name        string `json:"name"`
    Gender      string `json:"gender"`
    Age         string `json:"age"`
    Personality string `json:"personality"`
    Description string `json:"description"`
    ImageURL    string `json:"imageUrl"` // 用户上传, 作参考图
}

type Scene struct {
    ID          int64  `json:"id"`
    BookID      int64  `json:"bookId"`
    Name        string `json:"name"`
    Description string `json:"description"`
    ImageURL    string `json:"imageUrl"`
}

type Chapter struct {
    ID        int64  `json:"id"`
    BookID    int64  `json:"bookId"`
    Order     int    `json:"order"`
    Title     string `json:"title"`
    Status    string `json:"status"` // draft|storyboarding|rendering|done
    CreatedAt string `json:"createdAt"`
}

type Panel struct {
    ID           int64   `json:"id"`
    ChapterID    int64   `json:"chapterId"`
    Order        int     `json:"order"`
    Caption      string  `json:"caption"`      // 画面描述
    CharacterIDs []int64 `json:"characterIds"` // JSON 存储
    SceneID      int64   `json:"sceneId"`
    ImagePrompt  string  `json:"imagePrompt"`
    ImageURL     string  `json:"imageUrl"`
    Status       string  `json:"status"` // pending|rendering|done|failed
}
```

---

# Phase 0：项目骨架

### Task 0.1：Go module + chi + 健康检查

**Files:**
- Create: `go.mod`, `cmd/server/main.go`, `internal/httpx/router.go`, `internal/httpx/router_test.go`

**Interfaces:**
- Produces: `httpx.NewRouter() http.Handler`（挂载 `GET /api/health` → `{"ok":true}`）

- [ ] **Step 1: 初始化 module 与依赖**

```bash
cd /Users/seven/go/src/github.com/seven-agents/oh-my-commic
go mod init github.com/seven-agents/oh-my-commic
go get github.com/go-chi/chi/v5@latest
go get modernc.org/sqlite@latest
go get golang.org/x/crypto/bcrypt@latest
```

- [ ] **Step 2: 写失败测试** `internal/httpx/router_test.go`

```go
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(NewRouter())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Fatalf("want 200, got %d", resp.StatusCode) }
	var body map[string]bool
	json.NewDecoder(resp.Body).Decode(&body)
	if !body["ok"] { t.Fatalf("want ok=true, got %v", body) }
}
```

- [ ] **Step 3: 跑测试确认失败** — `go test ./internal/httpx/` → 编译失败（NewRouter 未定义）
- [ ] **Step 4: 实现** `internal/httpx/router.go`

```go
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 5: `cmd/server/main.go`**

```go
package main

import (
	"log"
	"net/http"

	"github.com/seven-agents/oh-my-commic/internal/httpx"
)

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", httpx.NewRouter()))
}
```

- [ ] **Step 6: 跑测试确认通过** — `go test ./...`
- [ ] **Step 7: Commit** — `git add -A && git commit -m "feat: Go 骨架与健康检查"`

### Task 0.2：配置加载（.env）

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`
- Create（本地，勿提交）: `.env`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)`，字段 `Port, DBPath, DataDir, DashScopeKey, TextModel, ImageModel, TextBaseURL, ImageBaseURL`

- [ ] **Step 1: 写失败测试** `config_test.go`

```go
package config

import ("os"; "testing")

func TestLoadDefaults(t *testing.T) {
	os.Setenv("DASHSCOPE_API_KEY", "sk-test")
	c, err := Load()
	if err != nil { t.Fatal(err) }
	if c.TextModel != "qwen-plus" { t.Fatalf("default text model wrong: %s", c.TextModel) }
	if c.DashScopeKey != "sk-test" { t.Fatalf("key not loaded") }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** `config.go`（用 `os.Getenv` + 默认值；启动时缺 `DASHSCOPE_API_KEY` 返回 error）

```go
package config

import ("errors"; "os")

type Config struct {
	Port, DBPath, DataDir            string
	DashScopeKey                     string
	TextModel, ImageModel            string
	TextBaseURL, ImageBaseURL        string
}

func get(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

func Load() (Config, error) {
	c := Config{
		Port:         get("PORT", "8080"),
		DBPath:       get("DB_PATH", "oh-my-commic.db"),
		DataDir:      get("DATA_DIR", "data"),
		DashScopeKey: os.Getenv("DASHSCOPE_API_KEY"),
		TextModel:    get("QWEN_TEXT_MODEL", "qwen-plus"),
		ImageModel:   get("QWEN_IMAGE_MODEL", "wan2.2-t2i-plus"),
		TextBaseURL:  get("QWEN_TEXT_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		ImageBaseURL: get("QWEN_IMAGE_BASE_URL", "https://dashscope.aliyuncs.com/api/v1"),
	}
	if c.DashScopeKey == "" { return c, errors.New("DASHSCOPE_API_KEY 未设置") }
	return c, nil
}
```

- [ ] **Step 4: 写 `.env.example`**（不含真实 key）与本地 `.env`（含用户提供的真实 key）

```
DASHSCOPE_API_KEY=sk-你的key
QWEN_TEXT_MODEL=qwen-plus
QWEN_IMAGE_MODEL=wan2.2-t2i-plus
```

- [ ] **Step 5: 跑测试确认通过；确认 `git status` 里没有 `.env`**
- [ ] **Step 6: Commit** — `git add -A && git commit -m "feat: 配置加载与 .env"`

### Task 0.3：SQLite 连接与迁移

**Files:**
- Create: `internal/db/db.go`, `internal/db/migrate.go`, `internal/db/db_test.go`

**Interfaces:**
- Produces: `db.Open(path string) (*sql.DB, error)`（含 `Migrate`，建全部表：users, books, characters, scenes, chapters, panels）

- [ ] **Step 1: 写失败测试**（用 `:memory:` 或临时文件，建表后能查到 `sqlite_master` 里 6 张表）

```go
package db

import ("testing")

func TestMigrateCreatesTables(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer d.Close()
	for _, tbl := range []string{"users","books","characters","scenes","chapters","panels"} {
		var name string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil { t.Fatalf("表 %s 不存在: %v", tbl, err) }
	}
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** `db.go`（`_ "modernc.org/sqlite"`，`sql.Open("sqlite", path)`，开启外键 `PRAGMA foreign_keys=ON`）与 `migrate.go`（按上文数据模型建表；`character_ids` 在 panels 里存 TEXT(JSON)；外键级联）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: SQLite 连接与迁移"`

---

# Phase 1：认证与隔离（面试重点）

### Task 1.1：用户仓库 + 密码哈希

**Files:**
- Create: `internal/auth/repo.go`, `internal/auth/repo_test.go`, `internal/models/models.go`

**Interfaces:**
- Produces: `auth.UserRepo` 接口 `Create(nickname, passwordHash string) (models.User, error)`、`ByNickname(string) (models.User, error)`；哈希用 `bcrypt`。

- [ ] **Step 1: 写失败测试**（Create 后能 ByNickname 查回；重复昵称报错）

```go
func TestCreateAndFetch(t *testing.T) {
	d, _ := db.Open(":memory:")
	repo := NewUserRepo(d)
	u, err := repo.Create("小明", "hash")
	if err != nil || u.ID == 0 { t.Fatalf("create failed: %v", err) }
	got, err := repo.ByNickname("小明")
	if err != nil || got.ID != u.ID { t.Fatalf("fetch mismatch") }
	if _, err := repo.Create("小明", "hash2"); err == nil { t.Fatal("重复昵称应报错") }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** `models.go`（贴上文全部 struct）与 `repo.go`（`users.nickname` 唯一索引）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 用户仓库与模型"`

### Task 1.2：注册/登录 + session

**Files:**
- Create: `internal/auth/service.go`, `internal/auth/session.go`, `internal/auth/handler.go`, `internal/auth/service_test.go`

**Interfaces:**
- Produces:
  - `service.Register(nickname, password string) (models.User, error)`（bcrypt 哈希）
  - `service.Login(nickname, password string) (token string, u models.User, err error)`
  - session：内存 map `token→userID`（48h 够用），`session.Issue(userID)`、`session.UserID(token)`
  - handler：`POST /api/register`、`POST /api/login`、`POST /api/logout`；登录成功 set cookie `session=token`

- [ ] **Step 1: 写失败测试**（错误密码登录失败；正确密码返回 token）

```go
func TestRegisterLogin(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Register("小明", "pw123456"); err != nil { t.Fatal(err) }
	if _, _, err := svc.Login("小明", "wrong"); err == nil { t.Fatal("错误密码应失败") }
	tok, u, err := svc.Login("小明", "pw123456")
	if err != nil || tok == "" || u.Nickname != "小明" { t.Fatalf("登录应成功: %v", err) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** service（bcrypt `GenerateFromPassword`/`CompareHashAndPassword`）、session（`crypto/rand` 生成 token）、handler
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 注册登录与 session"`

### Task 1.3：隔离中间件（核心）

**Files:**
- Create: `internal/auth/middleware.go`, `internal/auth/middleware_test.go`

**Interfaces:**
- Produces:
  - `auth.RequireUser(sess *Session) func(http.Handler) http.Handler`：从 cookie 取 token→userID，注入 context；无效→401
  - `auth.UserID(ctx context.Context) int64`：取当前用户 id

- [ ] **Step 1: 写失败测试**（无 cookie→401；有效 cookie→handler 能拿到正确 userID）

```go
func TestRequireUser(t *testing.T) {
	sess := NewSession()
	tok := sess.Issue(42)
	var seen int64
	h := RequireUser(sess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserID(r.Context()); w.WriteHeader(200)
	}))
	// 无 cookie
	rec := httptest.NewRecorder(); h.ServeHTTP(rec, httptest.NewRequest("GET","/",nil))
	if rec.Code != 401 { t.Fatalf("无 cookie 应 401, got %d", rec.Code) }
	// 有 cookie
	req := httptest.NewRequest("GET","/",nil); req.AddCookie(&http.Cookie{Name:"session",Value:tok})
	rec = httptest.NewRecorder(); h.ServeHTTP(rec, req)
	if rec.Code != 200 || seen != 42 { t.Fatalf("应注入 userID=42, got %d code=%d", seen, rec.Code) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** middleware（context key 用非导出类型避免碰撞）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 用户隔离中间件"`

---

# Phase 2：本地图片存储

### Task 2.1：存储模块（按 book_id + 路径安全）

**Files:**
- Create: `internal/storage/storage.go`, `internal/storage/storage_test.go`

**Interfaces:**
- Produces:
  - `storage.Local{ Root string }`
  - `Save(bookID int64, ext string, r io.Reader) (relURL string, err error)`：写入 `Root/{bookID}/{randomID}{ext}`，返回可访问相对 URL `/media/{bookID}/{file}`
  - `SaveBytes(bookID int64, ext string, b []byte) (string, error)`
  - 文件服务：`Handler() http.Handler` 映射 `/media/*` 到 Root（**校验路径不含 `..`**）

- [ ] **Step 1: 写失败测试**（Save 后文件存在于正确 book 目录；返回 URL 前缀正确）

```go
func TestSaveUnderBookDir(t *testing.T) {
	s := Local{Root: t.TempDir()}
	url, err := s.SaveBytes(7, ".png", []byte("img"))
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(url, "/media/7/") { t.Fatalf("URL 前缀错: %s", url) }
	// 文件应存在
	p := filepath.Join(s.Root, strings.TrimPrefix(url, "/media/"))
	if _, err := os.Stat(p); err != nil { t.Fatalf("文件未落地: %v", err) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** storage（`crypto/rand` 生成文件名；`os.MkdirAll`；Handler 用 `http.FileServer` + 清洗路径）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 本地图片存储(按 book_id)"`

---

# Phase 3：Book CRUD

### Task 3.1：Book 仓库 + service + handler（隔离贯穿）

**Files:**
- Create: `internal/book/repo.go`, `internal/book/service.go`, `internal/book/handler.go`, `internal/book/service_test.go`

**Interfaces:**
- Produces:
  - repo：`Create(userID int64, title, style, summary string) (Book,error)`、`ListByUser(userID) ([]Book,error)`、`Get(userID, bookID) (Book,error)`（**查询强制带 userID**）、`Update`, `Delete`
  - handler：`GET/POST /api/books`、`GET/PUT/DELETE /api/books/{id}`（全部挂 RequireUser）

- [ ] **Step 1: 写失败测试**（**越权测试最重要**：用户 A 建书，用户 B `Get` 返回 not found）

```go
func TestBookIsolation(t *testing.T) {
	repo := newTestBookRepo(t)
	b, _ := repo.Create(1, "A的书", "ghibli", "")
	// 用户 2 不能拿到用户 1 的书
	if _, err := repo.Get(2, b.ID); err == nil { t.Fatal("越权访问应失败(not found)") }
	// 本人可以
	if _, err := repo.Get(1, b.ID); err != nil { t.Fatalf("本人应可访问: %v", err) }
	// 列表隔离
	list, _ := repo.ListByUser(2)
	if len(list) != 0 { t.Fatalf("用户2 不应看到任何书, got %d", len(list)) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** repo/service/handler（`Get`/`Update`/`Delete` 的 SQL 都带 `WHERE id=? AND user_id=?`；查不到返回业务 `ErrNotFound`→handler 映射 404）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: 挂路由到 `httpx.NewRouter`**（需把 Router 改为接收依赖；见下）
- [ ] **Step 6: Commit** — `git commit -m "feat: Book CRUD 与隔离"`

### Task 3.2：Router 组装依赖

**Files:**
- Modify: `internal/httpx/router.go`, `cmd/server/main.go`

**Interfaces:**
- Produces: `httpx.NewRouter(deps Deps) http.Handler`，`Deps` 聚合 session、各 handler、storage.Handler。

- [ ] **Step 1: 改造 Router 接收 `Deps`**，注册 auth、book 路由与 `/media/*`；`main.go` 里 `config.Load`→`db.Open`→`Migrate`→构造各层→`NewRouter(deps)`
- [ ] **Step 2: 手动验证**：`go run ./cmd/server`，用 curl 走通 注册→登录(拿 cookie)→建书→列书

```bash
curl -s -X POST localhost:8080/api/register -d '{"nickname":"小明","password":"pw123456"}'
curl -s -c cookie.txt -X POST localhost:8080/api/login -d '{"nickname":"小明","password":"pw123456"}'
curl -s -b cookie.txt -X POST localhost:8080/api/books -d '{"title":"星星的故事"}'
curl -s -b cookie.txt localhost:8080/api/books
```

- [ ] **Step 3: Commit** — `git commit -m "feat: 路由组装与依赖注入"`

---

# Phase 4：资产（角色/宠物/场景）CRUD + 上传

### Task 4.1：Asset 仓库 + service（角色 & 场景）

**Files:**
- Create: `internal/asset/repo.go`, `internal/asset/service.go`, `internal/asset/repo_test.go`

**Interfaces:**
- Produces（均带 userID→bookID 归属校验：先 `book.Get(userID,bookID)` 确认书属于用户）:
  - `CreateCharacter(userID, bookID int64, c models.Character) (models.Character, error)`
  - `ListCharacters(userID, bookID) ([]models.Character, error)`
  - `CreateScene(userID, bookID int64, s models.Scene) (models.Scene, error)`
  - `ListScenes(userID, bookID) ([]models.Scene, error)`
  - `UpdateCharacter/DeleteCharacter/UpdateScene/DeleteScene`

- [ ] **Step 1: 写失败测试**（在他人书下建角色应失败；本人书下建/列正常）

```go
func TestCharacterBelongsToOwnedBook(t *testing.T) {
	env := newAssetTestEnv(t)          // 含 book.Repo + asset.Repo
	b, _ := env.books.Create(1, "书", "ghibli", "")
	// 用户 2 在用户 1 的书下建角色 → 失败
	if _, err := env.assets.CreateCharacter(2, b.ID, models.Character{Name:"狐狸",Type:"character"}); err == nil {
		t.Fatal("越权建角色应失败")
	}
	c, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name:"狐狸",Type:"character"})
	if err != nil || c.ID == 0 { t.Fatalf("本人建角色应成功: %v", err) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** repo/service（角色 `character_ids` 无关；场景对称实现）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 资产仓库(角色/场景)与归属校验"`

### Task 4.2：图片上传 handler（multipart → storage）

**Files:**
- Create: `internal/asset/handler.go`, `internal/asset/handler_test.go`
- Modify: `internal/httpx/router.go`

**Interfaces:**
- Produces:
  - `POST /api/books/{bookId}/upload`（multipart 字段 `file`）→ 校验类型(png/jpg/webp)与大小(≤5MB)→ `storage.Save(bookID,...)` → `{"imageUrl":"/media/.."}`
  - `GET/POST /api/books/{bookId}/characters`、`PUT/DELETE .../characters/{id}`
  - 场景同构：`.../scenes`

- [ ] **Step 1: 写失败测试**（上传非图片类型→400；正常 png→返回 imageUrl）
- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** handler（`r.ParseMultipartForm`，`http.DetectContentType` 校验白名单）
- [ ] **Step 4: 跑测试确认通过 + 手动 curl 上传一张图**
- [ ] **Step 5: Commit** — `git commit -m "feat: 资产上传与 CRUD 接口"`

---

# Phase 5：Chapter + Panel

### Task 5.1：Chapter CRUD + 状态机

**Files:**
- Create: `internal/chapter/repo.go`, `internal/chapter/service.go`, `internal/chapter/handler.go`, `internal/chapter/service_test.go`
- Modify: `internal/httpx/router.go`

**Interfaces:**
- Produces:
  - `CreateChapter(userID, bookID int64, title string) (models.Chapter, error)`（初始 status=`draft`，order 取当前最大+1）
  - `ListChapters(userID, bookID)`、`GetChapter(userID, chapterID)`（经 book 归属校验）
  - `SetStatus(userID, chapterID, status)`（合法流转：draft→storyboarding→rendering→done）
  - 路由：`GET/POST /api/books/{bookId}/chapters`、`GET /api/chapters/{id}`

- [ ] **Step 1: 写失败测试**（非法状态流转报错；order 自增）
- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 章节 CRUD 与状态机"`

### Task 5.2：Panel 仓库 + 排序 + 批量替换

**Files:**
- Create: `internal/panel/repo.go`, `internal/panel/service.go`, `internal/panel/handler.go`, `internal/panel/repo_test.go`
- Modify: `internal/httpx/router.go`

**Interfaces:**
- Produces:
  - `ReplacePanels(userID, chapterID int64, panels []models.Panel) ([]models.Panel, error)`（分镜确认后整章替换，重排 order）
  - `ListPanels(userID, chapterID)`、`UpdatePanel(userID, panelID, fields)`、`SetPanelImage(userID, panelID, url)`、`SetPanelStatus`
  - `character_ids` 用 JSON 序列化存 TEXT
  - 路由：`GET /api/chapters/{id}/panels`、`PUT /api/chapters/{id}/panels`（批量替换）、`PUT /api/panels/{id}`

- [ ] **Step 1: 写失败测试**（ReplacePanels 后 order 为 0..n-1；CharacterIDs JSON 往返一致）

```go
func TestReplacePanelsReorders(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, /*userID*/1)
	in := []models.Panel{{Caption:"A",CharacterIDs:[]int64{2,3}},{Caption:"B"}}
	out, err := env.panels.ReplacePanels(1, ch.ID, in)
	if err != nil { t.Fatal(err) }
	if out[0].Order != 0 || out[1].Order != 1 { t.Fatalf("order 未重排: %+v", out) }
	got, _ := env.panels.ListPanels(1, ch.ID)
	if len(got[0].CharacterIDs) != 2 || got[0].CharacterIDs[0] != 2 { t.Fatalf("CharacterIDs JSON 往返错: %+v", got[0]) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现**（ReplacePanels 用事务：删旧插新）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 分镜仓库与批量排序"`

---

# Phase 6：AI 网关 — 文本分镜

### Task 6.1：DashScope 文本客户端（compatible-mode）

**Files:**
- Create: `internal/ai/client.go`, `internal/ai/chat.go`, `internal/ai/chat_test.go`

**Interfaces:**
- Produces:
  - `ai.Client{ Key, TextBaseURL, ImageBaseURL, TextModel, ImageModel string, HTTP *http.Client }`
  - `Chat(ctx, messages []ai.Msg) (string, error)`：POST `{TextBaseURL}/chat/completions`，`Authorization: Bearer`，OpenAI 格式，取 `choices[0].message.content`
  - `ai.Msg{ Role, Content string }`

- [ ] **Step 1: 写失败测试**（用 `httptest.Server` 伪造 DashScope，断言请求头带 Bearer、body 含 model，返回解析正确）

```go
func TestChatParsesContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-x" { t.Fatal("缺 Bearer") }
		w.Write([]byte(`{"choices":[{"message":{"content":"你好"}}]}`))
	}))
	defer ts.Close()
	c := Client{Key:"sk-x", TextBaseURL: ts.URL, TextModel:"qwen-plus", HTTP: ts.Client()}
	got, err := c.Chat(context.Background(), []Msg{{Role:"user",Content:"hi"}})
	if err != nil || got != "你好" { t.Fatalf("解析失败: %q %v", got, err) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** client + chat
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 千问文本客户端"`

### Task 6.2：对话式分镜编排 + 结构化索引

**Files:**
- Create: `internal/ai/storyboard.go`, `internal/ai/storyboard_test.go`, `internal/ai/prompts.go`
- Create: `internal/story/service.go`, `internal/story/handler.go`
- Modify: `internal/httpx/router.go`

**Interfaces:**
- Produces:
  - `ai.Converse(ctx, history []Msg, assets AssetContext) (reply string, err error)`（阶段A 多轮对话；system prompt 注入本书角色/场景清单）
  - `ai.GenStoryboard(ctx, history []Msg, assets AssetContext, n int) ([]ai.PanelDraft, error)`：要求模型**只输出 JSON 数组**，每项 `{caption, characterIds, sceneId, imagePrompt}`；用 `json.Unmarshal` 解析（容错：截取首个 `[` 到末个 `]`）
  - `ai.AssetContext{ Characters []models.Character, Scenes []models.Scene }`
  - `ai.PanelDraft{ Caption string; CharacterIDs []int64; SceneID int64; ImagePrompt string }`
  - 路由：`POST /api/chapters/{id}/converse`（body: 消息历史）、`POST /api/chapters/{id}/storyboard`（生成分镜草稿并 `ReplacePanels` 落库，章节状态→storyboarding）

- [ ] **Step 1: 写失败测试**（伪造返回一段带前后缀噪音的 JSON，断言 `GenStoryboard` 能鲁棒解析出 N 个 draft，characterIds 正确）

```go
func TestGenStoryboardParsesJSON(t *testing.T) {
	body := "好的，这是分镜：\n[{\"caption\":\"小狐狸出发\",\"characterIds\":[1],\"sceneId\":2,\"imagePrompt\":\"fox in forest\"}]\n希望满意"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":`+strconv.Quote(body)+`}}]}`))
	}))
	defer ts.Close()
	c := Client{Key:"sk-x", TextBaseURL: ts.URL, TextModel:"qwen-plus", HTTP: ts.Client()}
	drafts, err := GenStoryboard(context.Background(), c, nil, AssetContext{}, 1)
	if err != nil || len(drafts) != 1 { t.Fatalf("应解析出1个: %v", err) }
	if drafts[0].CharacterIDs[0] != 1 || drafts[0].SceneID != 2 { t.Fatalf("字段解析错: %+v", drafts[0]) }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** storyboard（prompts.go 里放 system 模板：说明风格、要求 JSON、列出可用角色/场景 id+名字）、story service/handler
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 对话分镜与 AI 索引"`

---

# Phase 7：AI 网关 — 生图

### Task 7.1：DashScope 生图客户端（异步提交+轮询）

**Files:**
- Create: `internal/ai/image.go`, `internal/ai/image_test.go`

**Interfaces:**
- Produces:
  - `GenerateImage(ctx, prompt string, refImageURLs []string) (imageURL string, err error)`：
    - POST `{ImageBaseURL}/services/aigc/text2image/image-synthesis`，头 `X-DashScope-Async: enable`，body `{model, input:{prompt}, parameters:{n:1,size:"1024*1024"}}` → 拿 `output.task_id`
    - 轮询 `GET {ImageBaseURL}/tasks/{task_id}` 直到 `task_status` ∈ {SUCCEEDED,FAILED}；成功取 `output.results[0].url`
    - 轮询上限（如 60 次 * 2s）超时报错
  - 参考图：若模型支持，将 refImageURLs 拼入请求（wan 系列 `input.ref_img` / 或写进 prompt 说明；先按无参考图跑通，再加）

- [ ] **Step 1: 写失败测试**（伪造：第一次 POST 返回 task_id；GET 第一次 PENDING、第二次 SUCCEEDED 带 url。断言最终拿到 url，且轮询发生≥1次）

```go
func TestGenerateImagePolls(t *testing.T) {
	var polls int
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request){
		if r.Header.Get("X-DashScope-Async") != "enable" { t.Fatal("缺 async 头") }
		w.Write([]byte(`{"output":{"task_id":"t1","task_status":"PENDING"}}`))
	})
	mux.HandleFunc("/tasks/t1", func(w http.ResponseWriter, r *http.Request){
		polls++
		if polls < 2 { w.Write([]byte(`{"output":{"task_status":"PENDING"}}`)); return }
		w.Write([]byte(`{"output":{"task_status":"SUCCEEDED","results":[{"url":"http://img/x.png"}]}}`))
	})
	ts := httptest.NewServer(mux); defer ts.Close()
	c := Client{Key:"sk-x", ImageBaseURL: ts.URL, ImageModel:"wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: time.Millisecond}
	url, err := c.GenerateImage(context.Background(), "fox", nil)
	if err != nil || url != "http://img/x.png" { t.Fatalf("应拿到 url: %q %v", url, err) }
	if polls < 2 { t.Fatal("应至少轮询2次") }
}
```

- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** image.go（`PollInterval` 可注入以便测试）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit** — `git commit -m "feat: 千问生图客户端(异步轮询)"`

### Task 7.2：单格生图编排（拉取远程图→落本地→写回 Panel）

**Files:**
- Create: `internal/story/render.go`, `internal/story/render_test.go`
- Modify: `internal/story/handler.go`, `internal/httpx/router.go`

**Interfaces:**
- Produces:
  - `RenderPanel(userID, panelID int64) (models.Panel, error)`：
    1. 取 panel + 所在 chapter/book 归属校验
    2. 组 prompt = 吉卜力风格前缀 + panel.Caption + 命中角色设定摘要；参考图 = 命中角色/场景的 ImageURL（转成 DashScope 可访问的公网 URL 或先不传，见风险）
    3. 调 `GenerateImage` → 得远程 url → 下载字节 → `storage.SaveBytes(bookID,...)` → `SetPanelImage`，状态→done；失败→failed
  - 路由：`POST /api/panels/{id}/render`（同步返回更新后的 panel；前端逐格触发）

- [ ] **Step 1: 写失败测试**（用 fake ai + fake storage，断言渲染后 panel.ImageURL 落到本地 `/media/{bookID}/`，状态 done）
- [ ] **Step 2: 跑测试确认失败**
- [ ] **Step 3: 实现** render（用接口注入 ai 与 storage 便于测试）
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: 手动端到端**：真 key 跑一次单格生图，确认本地 `data/{bookId}/` 出现图片
- [ ] **Step 6: Commit** — `git commit -m "feat: 单格生图编排与落库"`

---

# Phase 8：前端骨架

> 前端以"构建 + 人工验证"为主，不写单测。每个页面完成后由用户看运行效果并按 YAML 结构图提意见。

### Task 8.1：Vite + React + TS + Tailwind + 路由 + API client

**Files:**
- Create: `web/`（`package.json`, `vite.config.ts`, `tailwind.config.js`, `index.html`, `src/main.tsx`, `src/App.tsx`, `src/api/client.ts`, `src/routes.tsx`）

**Interfaces:**
- Produces:
  - `api.get/post/put/del(path, body?)`：`fetch` 封装，`credentials:'include'`（带 cookie），基址 `/api`，错误统一抛出
  - 路由骨架：`/login /(书架) /books/:id /books/:id/assets/... /chapters/:id /read/:chapterId`
  - Vite proxy：`/api` 与 `/media` 转发到 `localhost:8080`

- [ ] **Step 1: `npm create vite@latest web -- --template react-ts`，装 tailwind、react-router-dom**
- [ ] **Step 2: 配 Tailwind + 吉卜力风 tokens**（暖色系、圆角、柔和阴影，写进 `tailwind.config.js` 与全局 CSS）
- [ ] **Step 3: 写 `api/client.ts` 与路由骨架、空白页占位**
- [ ] **Step 4: 手动验证** `npm run dev`，各路由能打开占位页
- [ ] **Step 5: Commit** — `git commit -m "feat: 前端骨架(Vite/React/Tailwind/路由)"`

### Task 8.2：认证页 P1 + 登录态

**Files:**
- Create: `web/src/pages/Login.tsx`, `web/src/auth/useAuth.ts`

- [ ] **Step 1: 实现** 登录/注册表单（切换 tab），成功后跳书架；`useAuth` 保存当前用户、未登录重定向到 `/login`
- [ ] **Step 2: 手动验证** 注册→登录→进入书架
- [ ] **Step 3: Commit** — `git commit -m "feat: 前端登录注册页"`

---

# Phase 9：前端 P2 书架 + P3 Book 工作台

### Task 9.1：书架主页 P2

**Files:**
- Create: `web/src/pages/Bookshelf.tsx`, `web/src/components/BookCard.tsx`

- [ ] **Step 1: 实现** 我的书封面网格 + "＋新建书"弹窗（标题/风格）；点卡片进 Book 工作台
- [ ] **Step 2: 手动验证** 建书、列书、进入
- [ ] **Step 3: Commit** — `git commit -m "feat: 书架主页"`

### Task 9.2：Book 工作台 P3

**Files:**
- Create: `web/src/pages/BookWorkspace.tsx`, `web/src/components/AssetPanel.tsx`, `web/src/components/ChapterList.tsx`

- [ ] **Step 1: 实现** 书信息头 + 演员表面板(角色/宠物/场景 列表 + ＋新建入口) + 章节列表(+新建章节)
- [ ] **Step 2: 手动验证** 面板数据正确、入口跳转正确
- [ ] **Step 3: Commit** — `git commit -m "feat: Book 工作台"`

---

# Phase 10：前端 P4 资产编辑

### Task 10.1：资产编辑页（上传图 + 表单）

**Files:**
- Create: `web/src/pages/AssetEditor.tsx`, `web/src/components/ImageUpload.tsx`

- [ ] **Step 1: 实现** 角色：上传图片(预览) + 名字/性别/年龄/性格/描述；场景：上传图 + 名字/描述；保存调后端
- [ ] **Step 2: 手动验证** 上传一张角色图并保存，回到工作台能看到该角色卡片(带图)
- [ ] **Step 3: Commit** — `git commit -m "feat: 资产编辑页"`

---

# Phase 11：前端 P5 Chapter 编辑器（核心）

### Task 11.1：阶段A 对话式生成分镜

**Files:**
- Create: `web/src/pages/ChapterEditor.tsx`, `web/src/components/ChatStoryboard.tsx`

- [ ] **Step 1: 实现** 故事介绍输入 + 人机对话区(调 `/converse`) + "产出分镜脚本"按钮(调 `/storyboard` 落库)
- [ ] **Step 2: 手动验证** 输入一句话，聊几轮，生成一组分镜草稿
- [ ] **Step 3: Commit** — `git commit -m "feat: 分镜对话阶段"`

### Task 11.2：阶段B 逐格生图

**Files:**
- Create: `web/src/components/PanelCard.tsx`, `web/src/components/PanelGrid.tsx`

- [ ] **Step 1: 实现** 分镜卡列表：每卡显示描述 + 自动索引到的角色/场景(可微调增删) + 「生成这张图」(调 `/panels/{id}/render`) + 生成中骨架 + 单格重生成
- [ ] **Step 2: 手动验证** 逐格生成，图片显示，重生成可用
- [ ] **Step 3: Commit** — `git commit -m "feat: 逐格生图"`

### Task 11.3：阶段C 拼版成页

**Files:**
- Create: `web/src/components/ComicCompose.tsx`

- [ ] **Step 1: 实现** 所有分镜图按网格模板拼版预览 + "保存成章"(章节状态→done)
- [ ] **Step 2: 手动验证** 拼版展示正常
- [ ] **Step 3: Commit** — `git commit -m "feat: 拼版成页"`

---

# Phase 12：前端 P6 阅读 + 整体打磨

### Task 12.1：漫画阅读页 P6

**Files:**
- Create: `web/src/pages/Reader.tsx`

- [ ] **Step 1: 实现** 翻页画布展示某章节拼好的漫画页(只读) + 章节切换
- [ ] **Step 2: 手动验证** 从工作台点已完成章节进入阅读
- [ ] **Step 3: Commit** — `git commit -m "feat: 漫画阅读页"`

### Task 12.2：吉卜力风打磨 + 端到端走查 + demo 脚本

**Files:**
- Create: `docs/DEMO.md`
- Modify: 各前端组件样式

- [ ] **Step 1: 统一** 配色/字体/圆角/插画式空状态/加载动画，贴合亲子风
- [ ] **Step 2: 端到端走查** 注册→建书→建角色/场景→新章节→对话分镜→逐格生图→拼版→阅读，记录 bug 修掉
- [ ] **Step 3: 写 `docs/DEMO.md`** 演示脚本（3 分钟主线 + 讲清三大亮点：隔离/AI索引/参考图一致性）
- [ ] **Step 4: Commit** — `git commit -m "chore: UI 打磨与 demo 脚本"`

---

## Self-Review（对照 spec 的覆盖检查）

- **多用户隔离**：Task 1.1–1.3（认证/中间件）+ 3.1/4.1/5.1 越权测试 ✅
- **Book 顶层 + 角色/场景属书**：Task 3.1 / 4.1 ✅
- **上传图作参考图**：Task 4.2（上传）+ 7.2（渲染时用参考图）✅
- **对话式分镜 + AI 索引**：Task 6.2 ✅
- **逐格生图 + 一致性**：Task 7.1 / 7.2 ✅
- **拼版成页 + 阅读**：Task 11.3 / 12.1 ✅
- **本地按 book_id 存图**：Task 2.1 / 7.2 ✅
- **6 页面全覆盖**：P1(8.2) P2(9.1) P3(9.2) P4(10.1) P5(11.x) P6(12.1) ✅
- **风险：参考图公网可达**：DashScope 拉取参考图需公网 URL，本地 `/media` 在开发机不可达 → 对策：生图阶段先跑通"无参考图"，一致性优先用"角色设定文字 + 固定 seed + 风格前缀"；若需真参考图，用 DashScope 图片上传接口或临时公网隧道，列为 Should（Task 7.1/7.2 内注明）。

未发现悬空类型或占位符。
