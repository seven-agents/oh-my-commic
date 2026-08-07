package asset

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/comicify"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// stubEditor is an ImageEditor that returns a URL to an in-test image server (so
// the comicify download step succeeds), or an error to exercise the 502 path.
type stubEditor struct {
	remoteURL string
	err       error
	calls     int
}

func (s *stubEditor) EditImage(_ context.Context, _ string, _ []string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.remoteURL, nil
}

// tinyPNG is a minimal valid 1x1 PNG. Its leading bytes make
// http.DetectContentType return "image/png".
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // signature
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR length + type
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89,
}

// handlerTestEnv bundles the Handler under test with the book.Repo used to seed
// owned books and the router that mounts the asset routes.
type handlerTestEnv struct {
	handler *Handler
	books   *book.Repo
	store   storage.Local
	editor  *stubEditor
	imgSrv  *httptest.Server
	router  chi.Router
}

// newHandlerTestEnv opens an in-memory DB, seeds two users, and wires a full
// asset Handler (service + local storage + comicify with a stub editor) onto a
// chi router. The stub editor returns a URL served by an in-test PNG server, so
// the comic-ification download+store step runs without any real network calls.
func newHandlerTestEnv(t *testing.T) *handlerTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedHandlerUsers(t, d, 2)

	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	t.Cleanup(imgSrv.Close)

	books := book.NewRepo(d)
	svc := NewService(NewRepo(d), books)
	store := storage.Local{Root: t.TempDir()}
	editor := &stubEditor{remoteURL: imgSrv.URL + "/styled.png"}
	comic := comicify.NewService(editor, store, imgSrv.Client())
	h := NewHandler(svc, store, comic)

	r := chi.NewRouter()
	h.Mount(r)
	return &handlerTestEnv{handler: h, books: books, store: store, editor: editor, imgSrv: imgSrv, router: r}
}

// seedUpload writes a stored upload under bookID and returns its /media URL, as
// if the client had just uploaded a reference image.
func (e *handlerTestEnv) seedUpload(t *testing.T, bookID int64) string {
	t.Helper()
	url, err := e.store.SaveBytes(bookID, ".png", tinyPNG)
	if err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	return url
}

// seedHandlerUsers inserts n users to satisfy the books.user_id foreign key.
func seedHandlerUsers(t *testing.T, d *sql.DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := d.Exec(
			`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
			"user"+string(rune('0'+i)), "hash",
		); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

// serveAs executes req against the router as the given authenticated user.
func (e *handlerTestEnv) serveAs(t *testing.T, req *http.Request, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	req = req.WithContext(auth.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// uploadRequest builds a multipart POST to the upload endpoint with a single
// "file" field carrying content.
func uploadRequest(t *testing.T, bookID int64, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "asset.bin")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	url := "/api/books/" + strconv.FormatInt(bookID, 10) + "/upload"
	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadRejectsNonImage(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	req := uploadRequest(t, b.ID, []byte("this is plain text, definitely not an image"))
	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非图片上传应为 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestUploadAcceptsPNG(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	req := uploadRequest(t, b.ID, tinyPNG)
	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("PNG 上传应为 200, got %d: %s", rec.Code, rec.Body)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	prefix := "/media/" + strconv.FormatInt(b.ID, 10) + "/"
	if !strings.HasPrefix(resp["imageUrl"], prefix) {
		t.Fatalf("imageUrl 应以 %q 开头, got %q", prefix, resp["imageUrl"])
	}
	if !strings.HasSuffix(resp["imageUrl"], ".png") {
		t.Fatalf("imageUrl 应以 .png 结尾, got %q", resp["imageUrl"])
	}
}

func TestUploadCrossUserBlocked(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	// 用户 2 向用户 1 的书上传 → 404,不泄露书存在。
	req := uploadRequest(t, b.ID, tinyPNG)
	rec := env.serveAs(t, req, 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权上传应为 404, got %d: %s", rec.Code, rec.Body)
	}
}

func TestCharacterCreateAndListViaHTTP(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	body := strings.NewReader(`{"name":"狐狸","type":"character","description":"聪明"}`)
	req := httptest.NewRequest(http.MethodPost, base, body)
	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建角色应为 201, got %d: %s", rec.Code, rec.Body)
	}
	var created models.Character
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == 0 || created.BookID != b.ID || created.Name != "狐狸" {
		t.Fatalf("创建结果异常: %+v", created)
	}

	rec = env.serveAs(t, httptest.NewRequest(http.MethodGet, base, nil), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("列角色应为 200, got %d", rec.Code)
	}
	var list []models.Character
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "狐狸" {
		t.Fatalf("期望 1 个角色, got %+v", list)
	}
}

func TestCharacterCreateCrossUser404(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	body := strings.NewReader(`{"name":"入侵","type":"character"}`)
	req := httptest.NewRequest(http.MethodPost, base, body)
	rec := env.serveAs(t, req, 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权创建角色应为 404, got %d: %s", rec.Code, rec.Body)
	}
}

func TestCharacterUpdateDeleteCrossUser404(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	// Owner creates a character.
	body := strings.NewReader(`{"name":"狐狸","type":"character","description":"原始"}`)
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base, body), 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %d %s", rec.Code, rec.Body)
	}
	var created models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	item := base + "/" + strconv.FormatInt(created.ID, 10)

	// Cross-user update → 404.
	upBody := strings.NewReader(`{"name":"篡改","type":"character","description":"改动"}`)
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item, upBody), 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权更新应为 404, got %d: %s", rec.Code, rec.Body)
	}

	// Cross-user delete → 404.
	rec = env.serveAs(t, httptest.NewRequest(http.MethodDelete, item, nil), 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权删除应为 404, got %d: %s", rec.Code, rec.Body)
	}

	// Asset unchanged for the owner.
	rec = env.serveAs(t, httptest.NewRequest(http.MethodGet, base, nil), 1)
	var list []models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Description != "原始" {
		t.Fatalf("角色应保持不变, got %+v", list)
	}
}

func TestSceneCreateAndCrossUser(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/scenes"

	// Owner creates a scene → 201.
	body := strings.NewReader(`{"name":"森林","description":"原始"}`)
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base, body), 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建场景应为 201, got %d: %s", rec.Code, rec.Body)
	}
	var created models.Scene
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == 0 || created.BookID != b.ID {
		t.Fatalf("创建场景结果异常: %+v", created)
	}

	// Cross-user create → 404.
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"name":"入侵"}`)), 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权创建场景应为 404, got %d: %s", rec.Code, rec.Body)
	}

	// Cross-user update/delete → 404.
	item := base + "/" + strconv.FormatInt(created.ID, 10)
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item, strings.NewReader(`{"name":"篡改"}`)), 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权更新场景应为 404, got %d: %s", rec.Code, rec.Body)
	}
	rec = env.serveAs(t, httptest.NewRequest(http.MethodDelete, item, nil), 2)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权删除场景应为 404, got %d: %s", rec.Code, rec.Body)
	}
}

func TestCharacterOwnerUpdateDelete(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base,
		strings.NewReader(`{"name":"狐狸","type":"character","description":"原始"}`)), 1)
	var created models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	item := base + "/" + strconv.FormatInt(created.ID, 10)

	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item,
		strings.NewReader(`{"name":"狐狸2","type":"character","description":"新的"}`)), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner 更新应为 200, got %d: %s", rec.Code, rec.Body)
	}
	var updated models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Description != "新的" {
		t.Fatalf("更新未生效: %+v", updated)
	}

	rec = env.serveAs(t, httptest.NewRequest(http.MethodDelete, item, nil), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner 删除应为 200, got %d", rec.Code)
	}
}

func TestSceneListAndOwnerUpdateDelete(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/scenes"

	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base,
		strings.NewReader(`{"name":"森林","description":"原始"}`)), 1)
	var created models.Scene
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	rec = env.serveAs(t, httptest.NewRequest(http.MethodGet, base, nil), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("列场景应为 200, got %d", rec.Code)
	}
	var list []models.Scene
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("期望 1 个场景, got %d", len(list))
	}

	item := base + "/" + strconv.FormatInt(created.ID, 10)
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item,
		strings.NewReader(`{"name":"森林2","description":"新的"}`)), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner 更新场景应为 200, got %d: %s", rec.Code, rec.Body)
	}
	rec = env.serveAs(t, httptest.NewRequest(http.MethodDelete, item, nil), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner 删除场景应为 200, got %d", rec.Code)
	}
}

func TestUploadMissingFileField400(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// A field, but not named "file".
	_ = mw.WriteField("other", "x")
	_ = mw.Close()
	url := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/upload"
	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺少 file 字段应为 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestUploadEmptyFile400(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	req := uploadRequest(t, b.ID, []byte{})
	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空文件应为 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestBadJSONBody400(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base,
		strings.NewReader(`{not json`)), 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应为 400, got %d", rec.Code)
	}
}

func TestInvalidAssetID400(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	url := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters/abc"

	rec := env.serveAs(t, httptest.NewRequest(http.MethodDelete, url, nil), 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法资产 ID 应为 400, got %d", rec.Code)
	}
}

func TestCreateCharacterComicifiesUpload(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"
	upload := env.seedUpload(t, b.ID)

	body := `{"name":"狐狸","type":"character","imageUrl":"` + upload + `"}`
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base, strings.NewReader(body)), 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建角色应为 201, got %d: %s", rec.Code, rec.Body)
	}
	var created models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if env.editor.calls != 1 {
		t.Fatalf("comicify should run once, ran %d", env.editor.calls)
	}
	if created.ImageURL == upload || !strings.HasPrefix(created.ImageURL, "/media/") {
		t.Fatalf("imageUrl should be replaced by stylized media url, got %q", created.ImageURL)
	}
}

func TestCreateCharacterSkipsComicifyWhenNoImage(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base,
		strings.NewReader(`{"name":"狐狸","type":"character"}`)), 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建角色应为 201, got %d: %s", rec.Code, rec.Body)
	}
	if env.editor.calls != 0 {
		t.Fatalf("comicify should be skipped without image, ran %d", env.editor.calls)
	}
}

func TestCreateCharacterComicifyErrorIs502(t *testing.T) {
	env := newHandlerTestEnv(t)
	env.editor.err = errors.New("upstream boom")
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"
	upload := env.seedUpload(t, b.ID)

	body := `{"name":"狐狸","type":"character","imageUrl":"` + upload + `"}`
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base, strings.NewReader(body)), 1)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("comicify 失败应为 502, got %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("error leaked upstream detail: %s", rec.Body)
	}
}

func TestUpdateCharacterComicifiesOnlyNewUpload(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/characters"

	// Create with an initial upload → 1 comicify call, stored stylized url.
	upload1 := env.seedUpload(t, b.ID)
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base,
		strings.NewReader(`{"name":"狐狸","type":"character","imageUrl":"`+upload1+`"}`)), 1)
	var created models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if env.editor.calls != 1 {
		t.Fatalf("create should comicify once, ran %d", env.editor.calls)
	}
	stylized := created.ImageURL
	item := base + "/" + strconv.FormatInt(created.ID, 10)

	// Update text only, keeping the stored stylized image → NO new comicify.
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item,
		strings.NewReader(`{"name":"狐狸2","type":"character","imageUrl":"`+stylized+`"}`)), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("更新应为 200, got %d: %s", rec.Code, rec.Body)
	}
	if env.editor.calls != 1 {
		t.Fatalf("unchanged image must not re-comicify, calls=%d", env.editor.calls)
	}

	// Update with a NEW upload → comicify runs again.
	upload2 := env.seedUpload(t, b.ID)
	rec = env.serveAs(t, httptest.NewRequest(http.MethodPut, item,
		strings.NewReader(`{"name":"狐狸2","type":"character","imageUrl":"`+upload2+`"}`)), 1)
	if rec.Code != http.StatusOK {
		t.Fatalf("更新(新图)应为 200, got %d: %s", rec.Code, rec.Body)
	}
	if env.editor.calls != 2 {
		t.Fatalf("new upload must re-comicify, calls=%d", env.editor.calls)
	}
	var updated models.Character
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.ImageURL == upload2 {
		t.Fatalf("new upload should be replaced by stylized url, got raw upload")
	}
}

func TestCreateSceneComicifiesUpload(t *testing.T) {
	env := newHandlerTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	base := "/api/books/" + strconv.FormatInt(b.ID, 10) + "/scenes"
	upload := env.seedUpload(t, b.ID)

	body := `{"name":"森林","imageUrl":"` + upload + `"}`
	rec := env.serveAs(t, httptest.NewRequest(http.MethodPost, base, strings.NewReader(body)), 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建场景应为 201, got %d: %s", rec.Code, rec.Body)
	}
	var created models.Scene
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if env.editor.calls != 1 {
		t.Fatalf("scene comicify should run once, ran %d", env.editor.calls)
	}
	if created.ImageURL == upload || !strings.HasPrefix(created.ImageURL, "/media/") {
		t.Fatalf("scene imageUrl should be replaced, got %q", created.ImageURL)
	}
}

func TestUploadInvalidBookID400(t *testing.T) {
	env := newHandlerTestEnv(t)
	req := uploadRequest(t, 0, tinyPNG)
	// bookId 0 in path -> parseBookID rejects.
	req = httptest.NewRequest(http.MethodPost, "/api/books/abc/upload", req.Body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := env.serveAs(t, req, 1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 bookId 应为 400, got %d", rec.Code)
	}
}
