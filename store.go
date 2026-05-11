package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const uncategorizedLabel = "Uncategorized"

type Bookmark struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Category  string `json:"category"`
	HasIcon   bool   `json:"hasIcon"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

type appStore struct {
	db       *sql.DB
	dataDir  string
	iconsDir string
}

func newStore(db *sql.DB, dataDir string) *appStore {
	return &appStore{
		db:       db,
		dataDir:  dataDir,
		iconsDir: filepath.Join(dataDir, "icons"),
	}
}

func (s *appStore) ListBookmarks() ([]Bookmark, error) {
	rows, err := s.db.Query(`
SELECT id, name, url, category,
	CASE WHEN icon_path IS NOT NULL AND TRIM(icon_path) != '' THEN 1 ELSE 0 END,
	sort_order, created_at
FROM bookmarks
ORDER BY category, sort_order, name COLLATE NOCASE, id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		var hasIcon int
		if err := rows.Scan(&b.ID, &b.Name, &b.URL, &b.Category, &hasIcon, &b.SortOrder, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.HasIcon = hasIcon != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *appStore) GetBookmark(id int64) (Bookmark, string, error) {
	var b Bookmark
	var iconPath sql.NullString
	err := s.db.QueryRow(`
SELECT id, name, url, category, icon_path, sort_order, created_at
FROM bookmarks WHERE id = ?
`, id).Scan(&b.ID, &b.Name, &b.URL, &b.Category, &iconPath, &b.SortOrder, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, "", errNotFound
	}
	if err != nil {
		return Bookmark{}, "", err
	}
	path := ""
	if iconPath.Valid {
		path = strings.TrimSpace(iconPath.String)
	}
	b.HasIcon = path != ""
	return b, path, nil
}

func (s *appStore) ListCategories() ([]string, error) {
	rows, err := s.db.Query(`
SELECT DISTINCT TRIM(category) AS c FROM bookmarks WHERE TRIM(category) != '' ORDER BY c COLLATE NOCASE
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *appStore) CreateBookmark(name, rawURL, category string, sortOrder int, iconReader io.Reader, iconMime string) (Bookmark, error) {
	u, err := normalizeURL(rawURL)
	if err != nil {
		return Bookmark{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Bookmark{}, errBadRequest
	}
	cat := strings.TrimSpace(category)
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.Exec(`
INSERT INTO bookmarks (name, url, category, icon_path, sort_order, created_at)
VALUES (?, ?, ?, NULL, ?, ?)
`, name, u, cat, sortOrder, now)
	if err != nil {
		return Bookmark{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Bookmark{}, err
	}

	var iconPath string
	if iconReader != nil && iconMime != "" {
		iconPath, err = s.saveIconFile(id, iconReader, iconMime)
		if err != nil {
			_, _ = s.db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
			return Bookmark{}, err
		}
		if _, err := s.db.Exec(`UPDATE bookmarks SET icon_path = ? WHERE id = ?`, iconPath, id); err != nil {
			_ = os.Remove(iconPath)
			_, _ = s.db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
			return Bookmark{}, err
		}
	}

	return s.bookmarkByID(id)
}

func (s *appStore) UpdateBookmark(id int64, name, rawURL, category string, sortOrder int, replaceIcon bool, iconReader io.Reader, iconMime string) (Bookmark, error) {
	_, oldPath, err := s.GetBookmark(id)
	if err != nil {
		return Bookmark{}, err
	}
	u, err := normalizeURL(rawURL)
	if err != nil {
		return Bookmark{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Bookmark{}, errBadRequest
	}
	cat := strings.TrimSpace(category)

	if replaceIcon && iconReader != nil && iconMime != "" {
		newPath, err := s.saveIconFile(id, iconReader, iconMime)
		if err != nil {
			return Bookmark{}, err
		}
		if oldPath != "" && oldPath != newPath {
			s.removeIconFileOnDisk(oldPath)
		}
		if _, err := s.db.Exec(`UPDATE bookmarks SET name=?, url=?, category=?, icon_path=?, sort_order=? WHERE id=?`,
			name, u, cat, newPath, sortOrder, id); err != nil {
			_ = os.Remove(newPath)
			return Bookmark{}, err
		}
	} else {
		if _, err := s.db.Exec(`UPDATE bookmarks SET name=?, url=?, category=?, sort_order=? WHERE id=?`,
			name, u, cat, sortOrder, id); err != nil {
			return Bookmark{}, err
		}
	}
	return s.bookmarkByID(id)
}

// removeIconFileOnDisk deletes an icon file only if it lives under iconsDir (safety).
func (s *appStore) removeIconFileOnDisk(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	cp := filepath.Clean(path)
	base := filepath.Clean(s.iconsDir)
	rel, err := filepath.Rel(base, cp)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		log.Printf("icons: skip remove, path outside icons dir: %q", path)
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("icons: could not remove file %q: %v", path, err)
	}
}

func (s *appStore) ClearIcon(id int64) error {
	var p sql.NullString
	if err := s.db.QueryRow(`SELECT icon_path FROM bookmarks WHERE id=?`, id).Scan(&p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	path := ""
	if p.Valid {
		path = strings.TrimSpace(p.String)
	}
	// Clear DB first so the app never references a file we are about to delete.
	if _, err := s.db.Exec(`UPDATE bookmarks SET icon_path = NULL WHERE id=?`, id); err != nil {
		return err
	}
	s.removeIconFileOnDisk(path)
	return nil
}

func (s *appStore) DeleteBookmark(id int64) error {
	var p sql.NullString
	err := s.db.QueryRow(`SELECT icon_path FROM bookmarks WHERE id=?`, id).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM bookmarks WHERE id=?`, id); err != nil {
		return err
	}
	if p.Valid && strings.TrimSpace(p.String) != "" {
		s.removeIconFileOnDisk(p.String)
	}
	return nil
}

func (s *appStore) DeleteAllBookmarks() error {
	rows, err := s.db.Query(`SELECT icon_path FROM bookmarks WHERE icon_path IS NOT NULL AND icon_path != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return err
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM bookmarks`); err != nil {
		return err
	}
	for _, p := range paths {
		s.removeIconFileOnDisk(p)
	}
	return nil
}

func (s *appStore) bookmarkByID(id int64) (Bookmark, error) {
	b, _, err := s.GetBookmark(id)
	return b, err
}

func (s *appStore) saveIconFile(id int64, r io.Reader, mime string) (string, error) {
	ext := extFromMime(mime)
	dest := filepath.Join(s.iconsDir, fmt.Sprintf("%d%s", id, ext))
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, io.LimitReader(r, maxIconBytes))
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if n == 0 {
		_ = os.Remove(tmp)
		return "", errBadRequest
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

func extFromMime(mime string) string {
	m := strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	switch m {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	default:
		return ".img"
	}
}

const maxIconBytes = 2 << 20 // 2 MiB

func (s *appStore) URLExistsNormalized(u string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM bookmarks WHERE url = ?`, u).Scan(&n)
	return n > 0, err
}

const maxCategoryOrderEntries = 500

// GetCategoryOrder returns saved display category names (trimmed), first to last.
func (s *appStore) GetCategoryOrder() ([]string, error) {
	var raw string
	err := s.db.QueryRow(`SELECT v FROM settings WHERE k = 'category_order'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var order []string
	if err := json.Unmarshal([]byte(raw), &order); err != nil || order == nil {
		return []string{}, nil
	}
	return order, nil
}

// SetCategoryOrder persists display category order. Values are trimmed; empty strings dropped; duplicates collapsed.
func (s *appStore) SetCategoryOrder(order []string) error {
	seen := make(map[string]struct{})
	var out []string
	for _, x := range order {
		t := strings.TrimSpace(x)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) > maxCategoryOrderEntries {
			return errBadRequest
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO settings (k, v) VALUES ('category_order', ?)
ON CONFLICT(k) DO UPDATE SET v = excluded.v
`, string(b))
	return err
}
