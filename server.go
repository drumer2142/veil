package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxImportBytes = 10 << 20 // 10 MiB
const maxBookmarksImport = 5000

type exportDoc struct {
	Version         int              `json:"version"`
	ExportedAt      string           `json:"exportedAt"`
	CategoryOrder   []string         `json:"categoryOrder,omitempty"`
	Bookmarks       []exportBookmark `json:"bookmarks"`
}

type exportBookmark struct {
	Name      string       `json:"name"`
	URL       string       `json:"url"`
	Category  string       `json:"category"`
	SortOrder int          `json:"sortOrder"`
	Icon      *iconPayload `json:"icon,omitempty"`
}

type iconPayload struct {
	Mime string `json:"mime"`
	Data string `json:"data"` // base64
}

type importDoc struct {
	Version         int              `json:"version"`
	CategoryOrder   []string         `json:"categoryOrder"`
	Bookmarks       []importBookmark `json:"bookmarks"`
}

type categoryOrderBody struct {
	Order []string `json:"order"`
}

type importBookmark struct {
	Name      string       `json:"name"`
	URL       string       `json:"url"`
	Category  string       `json:"category"`
	SortOrder int          `json:"sortOrder"`
	Icon      *iconPayload `json:"icon,omitempty"`
}

type createBookmarkJSON struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Category  string `json:"category"`
	SortOrder int    `json:"sortOrder"`
}

type updateBookmarkJSON struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Category   string `json:"category"`
	SortOrder  int    `json:"sortOrder"`
	ClearIcon  bool   `json:"clearIcon"`
}

type server struct {
	store *appStore
	mux   *http.ServeMux
}

func newServer(st *appStore) http.Handler {
	s := &server{store: st, mux: http.NewServeMux()}
	s.routes()
	return loggingMiddleware(s.mux)
}

func (s *server) routes() {
	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}

	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	s.mux.HandleFunc("GET /api/bookmarks", s.handleListBookmarks)
	s.mux.HandleFunc("POST /api/bookmarks", s.handleCreateBookmark)
	s.mux.HandleFunc("GET /api/bookmarks/", s.handleBookmarkSubroutes)
	s.mux.HandleFunc("PUT /api/bookmarks/", s.handleBookmarkSubroutes)
	s.mux.HandleFunc("DELETE /api/bookmarks/", s.handleBookmarkSubroutes)

	s.mux.HandleFunc("GET /api/categories", s.handleListCategories)
	s.mux.HandleFunc("GET /api/category-order", s.handleGetCategoryOrder)
	s.mux.HandleFunc("PUT /api/category-order", s.handlePutCategoryOrder)
	s.mux.HandleFunc("GET /api/export", s.handleExport)
	s.mux.HandleFunc("POST /api/import", s.handleImport)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func (s *server) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListBookmarks()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.ListCategories()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *server) handleGetCategoryOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.store.GetCategoryOrder()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"order": o})
}

func (s *server) handlePutCategoryOrder(w http.ResponseWriter, r *http.Request) {
	var body categoryOrderBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.store.SetCategoryOrder(body.Order); err != nil {
		if errors.Is(err, errBadRequest) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	o, err := s.store.GetCategoryOrder()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"order": o})
}

func (s *server) handleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		s.createBookmarkJSON(w, r)
	case strings.HasPrefix(ct, "multipart/form-data"):
		s.createBookmarkMultipart(w, r)
	default:
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
	}
}

func (s *server) createBookmarkJSON(w http.ResponseWriter, r *http.Request) {
	var body createBookmarkJSON
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	b, err := s.store.CreateBookmark(body.Name, body.URL, body.Category, body.SortOrder, nil, "")
	if err != nil {
		writeBookmarkErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *server) createBookmarkMultipart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 21); err != nil { // 2 MiB + fields
		http.Error(w, "invalid multipart", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	rawURL := r.FormValue("url")
	category := r.FormValue("category")
	sortOrder, _ := strconv.Atoi(r.FormValue("sortOrder"))
	var iconReader io.Reader
	var iconMime string
	if fh, hdr, err := r.FormFile("icon"); err == nil {
		defer fh.Close()
		if hdr.Size > maxIconBytes {
			http.Error(w, "icon too large", http.StatusBadRequest)
			return
		}
		iconMime = hdr.Header.Get("Content-Type")
		if iconMime == "" {
			iconMime = "application/octet-stream"
		}
		iconReader = fh
	} else if err != nil && !errors.Is(err, http.ErrMissingFile) {
		http.Error(w, "invalid icon", http.StatusBadRequest)
		return
	}
	b, err := s.store.CreateBookmark(name, rawURL, category, sortOrder, iconReader, iconMime)
	if err != nil {
		writeBookmarkErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *server) handleBookmarkSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/bookmarks/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 2 && parts[1] == "icon" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.serveIcon(w, r, id)
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.updateBookmark(w, r, id)
	case http.MethodDelete:
		s.deleteBookmark(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) serveIcon(w http.ResponseWriter, r *http.Request, id int64) {
	_, iconPath, err := s.store.GetBookmark(id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if iconPath == "" {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(filepath.Clean(iconPath), filepath.Clean(s.store.iconsDir)) {
		http.Error(w, "invalid icon path", http.StatusInternalServerError)
		return
	}
	f, err := os.Open(iconPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	ext := filepath.Ext(iconPath)
	mt := mime.TypeByExtension(ext)
	if mt == "" {
		mt = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mt)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, f)
}

func (s *server) updateBookmark(w http.ResponseWriter, r *http.Request, id int64) {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		s.updateBookmarkJSON(w, r, id)
	case strings.HasPrefix(ct, "multipart/form-data"):
		s.updateBookmarkMultipart(w, r, id)
	default:
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
	}
}

func (s *server) updateBookmarkJSON(w http.ResponseWriter, r *http.Request, id int64) {
	var body updateBookmarkJSON
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.ClearIcon {
		if err := s.store.ClearIcon(id); err != nil {
			writeBookmarkErr(w, err)
			return
		}
	}
	b, err := s.store.UpdateBookmark(id, body.Name, body.URL, body.Category, body.SortOrder, false, nil, "")
	if err != nil {
		writeBookmarkErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *server) updateBookmarkMultipart(w http.ResponseWriter, r *http.Request, id int64) {
	if err := r.ParseMultipartForm(1 << 21); err != nil {
		http.Error(w, "invalid multipart", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	rawURL := r.FormValue("url")
	category := r.FormValue("category")
	sortOrder, _ := strconv.Atoi(r.FormValue("sortOrder"))
	clearIcon := r.FormValue("clearIcon") == "1" || strings.EqualFold(r.FormValue("clearIcon"), "true")
	if clearIcon {
		if err := s.store.ClearIcon(id); err != nil {
			writeBookmarkErr(w, err)
			return
		}
	}
	var iconReader io.Reader
	var iconMime string
	replace := false
	if fh, hdr, err := r.FormFile("icon"); err == nil {
		defer fh.Close()
		if hdr.Size > maxIconBytes {
			http.Error(w, "icon too large", http.StatusBadRequest)
			return
		}
		iconMime = hdr.Header.Get("Content-Type")
		if iconMime == "" {
			iconMime = "application/octet-stream"
		}
		iconReader = fh
		replace = true
	} else if err != nil && !errors.Is(err, http.ErrMissingFile) {
		http.Error(w, "invalid icon", http.StatusBadRequest)
		return
	}
	b, err := s.store.UpdateBookmark(id, name, rawURL, category, sortOrder, replace, iconReader, iconMime)
	if err != nil {
		writeBookmarkErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *server) deleteBookmark(w http.ResponseWriter, r *http.Request, id int64) {
	if err := s.store.DeleteBookmark(id); err != nil {
		writeBookmarkErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListBookmarks()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	catOrder, err := s.store.GetCategoryOrder()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	doc := exportDoc{
		Version:        1,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		CategoryOrder:  catOrder,
		Bookmarks:      make([]exportBookmark, 0, len(list)),
	}
	for _, b := range list {
		eb := exportBookmark{Name: b.Name, URL: b.URL, Category: b.Category, SortOrder: b.SortOrder}
		_, iconPath, err := s.store.GetBookmark(b.ID)
		if err != nil {
			continue
		}
		if iconPath != "" {
			data, err := os.ReadFile(iconPath)
			if err == nil && len(data) > 0 {
				ext := filepath.Ext(iconPath)
				mt := mime.TypeByExtension(ext)
				if mt == "" {
					mt = "application/octet-stream"
				}
				eb.Icon = &iconPayload{Mime: mt, Data: base64.StdEncoding.EncodeToString(data)}
			}
		}
		doc.Bookmarks = append(doc.Bookmarks, eb)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="dashboard-export.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return
	}
}

func (s *server) handleImport(w http.ResponseWriter, r *http.Request) {
	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode != "replace" && mode != "merge" {
		http.Error(w, "query mode must be replace or merge", http.StatusBadRequest)
		return
	}
	body, err := readImportBody(r)
	if err != nil {
		if errors.Is(err, errPayloadLimit) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		if errors.Is(err, errBadRequest) {
			http.Error(w, "invalid import body", http.StatusBadRequest)
			return
		}
		http.Error(w, "invalid import body", http.StatusBadRequest)
		return
	}
	var doc importDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if doc.Version != 0 && doc.Version != 1 {
		http.Error(w, "unsupported export version", http.StatusBadRequest)
		return
	}
	if len(doc.Bookmarks) > maxBookmarksImport {
		http.Error(w, "too many bookmarks", http.StatusBadRequest)
		return
	}
	if mode == "replace" {
		if err := s.store.DeleteAllBookmarks(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}
	imported := 0
	skipped := 0
	for _, row := range doc.Bookmarks {
		u, err := normalizeURL(row.URL)
		if err != nil {
			skipped++
			continue
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			skipped++
			continue
		}
		if mode == "merge" {
			exists, err := s.store.URLExistsNormalized(u)
			if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if exists {
				skipped++
				continue
			}
		}
		var iconR io.Reader
		var iconMime string
		if row.Icon != nil && row.Icon.Data != "" {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(row.Icon.Data))
			if err != nil || len(raw) > maxIconBytes {
				skipped++
				continue
			}
			iconR = bytes.NewReader(raw)
			iconMime = row.Icon.Mime
			if iconMime == "" {
				iconMime = http.DetectContentType(raw)
			}
		}
		if _, errCreate := s.store.CreateBookmark(name, u, row.Category, row.SortOrder, iconR, iconMime); errCreate != nil {
			skipped++
			continue
		}
		imported++
	}
	if len(doc.CategoryOrder) > 0 {
		if err := s.store.SetCategoryOrder(doc.CategoryOrder); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	} else if mode == "replace" {
		if err := s.store.SetCategoryOrder(nil); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": imported, "skipped": skipped})
}

func readImportBody(r *http.Request) ([]byte, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		b, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxImportBytes))
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return nil, errPayloadLimit
			}
			return nil, err
		}
		return b, nil
	}
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImportBytes); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return nil, errPayloadLimit
			}
			return nil, err
		}
		if r.MultipartForm == nil {
			return nil, errBadRequest
		}
		fhs := r.MultipartForm.File["file"]
		if len(fhs) == 0 {
			return nil, errBadRequest
		}
		f, err := fhs[0].Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(io.LimitReader(f, maxImportBytes))
	}
	return nil, errBadRequest
}

func writeBookmarkErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvalidURL):
		http.Error(w, "invalid url", http.StatusBadRequest)
	case errors.Is(err, errBadRequest):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, errNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, "server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
