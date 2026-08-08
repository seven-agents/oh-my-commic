# 社区功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让未登录访客能浏览并完整阅读用户「公开」的漫画，登录用户可发布/管理自己的漫画到社区（含点赞、独立访客浏览量），主页提供「社区 / 我的漫画」两个入口。

**Architecture:** 后端在 `book` 包加一个 owner 写操作 `SetVisibility`；新增 `internal/community` 包做跨表**只读** feed / 公开阅读详情 + 点赞 / 浏览计数；新增 `OptionalUser` 中间件让公开端点可选读 session。前端把 `/` 改为公开 Home（两入口），书架挪到 `/my`，新增社区 feed 与公开阅读器（复用从 BookReader 抽出的 `BookReaderView`）。图片沿用已公开的 `/media`。

**Tech Stack:** Go(chi, modernc.org/sqlite)、React+Vite+TS+Tailwind、kin-openapi 契约 E2E、Vitest、Playwright。

## Global Constraints

- **多用户隔离 / 隐私**：公开只读端点**只**返回 `is_public=1` 的书；非公开 / 不存在一律 **404**（不泄露存在性）。作者信息**只**暴露 `nickname`+`avatarUrl`，**绝不**返回 username/email 或其它账号字段；公开阅读详情**不**返回章节 `conversation`/`panelCount`（storyboard 聊天内部数据）。
- **不可变**：service 返回新对象，不原地改入参。
- **小文件、单一职责**（200–400 行常态，800 上限）。
- **错误处理**：`%w` 包裹；绝不把 API key / 上游 body / 账号敏感字段返回客户端或打日志。
- **迁移**：只用幂等 `CREATE TABLE IF NOT EXISTS` / `ALTER TABLE ADD COLUMN`（`isDuplicateColumn` 容错）/ `CREATE INDEX IF NOT EXISTS`；旧库不重建。新表进 `schemaStatements`，新列进 `alterStatements`，索引进 `indexStatements`。
- **API 契约单一真相 = `docs/openapi.yaml`（OpenAPI 3.1）**：改任何 `/api/v1/*` 端点必须同步；`test/contract` 契约 E2E（kin-openapi 校验真实响应）必须覆盖新端点；前端 `web/src/api`（client.ts / types.ts）以 openapi 为准。
- **端点前缀**：业务端点全部在 `/api/v1/*`（handler 用资源相对路径 mount）。
- **浏览量 = 独立访客数**：`viewer_key` 登录用户 `u:{userId}`、匿名 `anon:{clientId}`；仅当关系表实际插入/删除（RowsAffected==1）才维护 `books` 上的反范式计数。
- **测试全 mock**：不碰真实 AI/key；DB 用 `db.Open(":memory:")` + `t.Cleanup(func(){ d.Close() })`。
- **git**：本功能在分支 `feat/community`（已创建）；提交信息中文 `type: 描述`，不带署名。

---

## 文件结构

**后端**
- `internal/db/migrate.go`（改）：新增 books 三列、book_likes / book_views 两表、feed 索引。
- `internal/models/models.go`（改）：`Book` 增 `LikeCount`/`ViewCount`/`PublishedAt`。
- `internal/book/repo.go`（改）：`bookColumns`/`scanBook` 加三列；新增 `SetVisibility`。
- `internal/book/service.go`（改）：新增 `SetVisibility`（归属 + published_at 语义）。
- `internal/book/handler.go`（改）：新增 `PUT /books/{id}/visibility`。
- `internal/auth/middleware.go`（改）：新增 `OptionalUser`。
- `internal/community/types.go`（新）：`Author`/`CommunityBook`/`ReaderChapter`/`CommunityBookDetail`/`LikeResult`。
- `internal/community/repo.go`（新）：`ListPublic`/`GetPublicDetail`/`Like`/`Unlike`/`RecordView`。
- `internal/community/service.go`（新）：入参校验（limit/offset 夹取）+ 委托 repo。
- `internal/community/handler.go`（新）：公开 + 需登录端点，`Mount`/`MountPublic`。
- `internal/httpx/router.go`（改）：接入 community handler（公开组用 OptionalUser，like 在 RequireUser 组，visibility 在 RequireUser 组）。
- `cmd/server/main.go`（改）：构造 community repo/service/handler，填 `Deps`。
- `docs/openapi.yaml`（改）：新端点 + schema。
- `test/contract/contract_test.go`（改）：覆盖新端点。

**前端**
- `web/src/lib/clientId.ts`（新）：`getClientId()`。
- `web/src/api/types.ts`（改）：`CommunityBook`/`CommunityBookDetail`/`ReaderChapter`/`Author`/`LikeResult`。
- `web/src/api/client.ts`（改）：`listCommunity`/`getCommunityBook`/`recordView`/`likeBook`/`unlikeBook`/`setVisibility`。
- `web/src/components/reader/BookReaderView.tsx`（新）：从 BookReader 抽出的纯展示组件。
- `web/src/pages/BookReader.tsx`（改）：改用 BookReaderView（owner 私有阅读，行为不变）。
- `web/src/pages/CommunityReader.tsx`（新）：公开阅读容器（记 view + 点赞）。
- `web/src/pages/Community.tsx`（新）：feed 网格。
- `web/src/components/CommunityCard.tsx`（新）：社区卡片。
- `web/src/pages/Home.tsx`（新）：两入口落地页。
- `web/src/pages/Bookshelf.tsx`（改）：公开/私密开关 + 计数。
- `web/src/App.tsx`（改）：路由（`/`→Home、`/my`→Bookshelf、`/community`、`/community/books/:id`）。
- `web/src/auth/useAuth.tsx` 或登录跳转处（改）：登录后导航到 `/my`。
- `web/e2e/community.spec.ts`（新）：匿名浏览 + owner 发布 E2E。
- 文档：`CLAUDE.md`、`docs/frontend-api.md`、`docs/ARCHITECTURE-AND-PROMPTS.md`。

---

## Task 1: DB 迁移 + models（books 计数列 / 两张新表 / feed 索引）

**Files:**
- Modify: `internal/db/migrate.go`
- Modify: `internal/models/models.go:24-35`（Book 结构体）
- Modify: `internal/book/repo.go`（bookColumns 常量 + scanBook）
- Test: `internal/db/migrate_test.go`（新建，如已存在则追加）

**Interfaces:**
- Produces: `models.Book` 新字段 `LikeCount int`(json `likeCount`) / `ViewCount int`(json `viewCount`) / `PublishedAt string`(json `publishedAt`)；books 表列 `like_count`/`view_count`/`published_at`；表 `book_likes(book_id,user_id,created_at)` PK(book_id,user_id)；表 `book_views(book_id,viewer_key,created_at)` PK(book_id,viewer_key)。

- [ ] **Step 1: 写失败测试** — `internal/db/migrate_test.go`

```go
package db

import "testing"

func TestMigrateAddsCommunityTablesAndColumns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	// books 新列可写可读，默认 0/''。
	res, err := d.Exec(`INSERT INTO users (password_hash) VALUES ('h')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	uid, _ := res.LastInsertId()
	res, err = d.Exec(`INSERT INTO books (user_id, title) VALUES (?, '书')`, uid)
	if err != nil {
		t.Fatalf("insert book: %v", err)
	}
	bid, _ := res.LastInsertId()

	var like, view int
	var published string
	if err := d.QueryRow(
		`SELECT like_count, view_count, published_at FROM books WHERE id = ?`, bid,
	).Scan(&like, &view, &published); err != nil {
		t.Fatalf("select new columns: %v", err)
	}
	if like != 0 || view != 0 || published != "" {
		t.Fatalf("defaults wrong: like=%d view=%d published=%q", like, view, published)
	}

	// 两张新表存在且复合主键去重。
	if _, err := d.Exec(`INSERT INTO book_likes (book_id, user_id) VALUES (?, ?)`, bid, uid); err != nil {
		t.Fatalf("insert like: %v", err)
	}
	if _, err := d.Exec(`INSERT OR IGNORE INTO book_likes (book_id, user_id) VALUES (?, ?)`, bid, uid); err != nil {
		t.Fatalf("dup like should be ignored: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO book_views (book_id, viewer_key) VALUES (?, 'anon:x')`, bid); err != nil {
		t.Fatalf("insert view: %v", err)
	}

	// 幂等：再次 Migrate 不报错。
	if err := Migrate(d); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/db/ -run TestMigrateAddsCommunity -v`
Expected: FAIL（no such column: like_count / no such table: book_likes）

- [ ] **Step 3: 实现迁移** — 编辑 `internal/db/migrate.go`

在 `schemaStatements` 末尾（`settings` 表之后）追加两个元素：

```go
	`CREATE TABLE IF NOT EXISTS book_likes (
  book_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (book_id, user_id),
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS book_views (
  book_id INTEGER NOT NULL,
  viewer_key TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (book_id, viewer_key),
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
)`,
```

在 `alterStatements` 末尾追加三列：

```go
	`ALTER TABLE books ADD COLUMN like_count INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE books ADD COLUMN view_count INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE books ADD COLUMN published_at TEXT NOT NULL DEFAULT ''`,
```

在 `indexStatements` 末尾追加 feed 索引：

```go
	`CREATE INDEX IF NOT EXISTS idx_books_public_published ON books(is_public, published_at)`,
```

- [ ] **Step 4: 更新 models.Book** — 编辑 `internal/models/models.go`，在 `Book` 结构体 `UpdatedAt` 之后加：

```go
	LikeCount   int    `json:"likeCount"`
	ViewCount   int    `json:"viewCount"`
	PublishedAt string `json:"publishedAt"`
```

- [ ] **Step 5: 更新 book repo 的列与扫描** — 编辑 `internal/book/repo.go`

`bookColumns` 常量改为（末尾追加三列）：

```go
const bookColumns = "id, user_id, title, cover_url, style, summary, is_public, created_at, updated_at, like_count, view_count, published_at"
```

`scanBook` 的 `s.Scan(...)` 末尾追加三个字段：

```go
	if err := s.Scan(
		&b.ID, &b.UserID, &b.Title, &b.CoverURL, &b.Style,
		&b.Summary, &b.IsPublic, &b.CreatedAt, &b.UpdatedAt,
		&b.LikeCount, &b.ViewCount, &b.PublishedAt,
	); err != nil {
		return models.Book{}, err
	}
```

- [ ] **Step 6: 跑测试确认通过 + 回归**

Run: `go test ./internal/db/ ./internal/book/ -v`
Expected: PASS（含既有 book service/handler 测试不回归）

- [ ] **Step 7: 提交**

```bash
git add internal/db/migrate.go internal/db/migrate_test.go internal/models/models.go internal/book/repo.go
git commit -m "feat: 社区数据模型(books计数列/book_likes/book_views/feed索引)"
```

---

## Task 2: book.SetVisibility（发布/下架开关）

**Files:**
- Modify: `internal/book/repo.go`（新增 `SetVisibility` 方法）
- Modify: `internal/book/service.go`（新增 `SetVisibility`）
- Modify: `internal/book/handler.go`（新增 `PUT /books/{id}/visibility`）
- Test: `internal/book/service_test.go`（追加）、`internal/book/handler_test.go`（追加）

**Interfaces:**
- Consumes: `models.Book`（Task 1 新字段）、`Repo.Get`、`ErrNotFound`、`auth.UserID`、`auth.WithUserID`。
- Produces: `func (r *Repo) SetVisibility(userID, bookID int64, isPublic bool) (models.Book, error)`；`func (s *Service) SetVisibility(userID, bookID int64, isPublic bool) (models.Book, error)`；端点 `PUT /api/v1/books/{id}/visibility` body `{"isPublic":bool}` → 200 更新后的 book / 400 / 404。

- [ ] **Step 1: 写失败测试（repo/service 语义）** — `internal/book/service_test.go` 追加

```go
func TestSetVisibilityPublishesAndUnpublishes(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h'),(2,'h')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	svc := NewService(NewRepo(d))
	b, err := svc.Create(1, "书", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 发布：is_public=true 且 published_at 非空。
	pub, err := svc.SetVisibility(1, b.ID, true)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !pub.IsPublic || pub.PublishedAt == "" {
		t.Fatalf("publish should set is_public + published_at: %+v", pub)
	}

	// 下架：is_public=false，published_at 保留旧值。
	un, err := svc.SetVisibility(1, b.ID, false)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if un.IsPublic {
		t.Fatalf("unpublish should clear is_public: %+v", un)
	}
	if un.PublishedAt != pub.PublishedAt {
		t.Fatalf("unpublish must keep published_at: got %q want %q", un.PublishedAt, pub.PublishedAt)
	}

	// 非 owner：404。
	if _, err := svc.SetVisibility(2, b.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner should get ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/book/ -run TestSetVisibility -v`
Expected: FAIL（undefined: svc.SetVisibility）

- [ ] **Step 3: 实现 repo.SetVisibility** — `internal/book/repo.go` 追加

```go
// SetVisibility flips a book's public flag. Publishing (isPublic=true) stamps
// published_at with the current time so the community feed can order by recency;
// unpublishing leaves published_at untouched. It returns ErrNotFound when the
// book does not exist or belongs to another user. The refreshed row is returned.
func (r *Repo) SetVisibility(userID, bookID int64, isPublic bool) (models.Book, error) {
	var res sql.Result
	var err error
	if isPublic {
		res, err = r.db.Exec(
			`UPDATE books
			   SET is_public = 1, published_at = datetime('now'), updated_at = datetime('now')
			 WHERE id = ? AND user_id = ?`,
			bookID, userID,
		)
	} else {
		res, err = r.db.Exec(
			`UPDATE books
			   SET is_public = 0, updated_at = datetime('now')
			 WHERE id = ? AND user_id = ?`,
			bookID, userID,
		)
	}
	if err != nil {
		return models.Book{}, fmt.Errorf("set visibility for book %d for user %d: %w", bookID, userID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return models.Book{}, fmt.Errorf("set visibility for book %d for user %d: rows affected: %w", bookID, userID, err)
	}
	if affected == 0 {
		return models.Book{}, ErrNotFound
	}
	return r.Get(userID, bookID)
}
```

- [ ] **Step 4: 实现 service.SetVisibility** — `internal/book/service.go` 追加

```go
// SetVisibility publishes or unpublishes the book owned by userID. It returns
// ErrNotFound if the book is not owned by the user.
func (s *Service) SetVisibility(userID, bookID int64, isPublic bool) (models.Book, error) {
	return s.repo.SetVisibility(userID, bookID, isPublic)
}
```

- [ ] **Step 5: 跑 service 测试确认通过**

Run: `go test ./internal/book/ -run TestSetVisibility -v`
Expected: PASS

- [ ] **Step 6: 写 handler 失败测试** — `internal/book/handler_test.go` 追加（沿用文件里既有的 test 服务器/请求 helper；若无则用 `httptest` + `auth.WithUserID` 构造带用户上下文的请求）

```go
func TestVisibilityEndpoint(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	svc := NewService(NewRepo(d))
	b, _ := svc.Create(1, "书", "", "")
	h := NewHandler(svc)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUserID(req.Context(), 1)))
		})
	})
	r.Route("/api/v1", func(v1 chi.Router) { h.Mount(v1) })

	body := strings.NewReader(`{"isPublic":true}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+strconv.FormatInt(b.ID, 10)+"/visibility", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got models.Book
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsPublic {
		t.Fatalf("expected book public, got %+v", got)
	}
}
```

- [ ] **Step 7: 实现 handler** — `internal/book/handler.go`

在 `Mount` 里追加：

```go
	r.Put("/books/{id}/visibility", h.SetVisibility)
```

新增方法与输入类型：

```go
// visibilityInput is the JSON body accepted by SetVisibility.
type visibilityInput struct {
	IsPublic *bool `json:"isPublic"`
}

// SetVisibility handles PUT /api/v1/books/{id}/visibility.
func (h *Handler) SetVisibility(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	bookID, ok := parseID(w, r)
	if !ok {
		return
	}

	var in visibilityInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.IsPublic == nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	b, err := h.svc.SetVisibility(userID, bookID, *in.IsPublic)
	if err != nil {
		writeServiceError(w, err, "更新可见性失败")
		return
	}
	writeJSON(w, http.StatusOK, b)
}
```

> `visibilityInput.IsPublic` 用 `*bool` 以区分「缺失字段」与 `false`；缺失 → 400。

- [ ] **Step 8: 跑测试确认通过 + vet**

Run: `go test ./internal/book/ -v && go vet ./internal/book/`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/book/
git commit -m "feat: 书籍发布/下架开关(PUT /books/{id}/visibility)"
```

---

## Task 3: OptionalUser 中间件

**Files:**
- Modify: `internal/auth/middleware.go`
- Test: `internal/auth/middleware_test.go`（追加）

**Interfaces:**
- Consumes: `resolveUser`（已有私有函数）、`userIDKey`、`UserID`。
- Produces: `func OptionalUser(sess *Session) func(http.Handler) http.Handler` — 有有效 session 则把 userID 注入 context 并放行；无 session 也放行（**不** 401），此时 `auth.UserID(ctx)==0`。

- [ ] **Step 1: 写失败测试** — `internal/auth/middleware_test.go` 追加

```go
func TestOptionalUser(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`INSERT INTO users (id, password_hash) VALUES (7,'h')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	sess := NewSession(d)
	token, err := sess.Create(7)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	var seen int64
	h := OptionalUser(sess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// 无 cookie：放行，userID=0。
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusOK || seen != 0 {
		t.Fatalf("anon: code=%d seen=%d", rec.Code, seen)
	}

	// 有效 cookie：放行，userID=7。
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seen != 7 {
		t.Fatalf("authed: code=%d seen=%d", rec.Code, seen)
	}
}
```

> 注：`NewSession`/`sess.Create` 的确切签名以 `internal/auth/session.go` 为准；若 `Create` 名称/返回不同，按实际调整测试构造 token 的方式（目标是拿到一个能被 `resolveUser` 解析的有效 cookie 值）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/auth/ -run TestOptionalUser -v`
Expected: FAIL（undefined: OptionalUser）

- [ ] **Step 3: 实现** — `internal/auth/middleware.go` 追加

```go
// OptionalUser returns middleware for public endpoints that behave differently
// for authenticated users. It resolves the session cookie like RequireUser but
// NEVER rejects: a missing or unknown session simply passes through with no user
// ID in context (auth.UserID returns 0). A valid session injects the user ID so
// downstream handlers can personalize (e.g. compute a per-user "liked" flag or a
// stable viewer key) without gating access.
func OptionalUser(sess *Session) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := resolveUser(sess, r); ok {
				r = r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/auth/ -run TestOptionalUser -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -m "feat: OptionalUser 中间件(公开端点可选读 session)"
```

---

## Task 4: community 类型 + repo 只读（ListPublic / GetPublicDetail）

**Files:**
- Create: `internal/community/types.go`
- Create: `internal/community/repo.go`
- Test: `internal/community/repo_read_test.go`

**Interfaces:**
- Consumes: `models.Book`/`models.Chapter`/`models.Panel`、`database/sql`。
- Produces:
  - 类型见下（`Author`/`CommunityBook`/`ReaderChapter`/`CommunityBookDetail`/`LikeResult`）。
  - `func NewRepo(d *sql.DB) *Repo`
  - `func (r *Repo) ListPublic(viewerKey string, limit, offset int) ([]CommunityBook, error)` — `viewerKey==""` 表示匿名（`liked` 全 false）；按 `published_at DESC, id DESC` 排序。
  - `func (r *Repo) GetPublicDetail(viewerKey string, bookID int64) (CommunityBookDetail, error)` — 非公开/不存在返回 `ErrNotFound`；`chapters` 不含 conversation/panelCount。
  - `var ErrNotFound = errors.New("not found")`

> **viewerKey 约定（贯穿 Task 4–6）**：`liked` 的判定用 `user_id`，因此 repo 读取需要的是 **用户维度** key。为简化，`ListPublic`/`GetPublicDetail` 接收的 `viewerKey` 语义 = 登录用户传 `"u:{id}"`、匿名传 `""`。repo 内解析：若以 `"u:"` 开头则取其后数字作 user_id 关联 `book_likes` 判定 liked，否则 liked 恒 false。这样匿名（含 `anon:` 浏览 key）在读取时 liked 一律 false，符合「未登录不显示已赞」。

- [ ] **Step 1: 写类型文件** — `internal/community/types.go`

```go
// Package community serves the public, read-mostly community surface: a feed of
// books their owners have made public, a full read-only view of one public book,
// and per-book like / unique-view counters. Every read is filtered to
// is_public=1 and leaks no owner account fields beyond nickname + avatar.
package community

import "github.com/seven-agents/oh-my-commic/internal/models"

// Author is the minimal, privacy-safe attribution shown on community surfaces.
// It deliberately excludes username, email, and every other account field.
type Author struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

// CommunityBook is one feed list item: a public book plus its author, counters,
// and whether the requesting user has liked it (always false for anonymous).
type CommunityBook struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	CoverURL    string `json:"coverUrl"`
	Summary     string `json:"summary"`
	Author      Author `json:"author"`
	LikeCount   int    `json:"likeCount"`
	ViewCount   int    `json:"viewCount"`
	Liked       bool   `json:"liked"`
	PublishedAt string `json:"publishedAt"`
}

// ReaderChapter is a public, read-only chapter: only the fields the reader
// renders. It omits conversation and panelCount (storyboard-chat internals).
type ReaderChapter struct {
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	Order   int            `json:"order"`
	IsCover bool           `json:"isCover"`
	Panels  []models.Panel `json:"panels"`
}

// CommunityBookDetail is the full public read payload for one book: book meta +
// author + counters + liked + all chapters (each carrying its panels).
type CommunityBookDetail struct {
	ID        int64           `json:"id"`
	Title     string          `json:"title"`
	CoverURL  string          `json:"coverUrl"`
	Summary   string          `json:"summary"`
	Style     string          `json:"style"`
	Author    Author          `json:"author"`
	LikeCount int             `json:"likeCount"`
	ViewCount int             `json:"viewCount"`
	Liked     bool            `json:"liked"`
	Chapters  []ReaderChapter `json:"chapters"`
}

// LikeResult is returned by like / unlike: the fresh count and the new state.
type LikeResult struct {
	LikeCount int  `json:"likeCount"`
	Liked     bool `json:"liked"`
}
```

- [ ] **Step 2: 写失败测试** — `internal/community/repo_read_test.go`

```go
package community

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestListPublicOrdersAndFiltersPrivate(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`INSERT INTO users (id, nickname, avatar_url, password_hash) VALUES (1,'小明','/media/users/1/a.png','h')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// 两本公开(不同 published_at) + 一本私密。
	if _, err := d.Exec(`INSERT INTO books (id,user_id,title,summary,is_public,published_at) VALUES
	  (10,1,'先发','s1',1,'2026-08-08 10:00:00'),
	  (11,1,'后发','s2',1,'2026-08-08 12:00:00'),
	  (12,1,'私密','s3',0,'')`); err != nil {
		t.Fatalf("seed books: %v", err)
	}

	repo := NewRepo(d)
	list, err := repo.ListPublic("", 20, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 public, got %d", len(list))
	}
	if list[0].ID != 11 || list[1].ID != 10 {
		t.Fatalf("want newest-first [11,10], got [%d,%d]", list[0].ID, list[1].ID)
	}
	if list[0].Author.Nickname != "小明" || list[0].Author.AvatarURL != "/media/users/1/a.png" {
		t.Fatalf("author not populated: %+v", list[0].Author)
	}
	if list[0].Liked {
		t.Fatalf("anonymous liked must be false")
	}
}

func TestGetPublicDetailPrivateIs404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私密',0)`)

	repo := NewRepo(d)
	if _, err := repo.GetPublicDetail("", 12); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private book detail must be ErrNotFound, got %v", err)
	}
	if _, err := repo.GetPublicDetail("", 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing book detail must be ErrNotFound, got %v", err)
	}
}

func TestGetPublicDetailReturnsChaptersWithPanels(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,summary,style,is_public,published_at) VALUES (10,1,'书','梗概','ghibli',1,'2026-08-08 10:00:00')`)
	d.Exec(`INSERT INTO chapters (id,book_id,"order",title,summary,is_cover) VALUES (100,10,0,'封面','',1),(101,10,1,'第一章','章梗概',0)`)
	d.Exec(`INSERT INTO panels (id,chapter_id,"order",caption,image_url,status) VALUES (1000,101,0,'旁白','/media/10/x.png','done')`)

	repo := NewRepo(d)
	detail, err := repo.GetPublicDetail("", 10)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Title != "书" || detail.Author.Nickname != "小明" {
		t.Fatalf("detail meta wrong: %+v", detail)
	}
	if len(detail.Chapters) != 2 {
		t.Fatalf("want 2 chapters, got %d", len(detail.Chapters))
	}
	var first *ReaderChapter
	for i := range detail.Chapters {
		if detail.Chapters[i].Order == 1 {
			first = &detail.Chapters[i]
		}
	}
	if first == nil || len(first.Panels) != 1 || first.Panels[0].ImageURL != "/media/10/x.png" {
		t.Fatalf("chapter panels not populated: %+v", detail.Chapters)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/community/ -v`
Expected: FAIL（undefined: NewRepo / ErrNotFound）

- [ ] **Step 4: 实现 repo 只读部分** — `internal/community/repo.go`

```go
package community

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrNotFound is returned when a book does not exist or is not public. The two
// cases are indistinguishable so callers cannot probe for private books.
var ErrNotFound = errors.New("not found")

// Repo provides read + counter access to the community surface. All reads are
// filtered to is_public=1.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a Repo backed by d.
func NewRepo(d *sql.DB) *Repo {
	return &Repo{db: d}
}

// likeUserID extracts the numeric user id from a "u:{id}" viewer key, or 0 for
// anonymous / non-user keys (so liked resolves to false).
func likeUserID(viewerKey string) int64 {
	if strings.HasPrefix(viewerKey, "u:") {
		if id, err := strconv.ParseInt(viewerKey[2:], 10, 64); err == nil {
			return id
		}
	}
	return 0
}

// ListPublic returns public books newest-published first. viewerKey "u:{id}"
// personalizes the liked flag; "" (anonymous) yields liked=false throughout.
func (r *Repo) ListPublic(viewerKey string, limit, offset int) ([]CommunityBook, error) {
	uid := likeUserID(viewerKey)
	rows, err := r.db.Query(
		`SELECT b.id, b.title, b.cover_url, b.summary, b.like_count, b.view_count, b.published_at,
		        u.nickname, u.avatar_url,
		        EXISTS(SELECT 1 FROM book_likes l WHERE l.book_id = b.id AND l.user_id = ?) AS liked
		   FROM books b JOIN users u ON u.id = b.user_id
		  WHERE b.is_public = 1
		  ORDER BY b.published_at DESC, b.id DESC
		  LIMIT ? OFFSET ?`,
		uid, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list public books: %w", err)
	}
	defer rows.Close()

	out := make([]CommunityBook, 0)
	for rows.Next() {
		var c CommunityBook
		if err := rows.Scan(
			&c.ID, &c.Title, &c.CoverURL, &c.Summary, &c.LikeCount, &c.ViewCount, &c.PublishedAt,
			&c.Author.Nickname, &c.Author.AvatarURL, &c.Liked,
		); err != nil {
			return nil, fmt.Errorf("list public books: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list public books: %w", err)
	}
	return out, nil
}

// GetPublicDetail returns a full read-only view of one public book. It returns
// ErrNotFound if the book is missing or not public.
func (r *Repo) GetPublicDetail(viewerKey string, bookID int64) (CommunityBookDetail, error) {
	uid := likeUserID(viewerKey)
	var d CommunityBookDetail
	err := r.db.QueryRow(
		`SELECT b.id, b.title, b.cover_url, b.summary, b.style, b.like_count, b.view_count,
		        u.nickname, u.avatar_url,
		        EXISTS(SELECT 1 FROM book_likes l WHERE l.book_id = b.id AND l.user_id = ?) AS liked
		   FROM books b JOIN users u ON u.id = b.user_id
		  WHERE b.id = ? AND b.is_public = 1`,
		uid, bookID,
	).Scan(
		&d.ID, &d.Title, &d.CoverURL, &d.Summary, &d.Style, &d.LikeCount, &d.ViewCount,
		&d.Author.Nickname, &d.Author.AvatarURL, &d.Liked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CommunityBookDetail{}, ErrNotFound
	}
	if err != nil {
		return CommunityBookDetail{}, fmt.Errorf("get public detail %d: %w", bookID, err)
	}

	chapters, err := r.readerChapters(bookID)
	if err != nil {
		return CommunityBookDetail{}, err
	}
	d.Chapters = chapters
	return d, nil
}

// readerChapters loads all chapters (ordered) of a book with their panels,
// exposing only reader-visible fields.
func (r *Repo) readerChapters(bookID int64) ([]ReaderChapter, error) {
	rows, err := r.db.Query(
		`SELECT id, "order", title, summary, is_cover
		   FROM chapters WHERE book_id = ? ORDER BY "order" ASC, id ASC`,
		bookID,
	)
	if err != nil {
		return nil, fmt.Errorf("reader chapters for book %d: %w", bookID, err)
	}
	defer rows.Close()

	type chRow struct {
		id      int64
		chapter ReaderChapter
	}
	var chs []chRow
	for rows.Next() {
		var cr chRow
		if err := rows.Scan(&cr.id, &cr.chapter.Order, &cr.chapter.Title, &cr.chapter.Summary, &cr.chapter.IsCover); err != nil {
			return nil, fmt.Errorf("reader chapters for book %d: %w", bookID, err)
		}
		cr.chapter.Panels = []models.Panel{}
		chs = append(chs, cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reader chapters for book %d: %w", bookID, err)
	}

	out := make([]ReaderChapter, 0, len(chs))
	for _, cr := range chs {
		panels, err := r.chapterPanels(cr.id)
		if err != nil {
			return nil, err
		}
		cr.chapter.Panels = panels
		out = append(out, cr.chapter)
	}
	return out, nil
}

// chapterPanels loads a chapter's panels ordered for reading. Only the fields
// the reader needs are selected; the rest stay zero-valued.
func (r *Repo) chapterPanels(chapterID int64) ([]models.Panel, error) {
	rows, err := r.db.Query(
		`SELECT id, chapter_id, "order", caption, image_url, status
		   FROM panels WHERE chapter_id = ? ORDER BY "order" ASC, id ASC`,
		chapterID,
	)
	if err != nil {
		return nil, fmt.Errorf("panels for chapter %d: %w", chapterID, err)
	}
	defer rows.Close()

	out := make([]models.Panel, 0)
	for rows.Next() {
		var p models.Panel
		if err := rows.Scan(&p.ID, &p.ChapterID, &p.Order, &p.Caption, &p.ImageURL, &p.Status); err != nil {
			return nil, fmt.Errorf("panels for chapter %d: %w", chapterID, err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("panels for chapter %d: %w", chapterID, err)
	}
	return out, nil
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/community/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/community/types.go internal/community/repo.go internal/community/repo_read_test.go
git commit -m "feat: community 只读 repo(公开 feed 列表 + 公开阅读详情)"
```

---

## Task 5: community repo 写操作（Like / Unlike / RecordView，反范式计数维护）

**Files:**
- Modify: `internal/community/repo.go`
- Test: `internal/community/repo_write_test.go`

**Interfaces:**
- Consumes: `Repo`、`ErrNotFound`、`LikeResult`、`likeUserID`。
- Produces:
  - `func (r *Repo) Like(userID, bookID int64) (LikeResult, error)` — 目标须 is_public，否则 ErrNotFound；幂等（重复赞不重复计数）；返回最新 count + liked=true。
  - `func (r *Repo) Unlike(userID, bookID int64) (LikeResult, error)` — 幂等（未赞过取消不为负）；返回最新 count + liked=false。目标非公开/不存在 ErrNotFound。
  - `func (r *Repo) RecordView(bookID int64, viewerKey string) error` — 目标须 is_public，否则 ErrNotFound；`INSERT OR IGNORE` 到 book_views，仅新插入才 view_count+1（幂等去重）。

- [ ] **Step 1: 写失败测试** — `internal/community/repo_write_test.go`

```go
package community

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestLikeIsIdempotentAndMaintainsCount(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h'),(2,'小红','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'书',1,'t')`)
	repo := NewRepo(d)

	res, err := repo.Like(2, 10)
	if err != nil || res.LikeCount != 1 || !res.Liked {
		t.Fatalf("first like: res=%+v err=%v", res, err)
	}
	// 重复赞：仍为 1（幂等）。
	res, err = repo.Like(2, 10)
	if err != nil || res.LikeCount != 1 {
		t.Fatalf("dup like should stay 1: res=%+v err=%v", res, err)
	}
	// 另一个用户赞：2。
	res, _ = repo.Like(1, 10)
	if res.LikeCount != 2 {
		t.Fatalf("second user like should be 2, got %d", res.LikeCount)
	}
	// 取消赞：回到 1；再取消不为负。
	res, err = repo.Unlike(2, 10)
	if err != nil || res.LikeCount != 1 || res.Liked {
		t.Fatalf("unlike: res=%+v err=%v", res, err)
	}
	res, _ = repo.Unlike(2, 10)
	if res.LikeCount != 1 {
		t.Fatalf("dup unlike should stay 1, got %d", res.LikeCount)
	}
}

func TestLikePrivateBookIs404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私密',0)`)
	repo := NewRepo(d)
	if _, err := repo.Like(1, 12); !errors.Is(err, ErrNotFound) {
		t.Fatalf("like private must be ErrNotFound, got %v", err)
	}
}

func TestRecordViewDedupes(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'书',1,'t')`)
	repo := NewRepo(d)

	if err := repo.RecordView(10, "anon:abc"); err != nil {
		t.Fatalf("view1: %v", err)
	}
	if err := repo.RecordView(10, "anon:abc"); err != nil { // 同 key 去重
		t.Fatalf("view dup: %v", err)
	}
	if err := repo.RecordView(10, "u:1"); err != nil { // 不同 key +1
		t.Fatalf("view2: %v", err)
	}
	var vc int
	d.QueryRow(`SELECT view_count FROM books WHERE id=10`).Scan(&vc)
	if vc != 2 {
		t.Fatalf("unique viewers should be 2, got %d", vc)
	}
	// 非公开：404。
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私',0)`)
	if err := repo.RecordView(12, "anon:x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("view private must be ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/community/ -run 'TestLike|TestRecordView' -v`
Expected: FAIL（undefined: repo.Like）

- [ ] **Step 3: 实现写操作** — `internal/community/repo.go` 追加

```go
// ensurePublic returns ErrNotFound unless bookID exists and is public. Used to
// gate like / view so private books never leak via a 200.
func (r *Repo) ensurePublic(tx *sql.Tx, bookID int64) error {
	var one int
	err := tx.QueryRow(`SELECT 1 FROM books WHERE id = ? AND is_public = 1`, bookID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("ensure public %d: %w", bookID, err)
	}
	return nil
}

// Like records userID's like of a public book (idempotent) and returns the fresh
// count. The like row insert and the counter bump happen in one transaction; the
// counter is only incremented when a new row is actually inserted.
func (r *Repo) Like(userID, bookID int64) (LikeResult, error) {
	return r.toggleLike(userID, bookID, true)
}

// Unlike removes userID's like (idempotent) and returns the fresh count.
func (r *Repo) Unlike(userID, bookID int64) (LikeResult, error) {
	return r.toggleLike(userID, bookID, false)
}

func (r *Repo) toggleLike(userID, bookID int64, like bool) (LikeResult, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return LikeResult{}, fmt.Errorf("like tx: %w", err)
	}
	defer tx.Rollback()

	if err := r.ensurePublic(tx, bookID); err != nil {
		return LikeResult{}, err
	}

	var res sql.Result
	if like {
		res, err = tx.Exec(`INSERT OR IGNORE INTO book_likes (book_id, user_id) VALUES (?, ?)`, bookID, userID)
	} else {
		res, err = tx.Exec(`DELETE FROM book_likes WHERE book_id = ? AND user_id = ?`, bookID, userID)
	}
	if err != nil {
		return LikeResult{}, fmt.Errorf("toggle like: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return LikeResult{}, fmt.Errorf("toggle like rows: %w", err)
	}
	if affected == 1 {
		delta := 1
		if !like {
			delta = -1
		}
		if _, err := tx.Exec(`UPDATE books SET like_count = like_count + ? WHERE id = ?`, delta, bookID); err != nil {
			return LikeResult{}, fmt.Errorf("bump like_count: %w", err)
		}
	}

	var count int
	if err := tx.QueryRow(`SELECT like_count FROM books WHERE id = ?`, bookID).Scan(&count); err != nil {
		return LikeResult{}, fmt.Errorf("read like_count: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return LikeResult{}, fmt.Errorf("like commit: %w", err)
	}
	return LikeResult{LikeCount: count, Liked: like}, nil
}

// RecordView records a unique view of a public book keyed by viewerKey. Repeat
// views with the same key are ignored; view_count is bumped only on first sight.
func (r *Repo) RecordView(bookID int64, viewerKey string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("view tx: %w", err)
	}
	defer tx.Rollback()

	if err := r.ensurePublic(tx, bookID); err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT OR IGNORE INTO book_views (book_id, viewer_key) VALUES (?, ?)`, bookID, viewerKey)
	if err != nil {
		return fmt.Errorf("insert view: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("view rows: %w", err)
	}
	if affected == 1 {
		if _, err := tx.Exec(`UPDATE books SET view_count = view_count + 1 WHERE id = ?`, bookID); err != nil {
			return fmt.Errorf("bump view_count: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("view commit: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/community/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/community/repo.go internal/community/repo_write_test.go
git commit -m "feat: community 点赞/取消赞/记浏览(事务+反范式计数幂等)"
```

---

## Task 6: community service + handler + 路由接入

**Files:**
- Create: `internal/community/service.go`
- Create: `internal/community/handler.go`
- Modify: `internal/httpx/router.go`
- Modify: `cmd/server/main.go`
- Test: `internal/community/handler_test.go`

**Interfaces:**
- Consumes: `Repo`（Task 4/5）、`auth.OptionalUser`/`auth.RequireUser`/`auth.UserID`/`auth.Session`。
- Produces:
  - `func NewService(repo *Repo) *Service`；方法 `ListPublic(viewerKey string, limit, offset int)`（夹取 limit∈[1,50] 缺省 20、offset≥0）/ `GetPublicDetail` / `Like` / `Unlike` / `RecordView`。
  - `func NewHandler(svc *Service) *Handler`；`func (h *Handler) MountPublic(r chi.Router)`（feed/detail/view）；`func (h *Handler) MountAuthed(r chi.Router)`（like/unlike）。
  - `Deps` 增 `Community *community.Handler`；router 新增两组。

- [ ] **Step 1: 实现 service** — `internal/community/service.go`

```go
package community

const (
	defaultLimit = 20
	maxLimit     = 50
)

// Service applies input clamping at the boundary and delegates to Repo.
type Service struct {
	repo *Repo
}

// NewService wires a Service to its Repo.
func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// clampPaging normalizes caller paging into safe bounds (never errors).
func clampPaging(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ListPublic returns the public feed, newest first.
func (s *Service) ListPublic(viewerKey string, limit, offset int) ([]CommunityBook, error) {
	limit, offset = clampPaging(limit, offset)
	return s.repo.ListPublic(viewerKey, limit, offset)
}

// GetPublicDetail returns one public book's full read payload, or ErrNotFound.
func (s *Service) GetPublicDetail(viewerKey string, bookID int64) (CommunityBookDetail, error) {
	return s.repo.GetPublicDetail(viewerKey, bookID)
}

// Like / Unlike / RecordView delegate to the repo.
func (s *Service) Like(userID, bookID int64) (LikeResult, error)   { return s.repo.Like(userID, bookID) }
func (s *Service) Unlike(userID, bookID int64) (LikeResult, error) { return s.repo.Unlike(userID, bookID) }
func (s *Service) RecordView(bookID int64, viewerKey string) error {
	return s.repo.RecordView(bookID, viewerKey)
}
```

- [ ] **Step 2: 实现 handler** — `internal/community/handler.go`

```go
package community

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// Handler serves the community HTTP endpoints on top of a Service.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// MountPublic registers the public (OptionalUser) endpoints:
//
//	GET  /community/books
//	GET  /community/books/{id}
//	POST /community/books/{id}/view
func (h *Handler) MountPublic(r chi.Router) {
	r.Get("/community/books", h.List)
	r.Get("/community/books/{id}", h.Detail)
	r.Post("/community/books/{id}/view", h.RecordView)
}

// MountAuthed registers the authenticated endpoints (behind RequireUser):
//
//	POST   /community/books/{id}/like
//	DELETE /community/books/{id}/like
func (h *Handler) MountAuthed(r chi.Router) {
	r.Post("/community/books/{id}/like", h.Like)
	r.Delete("/community/books/{id}/like", h.Unlike)
}

// viewerKey builds the like/view identity from the (optional) authenticated user
// and a client-provided anonymous id. Logged-in users key by user id; anonymous
// users key by "anon:{clientId}"; a blank client id falls back to "anon:".
func viewerKeyFor(userID int64, clientID string) string {
	if userID != 0 {
		return "u:" + strconv.FormatInt(userID, 10)
	}
	return "anon:" + clientID
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	vk := ""
	if userID != 0 {
		vk = "u:" + strconv.FormatInt(userID, 10)
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := h.svc.ListPublic(vk, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取社区列表失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	vk := ""
	if userID != 0 {
		vk = "u:" + strconv.FormatInt(userID, 10)
	}
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.GetPublicDetail(vk, bookID)
	if err != nil {
		writeCommunityError(w, err, "获取内容失败")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type viewInput struct {
	ClientID string `json:"clientId"`
}

func (h *Handler) RecordView(w http.ResponseWriter, r *http.Request) {
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	var in viewInput
	// body 可选：空 body 允许（匿名无 clientId 时退化为 "anon:"）。
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	vk := viewerKeyFor(auth.UserID(r.Context()), in.ClientID)
	if err := h.svc.RecordView(bookID, vk); err != nil {
		writeCommunityError(w, err, "记录浏览失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request)   { h.like(w, r, true) }
func (h *Handler) Unlike(w http.ResponseWriter, r *http.Request) { h.like(w, r, false) }

func (h *Handler) like(w http.ResponseWriter, r *http.Request, like bool) {
	userID := auth.UserID(r.Context())
	bookID, ok := parseID(w, r)
	if !ok {
		return
	}
	var res LikeResult
	var err error
	if like {
		res, err = h.svc.Like(userID, bookID)
	} else {
		res, err = h.svc.Unlike(userID, bookID)
	}
	if err != nil {
		writeCommunityError(w, err, "操作失败")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// parseID reads the {id} path param; on a bad id writes 400 and returns ok=false.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return id, true
}

func writeCommunityError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "找不到这个内容")
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
```

- [ ] **Step 3: 写 handler 集成测试** — `internal/community/handler_test.go`

```go
package community

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/db"
)

// mount builds a router: public group with a middleware that injects userID when
// the test asks (uid!=0), plus an authed group that always injects uid.
func mountRouter(h *Handler, uid int64) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Group(func(pub chi.Router) {
			pub.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if uid != 0 {
						req = req.WithContext(auth.WithUserID(req.Context(), uid))
					}
					next.ServeHTTP(w, req)
				})
			})
			h.MountPublic(pub)
		})
		v1.Group(func(pr chi.Router) {
			pr.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(w, req.WithContext(auth.WithUserID(req.Context(), uid)))
				})
			})
			h.MountAuthed(pr)
		})
	})
	return r
}

func TestAnonymousCanListAndPrivateDetail404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'公开',1,'t'),(12,1,'私密',0,'')`)
	h := NewHandler(NewService(NewRepo(d)))
	srv := mountRouter(h, 0) // 匿名

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/community/books", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "公开") || strings.Contains(rec.Body.String(), "私密") {
		t.Fatalf("feed wrong: code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/community/books/12", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("private detail should 404, got %d", rec.Code)
	}
}

func TestLikeFlow(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'公开',1,'t')`)
	h := NewHandler(NewService(NewRepo(d)))
	srv := mountRouter(h, 1)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/community/books/10/like", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"likeCount":1`) {
		t.Fatalf("like: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/community/ -v`
Expected: PASS

- [ ] **Step 5: 接入 router** — 编辑 `internal/httpx/router.go`

import 加 `"github.com/seven-agents/oh-my-commic/internal/community"`。`Deps` 增字段：

```go
	// Community mounts the public community feed / reader routes (via OptionalUser)
	// and the authenticated like routes (via RequireUser). Optional: nil disables.
	Community *community.Handler
```

在 `r.Route("/api/v1", ...)` 内：**公开组**（在现有 public auth routes 之后、Protected 组之前）用 OptionalUser 包一层：

```go
		// Public community routes: readable without login, but OptionalUser lets a
		// logged-in visitor get a personalized "liked" flag and a stable view key.
		if deps.Community != nil {
			v1.Group(func(pub chi.Router) {
				pub.Use(auth.OptionalUser(deps.Session))
				deps.Community.MountPublic(pub)
			})
		}
```

在现有 Protected 组（`pr.Use(auth.RequireUser(...))`）内追加：

```go
			if deps.Community != nil {
				deps.Community.MountAuthed(pr)
			}
```

- [ ] **Step 6: 接线 main** — 编辑 `cmd/server/main.go`，在构造 book handler 附近加：

```go
	communityRepo := community.NewRepo(database)
	communityHandler := community.NewHandler(community.NewService(communityRepo))
```

（`database` 用该文件里已有的 `*sql.DB` 变量名为准）并在 `httpx.Deps{...}` 里加 `Community: communityHandler,`。import 加 community 包。

- [ ] **Step 7: 全量构建 + 测试**

Run: `go build ./... && go test ./internal/community/ ./internal/httpx/ -v && go vet ./...`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/community/ internal/httpx/router.go cmd/server/main.go
git commit -m "feat: community service/handler + 路由接入(公开组OptionalUser/点赞RequireUser)"
```

---

## Task 7: OpenAPI 契约 + 契约 E2E

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `test/contract/contract_test.go`

**Interfaces:**
- Consumes: 上述所有端点的真实响应形状。
- Produces: openapi 里 6 个新端点 + schema（`CommunityBook`/`CommunityBookDetail`/`ReaderChapter`/`Author`/`LikeResult`/`VisibilityInput`/`ViewInput`）；契约测试对每个新端点 `ValidateResponse`。

- [ ] **Step 1: 读现有 openapi 结构** — 打开 `docs/openapi.yaml`，定位 `paths:` 与 `components.schemas:`，比对已有 `Book`/`Panel`/`Chapter` schema 命名与风格（`additionalProperties: false`、`required` 用法）。

- [ ] **Step 2: 加 schemas** — 在 `components.schemas` 追加（字段与 Task 4 类型 JSON tag 完全一致；`Panel`/若已存在则 `$ref` 复用）：

```yaml
    Author:
      type: object
      additionalProperties: false
      required: [nickname, avatarUrl]
      properties:
        nickname: { type: string }
        avatarUrl: { type: string }
    CommunityBook:
      type: object
      additionalProperties: false
      required: [id, title, coverUrl, summary, author, likeCount, viewCount, liked, publishedAt]
      properties:
        id: { type: integer, format: int64 }
        title: { type: string }
        coverUrl: { type: string }
        summary: { type: string }
        author: { $ref: '#/components/schemas/Author' }
        likeCount: { type: integer }
        viewCount: { type: integer }
        liked: { type: boolean }
        publishedAt: { type: string }
    ReaderChapter:
      type: object
      additionalProperties: false
      required: [title, summary, order, isCover, panels]
      properties:
        title: { type: string }
        summary: { type: string }
        order: { type: integer }
        isCover: { type: boolean }
        panels:
          type: array
          items: { $ref: '#/components/schemas/Panel' }
    CommunityBookDetail:
      type: object
      additionalProperties: false
      required: [id, title, coverUrl, summary, style, author, likeCount, viewCount, liked, chapters]
      properties:
        id: { type: integer, format: int64 }
        title: { type: string }
        coverUrl: { type: string }
        summary: { type: string }
        style: { type: string }
        author: { $ref: '#/components/schemas/Author' }
        likeCount: { type: integer }
        viewCount: { type: integer }
        liked: { type: boolean }
        chapters:
          type: array
          items: { $ref: '#/components/schemas/ReaderChapter' }
    LikeResult:
      type: object
      additionalProperties: false
      required: [likeCount, liked]
      properties:
        likeCount: { type: integer }
        liked: { type: boolean }
    VisibilityInput:
      type: object
      additionalProperties: false
      required: [isPublic]
      properties:
        isPublic: { type: boolean }
    ViewInput:
      type: object
      additionalProperties: false
      properties:
        clientId: { type: string }
```

> 若现有 `Panel` schema 有 `additionalProperties: false` 且要求全部字段，而 community 返回的 Panel 只填部分字段（其余零值仍会序列化，故 JSON 里字段齐全）——零值字段仍出现在 JSON 中，schema 校验通过。确认 `Panel` schema 的 `required` 不包含 community 未返回、且 Go 端 `omitempty` 会省略的字段；`models.Panel` 无 `omitempty`，所有字段恒序列化，安全。

- [ ] **Step 3: 加 paths** — 在 `paths:` 追加 6 条（`GET/POST/DELETE`，响应引用上面 schema，错误用现有 `Error`）：`/community/books`、`/community/books/{id}`、`/community/books/{id}/view`、`/community/books/{id}/like`（post+delete）、`/books/{id}/visibility`（put）。每条含 200 及相应 400/401/404。cookie 安全方案沿用现有 `cookieAuth`（like/visibility 标注需要，公开三条不标注或标注 optional）。

- [ ] **Step 4: 加契约测试** — 打开 `test/contract/contract_test.go`，仿照既有用例（构造真实请求→拿响应→`router.FindRoute`+`ValidateResponse`）为每个新端点各加一个子测试：匿名 `GET /community/books`、匿名 `GET /community/books/{id}`（播种一本公开书）、`POST .../view`、登录 `POST/DELETE .../like`、`PUT /books/{id}/visibility`。播种数据不触发任何 AI。

- [ ] **Step 5: 跑契约测试**

Run: `go test ./test/contract/ -v`
Expected: PASS（每个新端点 ValidateResponse 通过）

- [ ] **Step 6: 负向自检** — 临时给 `CommunityBook` 加一个 `required: [bogus]` 字段，跑测试应变 RED，确认契约真的在校验；随后还原。

- [ ] **Step 7: 提交**

```bash
git add docs/openapi.yaml test/contract/contract_test.go
git commit -m "docs: openapi 补社区端点+schema 并加契约 E2E"
```

---

## Task 8: 前端 API 客户端 + 类型 + clientId

**Files:**
- Create: `web/src/lib/clientId.ts`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`
- Test: `web/src/lib/clientId.test.ts`

**Interfaces:**
- Produces: `getClientId(): string`；类型 `Author`/`CommunityBook`/`ReaderChapter`/`CommunityBookDetail`/`LikeResult`；`api.listCommunity`/`getCommunityBook`/`recordView`/`likeBook`/`unlikeBook`/`setVisibility`。

- [ ] **Step 1: 写 clientId 失败测试** — `web/src/lib/clientId.test.ts`

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { getClientId } from './clientId'

describe('getClientId', () => {
  beforeEach(() => localStorage.clear())

  it('生成并持久化一个稳定 id', () => {
    const a = getClientId()
    expect(a).toBeTruthy()
    const b = getClientId()
    expect(b).toBe(a) // 二次调用返回同一个
    expect(localStorage.getItem('omc.clientId')).toBe(a)
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/lib/clientId.test.ts`
Expected: FAIL（Cannot find module './clientId'）

- [ ] **Step 3: 实现 clientId** — `web/src/lib/clientId.ts`

```ts
const KEY = 'omc.clientId'

// getClientId 返回一个持久化在 localStorage 的匿名访客 id，用于社区浏览量去重。
// 首次调用生成随机 UUID 并落地；后续调用返回同一个。清除缓存会重置（本期可接受）。
export function getClientId(): string {
  try {
    const existing = localStorage.getItem(KEY)
    if (existing) return existing
    const id =
      typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : Math.random().toString(36).slice(2) + Date.now().toString(36)
    localStorage.setItem(KEY, id)
    return id
  } catch {
    // 隐私模式等：退化为进程内随机（不持久，但功能不阻断）。
    return Math.random().toString(36).slice(2)
  }
}
```

- [ ] **Step 4: 加类型** — `web/src/api/types.ts` 追加（与后端 JSON 完全一致）

```ts
export interface Author {
  nickname: string
  avatarUrl: string
}

export interface CommunityBook {
  id: number
  title: string
  coverUrl: string
  summary: string
  author: Author
  likeCount: number
  viewCount: number
  liked: boolean
  publishedAt: string
}

export interface ReaderChapter {
  title: string
  summary: string
  order: number
  isCover: boolean
  panels: Panel[]
}

export interface CommunityBookDetail {
  id: number
  title: string
  coverUrl: string
  summary: string
  style: string
  author: Author
  likeCount: number
  viewCount: number
  liked: boolean
  chapters: ReaderChapter[]
}

export interface LikeResult {
  likeCount: number
  liked: boolean
}
```

> `Panel` 类型已存在于 types.ts；`ReaderChapter.panels` 复用它（community 返回的 Panel 仅部分字段非零，其余为该类型字段的零值/空串，前端只读 imageUrl/caption/order/status，安全）。

- [ ] **Step 5: 加 client 方法** — `web/src/api/client.ts` 的 `api` 对象追加

```ts
  // 社区公开 feed（分页）。匿名可调。
  listCommunity: (limit = 20, offset = 0) =>
    request<CommunityBook[]>('GET', `/api/v1/community/books?limit=${limit}&offset=${offset}`),

  // 公开阅读详情（book+chapters+panels）。匿名可调；非公开/不存在抛 404。
  getCommunityBook: (id: number) =>
    request<CommunityBookDetail>('GET', `/api/v1/community/books/${id}`),

  // 记一次独立浏览（匿名带 clientId 去重）。
  recordView: (id: number, clientId: string) =>
    request<{ ok: boolean }>('POST', `/api/v1/community/books/${id}/view`, { clientId }),

  // 点赞 / 取消赞（需登录）。
  likeBook: (id: number) =>
    request<LikeResult>('POST', `/api/v1/community/books/${id}/like`),
  unlikeBook: (id: number) =>
    request<LikeResult>('DELETE', `/api/v1/community/books/${id}/like`),

  // owner 发布/下架整本书。
  setVisibility: (id: number, isPublic: boolean) =>
    request<Book>('PUT', `/api/v1/books/${id}/visibility`, { isPublic }),
```

文件顶部 import 补上新类型：`import type { User, Book, CommunityBook, CommunityBookDetail, LikeResult } from './types'`（保留原有 `User`；`Book` 若未 import 则加）。

- [ ] **Step 6: 跑测试 + 构建**

Run: `cd web && npx vitest run src/lib/clientId.test.ts && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 7: 提交**

```bash
git add web/src/lib/clientId.ts web/src/lib/clientId.test.ts web/src/api/types.ts web/src/api/client.ts
git commit -m "feat(web): 社区 API 客户端/类型 + 匿名 clientId"
```

---

## Task 9: 抽出 BookReaderView（DRY 阅读器展示层）

**Files:**
- Create: `web/src/components/reader/BookReaderView.tsx`
- Modify: `web/src/pages/BookReader.tsx`
- Test: `web/src/components/reader/BookReaderView.test.tsx`

**Interfaces:**
- Consumes: `ReaderPageData`/`ReaderPage`（已有）。
- Produces: `BookReaderView`，props：

```ts
interface BookReaderViewProps {
  loading: boolean
  error: string
  title: string
  coverPage: ReaderPageData | null   // kind:'cover'
  chapterPages: ReaderPageData[]     // kind:'chapter'
  headerRight?: ReactNode            // 头部右侧插槽（返回/编辑/点赞等）
  footer?: ReactNode                 // 底部插槽（点赞栏等）
}
```
  展示层负责：翻页 state、键盘左右、封面+章节页拼装渲染、loading/error/空态。**不**做数据获取。

- [ ] **Step 1: 写失败测试** — `web/src/components/reader/BookReaderView.test.tsx`

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { BookReaderView } from './BookReaderView'
import type { ReaderPageData } from './ReaderPage'

const cover: ReaderPageData = { kind: 'cover', bookId: 1, title: '我的书', coverUrl: '', summary: '梗概' }

describe('BookReaderView', () => {
  it('渲染封面标题与页码', () => {
    render(
      <MemoryRouter>
        <BookReaderView loading={false} error="" title="我的书" coverPage={cover} chapterPages={[]} />
      </MemoryRouter>,
    )
    expect(screen.getByText('我的书')).toBeInTheDocument()
    expect(screen.getByText(/第 1 \/ 1 页/)).toBeInTheDocument()
  })

  it('loading 显示加载态', () => {
    render(
      <MemoryRouter>
        <BookReaderView loading error="" title="" coverPage={null} chapterPages={[]} />
      </MemoryRouter>,
    )
    expect(screen.queryByText('我的书')).not.toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/reader/BookReaderView.test.tsx`
Expected: FAIL（Cannot find module './BookReaderView'）

- [ ] **Step 3: 实现 BookReaderView** — 把 `BookReader.tsx` 里的「翻页 state + 键盘监听 + pages 拼装 + loading/error/空态 + FlipButton」整段搬进 `BookReaderView.tsx`，改为吃 props（`coverPage`/`chapterPages`/`headerRight`/`footer`/`loading`/`error`/`title`）。`FlipButton` 一并移入本文件。`pages = coverPage ? [coverPage, ...chapterPages] : chapterPages`。`hasReadablePages = chapterPages.length > 0`。头部用传入的 `headerRight`，底部 `pages[index].kind==='chapter'` 之外也照常渲染 `footer`（若提供）。空态/错误文案沿用原文案。

- [ ] **Step 4: 改 BookReader 用 view** — `web/src/pages/BookReader.tsx` 只保留数据获取（3 个 fetch + 组装 `chapterPages`），把渲染交给 `BookReaderView`，`headerRight` 传原来的「← 返回书架 / ✏️ 去编辑」两个按钮，`title={book?.title ?? ''}`，`coverPage` 由 book 组装。行为与视觉保持不变。

- [ ] **Step 5: 跑测试 + 构建**

Run: `cd web && npx vitest run src/components/reader/ && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 6: 提交**

```bash
git add web/src/components/reader/BookReaderView.tsx web/src/components/reader/BookReaderView.test.tsx web/src/pages/BookReader.tsx
git commit -m "refactor(web): 抽出 BookReaderView 展示层(owner 阅读复用)"
```

---

## Task 10: Home 落地页 + 路由改造 + 登录跳转

**Files:**
- Create: `web/src/pages/Home.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/pages/Login.tsx`（或 useAuth 登录后跳转处，把落点从 `/` 改为 `/my`）
- Test: `web/src/pages/Home.test.tsx`

**Interfaces:**
- Consumes: `useAuth`（登录态）、react-router `Link`。
- Produces: 路由 `/`→Home（公开）、`/my`→Bookshelf（受保护）、`/community`→Community（公开，Task 11）、`/community/books/:id`→CommunityReader（公开，Task 12）。

- [ ] **Step 1: 写 Home 失败测试** — `web/src/pages/Home.test.tsx`

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Home from './Home'
import { AuthProvider } from '../auth/useAuth'

describe('Home', () => {
  it('展示社区与我的漫画两个入口', () => {
    render(
      <MemoryRouter>
        <AuthProvider>
          <Home />
        </AuthProvider>
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: /社区/ })).toHaveAttribute('href', '/community')
    expect(screen.getByRole('link', { name: /我的漫画/ })).toHaveAttribute('href', '/my')
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/pages/Home.test.tsx`
Expected: FAIL（Cannot find module './Home'）

- [ ] **Step 3: 实现 Home** — `web/src/pages/Home.tsx`：`AppHeader` + 两张大入口卡：`<Link to="/community">` 社区（副标题「看看大家的漫画」）、`<Link to="/my">` 我的漫画（副标题「创作与管理我的书」）。未登录时「我的漫画」卡仍可点（点进 `/my` → `RequireAuth` 会引导登录）。用现有 `Card`/`Button`/排版风格，保持吉卜力暖色调。

- [ ] **Step 4: 改路由** — `web/src/App.tsx`：
  - 新增 `import Home from './pages/Home'`、`import Community from './pages/Community'`、`import CommunityReader from './pages/CommunityReader'`（Task 11/12 会创建；本任务先建 Home，Community/CommunityReader 的 import 与 route 可先指向占位或在其任务完成后补齐——**为避免构建断裂，本步只加 Home 与 `/my`，Community/CommunityReader 两条路由留到各自任务加**）。
  - `<Route path="/" ...>` 改为 `element={<Home />}`（公开，去掉 Protected）。
  - 新增 `<Route path="/my" element={<Protected><Bookshelf /></Protected>} />`。

- [ ] **Step 5: 改登录后跳转** — 找到登录/注册成功后 `navigate('/')` 的位置（`web/src/pages/Login.tsx`），改为 `navigate('/my')`。同理若 BookReader/其它页有「返回书架」`to="/"` 想指向书架，改为 `to="/my"`（Task 9 的 headerRight 若含返回书架，一并改 `/my`）。

- [ ] **Step 6: 跑测试 + 构建**

Run: `cd web && npx vitest run src/pages/Home.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 7: 提交**

```bash
git add web/src/pages/Home.tsx web/src/pages/Home.test.tsx web/src/App.tsx web/src/pages/Login.tsx web/src/pages/BookReader.tsx
git commit -m "feat(web): 公开 Home 两入口 + 书架迁到 /my + 登录跳转 /my"
```

---

## Task 11: 社区 feed 页 + CommunityCard

**Files:**
- Create: `web/src/components/CommunityCard.tsx`
- Create: `web/src/pages/Community.tsx`
- Modify: `web/src/App.tsx`（加 `/community` 路由）
- Test: `web/src/components/CommunityCard.test.tsx`

**Interfaces:**
- Consumes: `api.listCommunity`、类型 `CommunityBook`、`mediaUrl`。
- Produces: `Community` 页（网格 + 加载更多）、`CommunityCard`（封面/标题/summary/作者昵称+头像/❤like/👁view）。

- [ ] **Step 1: 写 CommunityCard 失败测试** — `web/src/components/CommunityCard.test.tsx`

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { CommunityCard } from './CommunityCard'
import type { CommunityBook } from '../api/types'

const book: CommunityBook = {
  id: 5, title: '小熊的一天', coverUrl: '', summary: '温暖的故事',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: 3, viewCount: 12, liked: false, publishedAt: 't',
}

describe('CommunityCard', () => {
  it('展示标题/概述/作者/点赞与浏览数，链接到阅读页', () => {
    render(<MemoryRouter><CommunityCard book={book} /></MemoryRouter>)
    expect(screen.getByText('小熊的一天')).toBeInTheDocument()
    expect(screen.getByText('温暖的故事')).toBeInTheDocument()
    expect(screen.getByText('小明')).toBeInTheDocument()
    expect(screen.getByText(/3/)).toBeInTheDocument()
    expect(screen.getByText(/12/)).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/community/books/5')
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/CommunityCard.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现 CommunityCard** — `web/src/components/CommunityCard.tsx`：一个 `<Link to={/community/books/${book.id}}>` 卡片，封面用 `mediaUrl(book.coverUrl)`（空则占位），标题、`summary`（截断）、作者行（`avatarUrl` 有则圆头像否则昵称首字）、底部 `❤ {likeCount}`、`👁 {viewCount}`。风格沿用现有卡片。

- [ ] **Step 4: 实现 Community 页** — `web/src/pages/Community.tsx`：`AppHeader` + 标题「社区」+ 网格。`useEffect` 首屏 `api.listCommunity(20,0)`；state 保存 `items`/`loading`/`error`/`offset`/`done`。「加载更多」按钮再拉 `listCommunity(20, offset)` 追加（返回不足 20 则 `done=true`）。空态「还没有公开的漫画，快去发布第一本吧～」。错误用 `errorMessage`。

- [ ] **Step 5: 加路由** — `web/src/App.tsx` 加 `<Route path="/community" element={<Community />} />`（公开），并补 `import Community from './pages/Community'`（若 Task 10 未加）。

- [ ] **Step 6: 跑测试 + 构建**

Run: `cd web && npx vitest run src/components/CommunityCard.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 7: 提交**

```bash
git add web/src/components/CommunityCard.tsx web/src/components/CommunityCard.test.tsx web/src/pages/Community.tsx web/src/App.tsx
git commit -m "feat(web): 社区 feed 页 + CommunityCard"
```

---

## Task 12: 公开阅读器容器（记浏览 + 点赞）

**Files:**
- Create: `web/src/pages/CommunityReader.tsx`
- Modify: `web/src/App.tsx`（加 `/community/books/:id` 路由）
- Test: `web/src/pages/CommunityReader.test.tsx`（mock api）

**Interfaces:**
- Consumes: `api.getCommunityBook`/`recordView`/`likeBook`/`unlikeBook`、`getClientId`、`useAuth`、`BookReaderView`、`ReaderPageData`。
- Produces: `/community/books/:id` 页；挂载时拉详情 + 记一次 view；点赞栏（未登录点 → `navigate('/login')`）。

- [ ] **Step 1: 写失败测试（mock api）** — `web/src/pages/CommunityReader.test.tsx`

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: {
    getCommunityBook: vi.fn().mockResolvedValue({
      id: 5, title: '小熊的一天', coverUrl: '', summary: '温暖', style: 'ghibli',
      author: { nickname: '小明', avatarUrl: '' },
      likeCount: 2, viewCount: 9, liked: false, chapters: [],
    }),
    recordView: vi.fn().mockResolvedValue({ ok: true }),
    likeBook: vi.fn(), unlikeBook: vi.fn(),
  },
}))
import { api } from '../api/client'
import CommunityReader from './CommunityReader'
import { AuthProvider } from '../auth/useAuth'

describe('CommunityReader', () => {
  beforeEach(() => vi.clearAllMocks())

  it('加载详情并记一次浏览', async () => {
    render(
      <MemoryRouter initialEntries={['/community/books/5']}>
        <AuthProvider>
          <Routes>
            <Route path="/community/books/:id" element={<CommunityReader />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('小熊的一天')).toBeInTheDocument())
    expect(api.getCommunityBook).toHaveBeenCalledWith(5)
    expect(api.recordView).toHaveBeenCalledTimes(1)
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/pages/CommunityReader.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现 CommunityReader** — `web/src/pages/CommunityReader.tsx`：
  - `useParams` 取 id；`useEffect`（依赖 id）：`getCommunityBook(id)` 填 detail；成功后 `recordView(id, getClientId())`（失败静默）。用 `useRef` 守卫避免 StrictMode 双触发重复记 view。
  - 把 `detail.chapters` 用与 BookReader 同样的规则组装 `chapterPages`：过滤 `isCover`、只保留有已渲染分镜（`panels.some(p=>p.status==='done'&&p.imageUrl)`）的章，映射成 `{kind:'chapter', chapterTitle: ch.title, panels: ch.panels, summary: ch.summary}`；`coverPage` 由 detail 组装（`{kind:'cover', bookId: detail.id, title, coverUrl, summary}`）。
  - 渲染 `<BookReaderView loading error title={detail?.title} coverPage chapterPages headerRight={<Link to="/community">← 社区</Link>} footer={<LikeBar .../>} />`。
  - `LikeBar`：显示 `❤ {likeCount}`，登录用户点击切换 `likeBook/unlikeBook` 并更新本地 `liked`/`likeCount`；未登录点击 `navigate('/login')`。
- [ ] **Step 4: 加路由** — `web/src/App.tsx` 加 `<Route path="/community/books/:id" element={<CommunityReader />} />`（公开）+ import。

- [ ] **Step 5: 跑测试 + 构建**

Run: `cd web && npx vitest run src/pages/CommunityReader.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 6: 提交**

```bash
git add web/src/pages/CommunityReader.tsx web/src/pages/CommunityReader.test.tsx web/src/App.tsx
git commit -m "feat(web): 公开阅读器(记独立浏览+点赞栏,复用 BookReaderView)"
```

---

## Task 13: 书架发布开关 + 计数展示

**Files:**
- Modify: `web/src/pages/Bookshelf.tsx`
- Test: `web/src/pages/Bookshelf.test.tsx`（若已存在则追加；否则新建针对开关的用例，mock api）

**Interfaces:**
- Consumes: `api.setVisibility`、类型 `Book`（含 `isPublic`/`likeCount`/`viewCount`）。
- Produces: 每张书卡一个「公开/私密」开关，切换调 `setVisibility`；公开时展示 `❤`/`👁` 计数与「公开」徽标。

- [ ] **Step 1: 写失败测试（mock api）** — `web/src/pages/Bookshelf.test.tsx`

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: {
    get: vi.fn().mockResolvedValue([
      { id: 1, userId: 1, title: '书A', coverUrl: '', style: 'ghibli', summary: '',
        isPublic: false, createdAt: 't', updatedAt: 't', likeCount: 0, viewCount: 0, publishedAt: '' },
    ]),
    setVisibility: vi.fn().mockResolvedValue({
      id: 1, userId: 1, title: '书A', coverUrl: '', style: 'ghibli', summary: '',
      isPublic: true, createdAt: 't', updatedAt: 't', likeCount: 0, viewCount: 0, publishedAt: 't2',
    }),
  },
}))
import { api } from '../api/client'
import Bookshelf from './Bookshelf'
import { AuthProvider } from '../auth/useAuth'

describe('Bookshelf 公开开关', () => {
  beforeEach(() => vi.clearAllMocks())

  it('点击开关调用 setVisibility', async () => {
    render(<MemoryRouter><AuthProvider><Bookshelf /></AuthProvider></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('书A')).toBeInTheDocument())
    const toggle = screen.getByRole('button', { name: /公开|发布/ })
    fireEvent.click(toggle)
    await waitFor(() => expect(api.setVisibility).toHaveBeenCalledWith(1, true))
  })
})
```

> 若现有 Bookshelf 加载书籍不是走 `api.get('/api/v1/books')`，按其真实调用调整 mock 的方法名/返回；用例目标是「切换按钮 → 调 setVisibility(id, true)」。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/pages/Bookshelf.test.tsx`
Expected: FAIL（无匹配按钮）

- [ ] **Step 3: 实现开关** — `web/src/pages/Bookshelf.tsx`：每张书卡加一个按钮/开关，label 依 `book.isPublic` 显示「公开中」/「设为公开」。点击调 `api.setVisibility(book.id, !book.isPublic)`，用返回的 book 就地替换列表中该项（不可变更新 `setBooks(prev => prev.map(b => b.id===id?updated:b))`）。用 `useSubmitOnce` 或 `useRef` 守卫防双击。公开的书展示 `❤{likeCount}` `👁{viewCount}` 与「公开」小徽标。错误用现有错误展示。

- [ ] **Step 4: 跑测试 + 构建**

Run: `cd web && npx vitest run src/pages/Bookshelf.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 5: 提交**

```bash
git add web/src/pages/Bookshelf.tsx web/src/pages/Bookshelf.test.tsx
git commit -m "feat(web): 书架公开/私密开关 + 点赞浏览计数展示"
```

---

## Task 14: Playwright E2E（匿名浏览 + owner 发布，AI-free）

**Files:**
- Create: `web/e2e/community.spec.ts`
- Modify: `.github/workflows/e2e.yml`（如需补 env；沿用既有 INVITE_CODE/ADMIN_* 注入）

**Interfaces:**
- Consumes: 运行中的服务（e2e 既有启动方式）、邀请码注册流程（已有）。
- Produces: 两条 E2E：匿名访问 `/community`；owner 登录后在 `/my` 发布一本书使其出现在 `/community`。

- [ ] **Step 1: 写 E2E** — `web/e2e/community.spec.ts`：
  - 用例 A（匿名）：直接 `page.goto('/community')`，断言页面标题「社区」渲染、无重定向到 `/login`（未登录也能进）。若无种子公开书则断言空态文案；本用例主要验证「匿名可访问社区路由」。
  - 用例 B（owner 发布可见）：注册并登录一个新用户（复用 `register.spec.ts` 的邀请码流程 helper）→ 创建一本书（走既有创建流程；**不触发 AI**，仅建书 + 设标题）→ 打开 `/my` → 点该书「设为公开」→ `page.goto('/community')` → 断言能看到该书标题。
  - 若创建含封面/分镜需要 AI，则改为：直接在 DB 播种一本公开书不可行（E2E 不接触 DB），故用例 B 只验证「发布开关调用后书卡出现在社区列表」——列表项即使无封面也应渲染标题，断言标题可见即可。

- [ ] **Step 2: 本地跑 E2E（需本地起服务）**

Run: `cd web && npm run build && npx playwright test community.spec.ts`
Expected: PASS（如本地未装浏览器先 `npx playwright install`）

- [ ] **Step 3: 确认 CI 覆盖** — 检查 `.github/workflows/e2e.yml`：新 spec 会被既有 `playwright test` 全量跑；确认启动服务的 env 含 `INVITE_CODE`（注册用）。无需改动则跳过。

- [ ] **Step 4: 提交**

```bash
git add web/e2e/community.spec.ts .github/workflows/e2e.yml
git commit -m "test(web): 社区匿名浏览 + owner 发布可见 E2E"
```

---

## Task 15: 文档同步（CLAUDE.md / frontend-api.md / ARCHITECTURE-AND-PROMPTS.md）

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/frontend-api.md`
- Modify: `docs/ARCHITECTURE-AND-PROMPTS.md`

**Interfaces:**
- Consumes: 最终实现的端点/模块/路由。
- Produces: 三处文档与实现一致。

- [ ] **Step 1: 更新 CLAUDE.md** — 在「架构 / 模块」加 `community` 包一行职责；「核心约定」补：社区**公开只读**端点严格 `is_public` 过滤 + 404、作者仅暴露 nickname/avatar、公开阅读详情不含 conversation/panelCount；「关键数据模型」`Book` 补 `like_count/view_count/published_at` 与 `book_likes/book_views` 两表；记录前端路由变化（`/`=公开 Home，书架在 `/my`）。

- [ ] **Step 2: 更新 frontend-api.md** — 概览补 6 个社区端点（方法/路径/鉴权/用途），注明以 `openapi.yaml` 为准。

- [ ] **Step 3: 更新 ARCHITECTURE-AND-PROMPTS.md** — 「模块地图」后端加 `community` 行、前端加 `pages/Home`、`pages/Community`、`pages/CommunityReader`、`components/CommunityCard`、`components/reader/BookReaderView` 行。

- [ ] **Step 4: 无代码测试，构建校验** — 无。

- [ ] **Step 5: 提交**

```bash
git add CLAUDE.md docs/frontend-api.md docs/ARCHITECTURE-AND-PROMPTS.md
git commit -m "docs: 社区模块写入 CLAUDE.md/frontend-api/架构文档"
```

---

## 收尾（执行完所有任务后）

- [ ] 全量后端：`CGO_ENABLED=1 go test -race ./... && go vet ./...`
- [ ] 全量前端：`cd web && npm run test && npm run build`
- [ ] `feat/community` `--no-ff` 合并回 `main`，删分支。
