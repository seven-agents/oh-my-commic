package storage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/seven-agents/oh-my-commic/internal/imageutil"
)

// webCacheMaxEntries bounds the in-process cache of web-optimized images.
// Filenames are content hashes so entries are stable; only downscaled entries
// hold bytes (~200KB each), and the cap keeps worst-case memory bounded.
const webCacheMaxEntries = 256

// maxOptimizeBytes caps how large a stored file we read into memory to optimize.
// Anything larger is left to the file server (our own writes are far smaller).
const maxOptimizeBytes = 20 << 20

// webCacheEntry holds the optimized encoding of one media file. A nil data
// marks a file not worth optimizing (small image, non-image, decode failure) so
// it is never re-decoded on subsequent requests.
type webCacheEntry struct {
	data []byte
	mime string
}

// webImageCache memoizes imageutil.CompressForWeb results, keyed by the cleaned
// request path. It is safe for concurrent use.
type webImageCache struct {
	mu    sync.RWMutex
	items map[string]webCacheEntry
}

func newWebImageCache() *webImageCache {
	return &webImageCache{items: make(map[string]webCacheEntry)}
}

// optimized returns the web-optimized bytes for the media file at cleanPath
// (already validated, still carrying the /media/ prefix). ok is false when the
// caller should serve the raw file instead: an explicit full=true, a non-image
// extension, an unreadable/oversized/missing file, or an image already small
// enough that recompressing would not help (or would flatten PNG alpha).
func (c *webImageCache) optimized(root, cleanPath string, full bool) (data []byte, mime string, ok bool) {
	if full {
		return nil, "", false
	}
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if !allowedExts[ext] {
		return nil, "", false
	}

	c.mu.RLock()
	e, hit := c.items[cleanPath]
	c.mu.RUnlock()
	if hit {
		if e.data == nil {
			return nil, "", false
		}
		return e.data, e.mime, true
	}

	rel := strings.TrimPrefix(cleanPath, mediaPrefix)
	fp := filepath.Join(root, filepath.FromSlash(rel))

	info, err := os.Stat(fp)
	if err != nil || info.IsDir() || info.Size() > maxOptimizeBytes {
		return nil, "", false
	}
	raw, err := os.ReadFile(fp)
	if err != nil {
		return nil, "", false
	}

	out, outMime, changed := imageutil.CompressForWeb(raw, imageutil.MimeFromExt(ext))
	c.store(cleanPath, out, outMime, changed)
	if !changed {
		return nil, "", false
	}
	return out, outMime, true
}

// store memoizes a result, honoring the entry cap. A non-changed result is
// cached as a negative marker (nil data) so the file is not re-decoded next time.
func (c *webImageCache) store(key string, data []byte, mime string, changed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= webCacheMaxEntries {
		return
	}
	if changed {
		c.items[key] = webCacheEntry{data: data, mime: mime}
		return
	}
	c.items[key] = webCacheEntry{}
}
