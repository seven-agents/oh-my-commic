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
