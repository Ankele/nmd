package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

// App holds the app state.
type App struct {
	ctx     context.Context
	db      *sql.DB
	dataDir string
	dirty   atomic.Bool
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = a.initStorage()
}

// shutdown is called when the app exits.
func (a *App) shutdown(context.Context) {
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
}

// Ping returns a startup message for frontend connectivity check.
func (a *App) Ping() string {
	return fmt.Sprintf("nmd initialized")
}

// SetDirtyState syncs frontend unsaved-change status to backend.
func (a *App) SetDirtyState(isDirty bool) {
	a.dirty.Store(isDirty)
}

// beforeClose blocks app close when there are unsaved changes unless user confirms discard.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if !a.dirty.Load() {
		return false
	}

	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Unsaved Changes",
		Message:       "You have unsaved changes. Discard and close?",
		Buttons:       []string{"Cancel", "Discard"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return true
	}
	return choice != "Discard"
}

type RecentFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// OpenFileResult is the payload returned to frontend when opening files.
type OpenFileResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// SaveFileRequest is the payload sent by frontend when saving files.
type SaveFileRequest struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// SaveFileResult is returned after save operation.
type SaveFileResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type ExportPDFRequest struct {
	Path         string `json:"path"`
	FileName     string `json:"fileName"`
	Content      string `json:"content"`
	DocumentPath string `json:"documentPath"`
}

type ExportPDFResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type SaveImageAssetRequest struct {
	DocumentPath string `json:"documentPath"`
	FileName     string `json:"fileName"`
	DataURL      string `json:"dataURL"`
}

type SaveImageAssetResult struct {
	AbsolutePath string `json:"absolutePath"`
	RelativePath string `json:"relativePath"`
}

func readMarkdownFile(path string) (*OpenFileResult, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, err
	}

	return &OpenFileResult{
		Path:    cleanPath,
		Name:    filepath.Base(cleanPath),
		Content: string(data),
	}, nil
}

func (a *App) initStorage() error {
	if a.db != nil {
		return nil
	}

	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	a.dataDir = filepath.Join(cfgDir, "nmd")
	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(a.dataDir, "nmd.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS recent_files (
  path TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`)
	if err != nil {
		_ = db.Close()
		return err
	}

	a.db = db
	return nil
}

func (a *App) ensureDB() error {
	if a.db != nil {
		return nil
	}
	return a.initStorage()
}

// OpenMarkdownFile opens a native file dialog and reads markdown file content.
func (a *App) OpenMarkdownFile() (*OpenFileResult, error) {
	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Markdown File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Markdown Files (*.md)",
				Pattern:     "*.md;*.markdown;*.txt",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(selectedPath) == "" {
		return nil, nil
	}

	return readMarkdownFile(selectedPath)
}

// OpenMarkdownFileAtPath reads a file from an absolute path for recent-file reopen.
func (a *App) OpenMarkdownFileAtPath(path string) (*OpenFileResult, error) {
	return readMarkdownFile(path)
}

// AddRecentFile stores or updates a file in recent-file history.
func (a *App) AddRecentFile(path string, name string) error {
	if err := a.ensureDB(); err != nil {
		return err
	}

	cleanPath := strings.TrimSpace(path)
	cleanName := strings.TrimSpace(name)
	if cleanPath == "" || cleanName == "" {
		return errors.New("path and name are required")
	}

	_, err := a.db.Exec(`
INSERT INTO recent_files(path, name, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(path) DO UPDATE SET
  name=excluded.name,
  updated_at=CURRENT_TIMESTAMP;
`, cleanPath, cleanName)
	return err
}

// ListRecentFiles returns recent files ordered by latest update.
func (a *App) ListRecentFiles(limit int) ([]RecentFile, error) {
	if err := a.ensureDB(); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	rows, err := a.db.Query(`
SELECT path, name
FROM recent_files
ORDER BY updated_at DESC
LIMIT ?;
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]RecentFile, 0, limit)
	for rows.Next() {
		item := RecentFile{}
		if err := rows.Scan(&item.Path, &item.Name); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// RemoveRecentFile removes a single file from recent-file history.
func (a *App) RemoveRecentFile(path string) error {
	if err := a.ensureDB(); err != nil {
		return err
	}
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("path is required")
	}
	_, err := a.db.Exec(`DELETE FROM recent_files WHERE path = ?;`, cleanPath)
	return err
}

// ClearRecentFiles clears all recent-file history.
func (a *App) ClearRecentFiles() error {
	if err := a.ensureDB(); err != nil {
		return err
	}
	_, err := a.db.Exec(`DELETE FROM recent_files;`)
	return err
}

// SaveMarkdownFile saves markdown content to selected path.
func (a *App) SaveMarkdownFile(req SaveFileRequest) (*SaveFileResult, error) {
	targetPath := strings.TrimSpace(req.Path)
	defaultName := strings.TrimSpace(req.FileName)
	if defaultName == "" {
		defaultName = "untitled.md"
	}

	if targetPath == "" {
		savedPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "Save Markdown File",
			DefaultFilename: defaultName,
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown Files (*.md)",
					Pattern:     "*.md",
				},
				{
					DisplayName: "Text Files (*.txt)",
					Pattern:     "*.txt",
				},
			},
		})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(savedPath) == "" {
			return nil, nil
		}
		targetPath = savedPath
	}

	if err := os.WriteFile(targetPath, []byte(req.Content), 0644); err != nil {
		return nil, err
	}

	return &SaveFileResult{
		Path: targetPath,
		Name: filepath.Base(targetPath),
	}, nil
}

// ExportMarkdownAsPDF exports markdown content into a PDF file.
func (a *App) ExportMarkdownAsPDF(req ExportPDFRequest) (*ExportPDFResult, error) {
	targetPath := strings.TrimSpace(req.Path)
	defaultName := strings.TrimSpace(req.FileName)
	if defaultName == "" {
		defaultName = "document.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(defaultName), ".pdf") {
		defaultName += ".pdf"
	}

	if targetPath == "" {
		savedPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "Export PDF",
			DefaultFilename: defaultName,
			Filters: []runtime.FileFilter{
				{
					DisplayName: "PDF Files (*.pdf)",
					Pattern:     "*.pdf",
				},
			},
		})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(savedPath) == "" {
			return nil, nil
		}
		targetPath = savedPath
	}

	if !strings.HasSuffix(strings.ToLower(targetPath), ".pdf") {
		targetPath += ".pdf"
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 14)
	fontName, err := configurePDFFont(pdf)
	if err != nil {
		return nil, err
	}
	pdf.SetFont(fontName, "", 11)
	pdf.AddPage()

	lines := strings.Split(strings.ReplaceAll(req.Content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		renderLine := line
		trimmed := strings.TrimSpace(line)
		if src, ok := parseMarkdownImageSrc(trimmed); ok {
			if abs, err := a.resolveImageAbsolutePath(req.DocumentPath, src); err == nil {
				opts := gofpdf.ImageOptions{ReadDpi: true}
				y := pdf.GetY()
				pdf.ImageOptions(abs, 14, y, 182, 0, false, opts, 0, "")
				pdf.Ln(4)
				continue
			}
		}

		switch {
		case strings.HasPrefix(trimmed, "# "):
			pdf.SetFont(fontName, "", 18)
			renderLine = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		case strings.HasPrefix(trimmed, "## "):
			pdf.SetFont(fontName, "", 15)
			renderLine = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
		case strings.HasPrefix(trimmed, "### "):
			pdf.SetFont(fontName, "", 13)
			renderLine = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
		default:
			pdf.SetFont(fontName, "", 11)
		}

		if strings.TrimSpace(renderLine) == "" {
			pdf.Ln(4)
			continue
		}
		pdf.MultiCell(0, 6, renderLine, "", "L", false)
	}

	if err := pdf.OutputFileAndClose(targetPath); err != nil {
		return nil, err
	}
	if info, err := os.Stat(targetPath); err != nil || info.Size() == 0 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("pdf export finished but output file is empty")
	}

	return &ExportPDFResult{
		Path: targetPath,
		Name: filepath.Base(targetPath),
	}, nil
}

func parseMarkdownImageSrc(line string) (string, bool) {
	m := regexp.MustCompile(`^!\[[^\]]*\]\((.+)\)$`).FindStringSubmatch(strings.TrimSpace(line))
	if len(m) != 2 {
		return "", false
	}
	inside := strings.TrimSpace(m[1])
	if inside == "" {
		return "", false
	}

	// Handle path in angle brackets: ![](</a/b c.png>)
	if strings.HasPrefix(inside, "<") {
		end := strings.Index(inside, ">")
		if end <= 1 {
			return "", false
		}
		return strings.TrimSpace(inside[1:end]), true
	}

	// Handle optional title: ![](path \"title\")
	parts := strings.Fields(inside)
	if len(parts) == 0 {
		return "", false
	}
	return strings.TrimSpace(parts[0]), true
}

func (a *App) resolveImageAbsolutePath(documentPath string, src string) (string, error) {
	s := strings.TrimSpace(src)
	s = strings.Trim(s, "<>")
	if s == "" {
		return "", errors.New("empty image src")
	}
	if strings.HasPrefix(strings.ToLower(s), "http://") || strings.HasPrefix(strings.ToLower(s), "https://") {
		return "", errors.New("remote image not supported in pdf export")
	}
	if strings.HasPrefix(strings.ToLower(s), "data:") {
		return "", errors.New("data url image not supported in pdf export")
	}

	if strings.HasPrefix(strings.ToLower(s), "file://") {
		u, err := url.Parse(s)
		if err == nil && u.Path != "" {
			s = u.Path
		}
	}
	if unescaped, err := url.PathUnescape(s); err == nil {
		s = unescaped
	}

	if filepath.IsAbs(s) {
		clean := filepath.Clean(s)
		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		} else {
			return "", err
		}
	}

	candidates := make([]string, 0, 4)
	doc := strings.TrimSpace(documentPath)
	if doc != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(doc), s))
	}
	if a.dataDir != "" {
		candidates = append(candidates, filepath.Join(a.dataDir, s))
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		candidates = append(candidates, filepath.Join(cwd, s))
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		candidates = append(candidates, filepath.Join(home, s))
	}

	for _, c := range candidates {
		clean := filepath.Clean(c)
		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		}
	}
	return "", errors.New("image file not found")
}

// SaveImageAsset stores a pasted/dropped image into assets/YYYY-MM-DD and returns file paths.
func (a *App) SaveImageAsset(req SaveImageAssetRequest) (*SaveImageAssetResult, error) {
	if strings.TrimSpace(req.DataURL) == "" {
		return nil, errors.New("dataURL is required")
	}

	raw, ext, err := decodeDataURLImage(req.DataURL)
	if err != nil {
		return nil, err
	}

	baseDir := ""
	docPath := strings.TrimSpace(req.DocumentPath)
	if docPath != "" {
		baseDir = filepath.Dir(docPath)
	}
	if baseDir == "" {
		if a.dataDir == "" {
			if err := a.initStorage(); err != nil {
				return nil, err
			}
		}
		baseDir = a.dataDir
	}

	dateDir := time.Now().Format("2006-01-02")
	assetDir := filepath.Join(baseDir, "assets", dateDir)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return nil, err
	}

	name := sanitizeFileName(req.FileName)
	if name == "" {
		name = fmt.Sprintf("image-%d", time.Now().UnixNano()/1e6)
	}
	if ext != "" && !strings.HasSuffix(strings.ToLower(name), ext) {
		name += ext
	}

	abs := filepath.Join(assetDir, name)
	if err := os.WriteFile(abs, raw, 0644); err != nil {
		return nil, err
	}

	rel := abs
	if docPath != "" {
		if rp, err := filepath.Rel(filepath.Dir(docPath), abs); err == nil {
			rel = filepath.ToSlash(rp)
		}
	}

	return &SaveImageAssetResult{
		AbsolutePath: abs,
		RelativePath: rel,
	}, nil
}

func decodeDataURLImage(dataURL string) ([]byte, string, error) {
	re := regexp.MustCompile(`^data:(image/[a-zA-Z0-9.+-]+);base64,(.+)$`)
	m := re.FindStringSubmatch(strings.TrimSpace(dataURL))
	if len(m) != 3 {
		return nil, "", errors.New("invalid image data URL")
	}

	mime := strings.ToLower(m[1])
	ext := ".png"
	switch mime {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "image/png":
		ext = ".png"
	}

	raw, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil {
		return nil, "", err
	}
	return raw, ext, nil
}

func sanitizeFileName(name string) string {
	n := strings.TrimSpace(name)
	n = strings.ReplaceAll(n, " ", "-")
	n = regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(n, "")
	n = strings.Trim(n, ".-")
	return n
}

// LoadImageDataURL reads an image file and returns a data URL for WebView-safe preview.
func (a *App) LoadImageDataURL(path string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", errors.New("path is required")
	}

	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(cleanPath))
	mime := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".svg":
		mime = "image/svg+xml"
	case ".bmp":
		mime = "image/bmp"
	case ".png":
		mime = "image/png"
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(raw)), nil
}

// ResolveImageDataURL resolves markdown image src against document path and returns a data URL.
func (a *App) ResolveImageDataURL(documentPath string, src string) (string, error) {
	s := strings.TrimSpace(src)
	if s == "" {
		return "", errors.New("image src is required")
	}
	s = strings.Trim(s, "<>")
	if strings.HasPrefix(strings.ToLower(s), "data:") {
		return s, nil
	}
	if strings.HasPrefix(strings.ToLower(s), "http://") || strings.HasPrefix(strings.ToLower(s), "https://") {
		return s, nil
	}

	if strings.HasPrefix(strings.ToLower(s), "file://") {
		u, err := url.Parse(s)
		if err == nil && u.Path != "" {
			s = u.Path
		}
	}

	if unescaped, err := url.PathUnescape(s); err == nil {
		s = unescaped
	}

	abs := s
	if !filepath.IsAbs(abs) {
		doc := strings.TrimSpace(documentPath)
		if doc != "" {
			abs = filepath.Join(filepath.Dir(doc), s)
		} else {
			// Fallbacks for restored drafts that may not carry document path.
			candidates := make([]string, 0, 3)
			if a.dataDir != "" {
				candidates = append(candidates, filepath.Join(a.dataDir, s))
			}
			if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
				candidates = append(candidates, filepath.Join(cwd, s))
			}
			if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
				candidates = append(candidates, filepath.Join(home, s))
			}
			for _, c := range candidates {
				clean := filepath.Clean(c)
				if _, err := os.Stat(clean); err == nil {
					abs = clean
					return a.LoadImageDataURL(abs)
				}
			}
			return "", errors.New("cannot resolve relative image path without document path")
		}
	}
	abs = filepath.Clean(abs)
	return a.LoadImageDataURL(abs)
}

func configurePDFFont(pdf *gofpdf.Fpdf) (string, error) {
	type fontCandidate struct {
		Name string
		Path string
	}

	candidates := []fontCandidate{
		{Name: "ArialUnicode", Path: "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"},
		{Name: "NISC18030", Path: "/System/Library/Fonts/Supplemental/NISC18030.ttf"},
	}

	var lastErr error
	for _, candidate := range candidates {
		fontBytes, err := os.ReadFile(candidate.Path)
		if err != nil {
			continue
		}

		// Try candidate on a temporary pdf instance first.
		probe := gofpdf.New("P", "mm", "A4", "")
		probe.AddUTF8FontFromBytes(candidate.Name, "", fontBytes)
		if probe.Error() != nil {
			lastErr = probe.Error()
			continue
		}

		pdf.AddUTF8FontFromBytes(candidate.Name, "", fontBytes)
		if pdf.Error() == nil {
			return candidate.Name, nil
		}
		lastErr = pdf.Error()
	}

	// Last resort: fallback to built-in latin font.
	pdf.SetFont("Arial", "", 11)
	if pdf.Error() == nil {
		return "Arial", nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", pdf.Error()
}
