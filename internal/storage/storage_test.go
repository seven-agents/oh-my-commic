package storage

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUnderBookDir(t *testing.T) {
	s := Local{Root: t.TempDir()}
	url, err := s.SaveBytes(7, ".png", []byte("img"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/media/7/") {
		t.Fatalf("URL 前缀错: %s", url)
	}
	// 文件应存在
	p := filepath.Join(s.Root, strings.TrimPrefix(url, "/media/"))
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("文件未落地: %v", err)
	}
}

func TestSaveFromReader(t *testing.T) {
	s := Local{Root: t.TempDir()}
	url, err := s.Save(42, ".jpg", bytes.NewReader([]byte("hello")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/media/42/") {
		t.Fatalf("URL 前缀错: %s", url)
	}
	p := filepath.Join(s.Root, strings.TrimPrefix(url, "/media/"))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("文件未落地: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("内容错: %q", string(b))
	}
}

func TestSaveRejectsMaliciousExt(t *testing.T) {
	s := Local{Root: t.TempDir()}
	bad := []string{
		"/../../evil",
		"../evil",
		"..",
		"foo/bar.png",
		".gif",  // 不在白名单
		".exe",  // 不在白名单
		"png",   // 缺少前导点
		"",      // 空
		".p/ng", // 含斜杠
	}
	for _, ext := range bad {
		if _, err := s.SaveBytes(1, ext, []byte("x")); err == nil {
			t.Fatalf("恶意 ext 未被拒绝: %q", ext)
		}
	}
}

func TestSaveAcceptsAllowedExts(t *testing.T) {
	s := Local{Root: t.TempDir()}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		if _, err := s.SaveBytes(1, ext, []byte("x")); err != nil {
			t.Fatalf("合法 ext 被拒绝 %q: %v", ext, err)
		}
	}
}

func TestSaveUniqueFilenames(t *testing.T) {
	s := Local{Root: t.TempDir()}
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		url, err := s.SaveBytes(3, ".png", []byte("x"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[url] {
			t.Fatalf("文件名冲突: %s", url)
		}
		seen[url] = true
	}
}

func TestHandlerServesFile(t *testing.T) {
	s := Local{Root: t.TempDir()}
	url, err := s.SaveBytes(9, ".png", []byte("pixels"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码错: %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "pixels" {
		t.Fatalf("内容错: %q", buf.String())
	}
}

func TestHandlerRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	// 在 Root 外部放一个秘密文件
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := Local{Root: root}
	handler := s.Handler()

	traversals := []string{
		"/media/../../" + filepath.Base(secretDir) + "/secret.txt",
		"/media/..%2f..%2fsecret.txt",
		"/media/....//secret.txt",
	}
	for _, p := range traversals {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if body := rec.Body.String(); strings.Contains(body, "TOPSECRET") {
			t.Fatalf("路径穿越泄露文件: %s -> %s", p, body)
		}
	}
}

func TestHandlerNoDirListing(t *testing.T) {
	s := Local{Root: t.TempDir()}
	if _, err := s.SaveBytes(5, ".png", []byte("x")); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/media/5/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		if strings.Contains(buf.String(), ".png") {
			t.Fatalf("暴露了目录列表: %s", buf.String())
		}
	}
}
