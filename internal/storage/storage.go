// Package storage provides book-scoped local file storage for media assets.
//
// Files are stored under Root/{bookID}/{randomID}{ext} and served over HTTP
// under the /media/ path prefix. The design is path-traversal safe: filenames
// are generated with crypto/rand (never user-controlled) and user-provided
// extensions are validated against an image allowlist.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// mediaPrefix is the URL path prefix under which stored files are served.
const mediaPrefix = "/media/"

// randomNameBytes is the number of random bytes used for a filename (hex-encoded).
const randomNameBytes = 16

// allowedExts is the set of permitted, lower-cased image extensions (with dot).
var allowedExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// Local stores media files on the local filesystem rooted at Root.
type Local struct {
	Root string
}

// validateExt validates a user-provided extension. It must be a known image
// extension with a leading dot and no path separators or traversal sequences.
func validateExt(ext string) (string, error) {
	if ext == "" {
		return "", fmt.Errorf("storage: 空扩展名")
	}
	if strings.ContainsAny(ext, `/\`) || strings.Contains(ext, "..") {
		return "", fmt.Errorf("storage: 非法扩展名 %q", ext)
	}
	lower := strings.ToLower(ext)
	if !allowedExts[lower] {
		return "", fmt.Errorf("storage: 不支持的扩展名 %q", ext)
	}
	return lower, nil
}

// randomID returns a hex-encoded random identifier suitable as a filename stem.
func randomID() (string, error) {
	buf := make([]byte, randomNameBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("storage: 生成随机文件名失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// SaveBytes writes b to Root/{bookID}/{randomID}{ext} and returns the relative
// URL /media/{bookID}/{file}. ext must be an allowed image extension.
func (s Local) SaveBytes(bookID int64, ext string, b []byte) (string, error) {
	return s.saveUnder(strconv.FormatInt(bookID, 10), ext, b)
}

// SaveUserAvatar writes b under Root/users/{userID}/{randomID}{ext} and returns
// the relative URL /media/users/{userID}/{file}. User avatars live in their own
// "users/" namespace so they never collide with the book-scoped {bookID}/ dirs.
// ext must be an allowed image extension. The userID is rendered as a decimal
// string (never user-controlled path text), so the resulting directory is
// path-traversal safe.
func (s Local) SaveUserAvatar(userID int64, ext string, b []byte) (string, error) {
	return s.saveUnder(path.Join("users", strconv.FormatInt(userID, 10)), ext, b)
}

// saveUnder writes b to Root/{subDir}/{randomID}{ext} and returns the relative
// URL /media/{subDir}/{file}. subDir must be a server-generated, slash-separated
// path segment (never raw user input); ext is validated against the image
// allowlist. It is the shared implementation behind SaveBytes/SaveUserAvatar.
func (s Local) saveUnder(subDir, ext string, b []byte) (string, error) {
	cleanExt, err := validateExt(ext)
	if err != nil {
		return "", err
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(s.Root, filepath.FromSlash(subDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("storage: 创建目录失败: %w", err)
	}

	name := id + cleanExt
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		return "", fmt.Errorf("storage: 写入文件失败: %w", err)
	}

	// URLs always use forward slashes regardless of OS path separator.
	relURL := mediaPrefix + path.Join(subDir, name)
	return relURL, nil
}

// ReadByURL resolves a media URL of the form /media/{bookId}/{file} to a file
// under Root and returns its bytes plus lower-cased extension (with dot).
//
// It is path-traversal safe: the URL is percent-independent (callers pass the
// server-generated relURL), cleaned, and any residual ".." or absolute escape is
// rejected. The resolved path is additionally verified to remain within Root.
func (s Local) ReadByURL(relURL string) (data []byte, ext string, err error) {
	if !strings.HasPrefix(relURL, mediaPrefix) {
		return nil, "", fmt.Errorf("storage: 非法媒体地址")
	}
	rel := strings.TrimPrefix(relURL, mediaPrefix)
	// path.Clean collapses any "." / ".." segments; reject anything that still
	// escapes (leading ".." or absolute) so we never read outside Root.
	clean := path.Clean("/" + rel)
	if strings.Contains(rel, "..") || clean == "/" {
		return nil, "", fmt.Errorf("storage: 非法媒体地址")
	}

	full := filepath.Join(s.Root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	rootAbs, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, "", fmt.Errorf("storage: 解析根目录失败: %w", err)
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return nil, "", fmt.Errorf("storage: 解析路径失败: %w", err)
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("storage: 非法媒体地址")
	}

	b, err := os.ReadFile(fullAbs)
	if err != nil {
		return nil, "", fmt.Errorf("storage: 读取媒体文件失败: %w", err)
	}
	return b, strings.ToLower(filepath.Ext(fullAbs)), nil
}

// Save reads all of r and delegates to SaveBytes.
func (s Local) Save(bookID int64, ext string, r io.Reader) (string, error) {
	if r == nil {
		return "", fmt.Errorf("storage: reader 为空")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("storage: 读取输入失败: %w", err)
	}
	return s.SaveBytes(bookID, ext, b)
}

// Handler serves stored files under the /media/ prefix from Root.
//
// It is path-traversal safe: http.Dir already rejects paths escaping Root, and
// noDirFS additionally disables directory listings. As defense-in-depth the
// wrapper cleans the request path and rejects any that still contain traversal
// sequences after decoding.
func (s Local) Handler() http.Handler {
	fs := http.FileServer(noDirFS{http.Dir(s.Root)})
	stripped := http.StripPrefix(mediaPrefix, fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is already percent-decoded by net/http. Reject any
		// residual traversal sequence defensively.
		clean := path.Clean(r.URL.Path)
		if strings.Contains(r.URL.Path, "..") || strings.Contains(clean, "..") {
			http.Error(w, "非法路径", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(clean, mediaPrefix) && clean != strings.TrimSuffix(mediaPrefix, "/") {
			http.NotFound(w, r)
			return
		}
		stripped.ServeHTTP(w, r)
	})
}

// noDirFS wraps an http.FileSystem to disable directory listings by returning
// an error when a directory is opened.
type noDirFS struct {
	fs http.FileSystem
}

// Open opens the named file, returning os.ErrPermission for directories so the
// FileServer responds 404 instead of listing contents.
func (n noDirFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, os.ErrPermission
	}
	return f, nil
}
