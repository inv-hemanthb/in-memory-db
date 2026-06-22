package api

import (
	"html/template"
	"net/http"
	"path/filepath"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

type ResultData struct {
	Title       string
	Error       string
	Item        *apidb.Item
	Extra       string
	BatchCount  int
	TotalMs     int64
	AvgMs       float64
	CacheHits   int
	CacheMisses int
}

type responseData struct {
	Result  ResultData
	Metrics MetricsView
}

func loadTemplates() (*template.Template, error) {
	root, err := db.RepoRoot()
	if err != nil {
		return nil, err
	}

	templatesDir := filepath.Join(root, "web", "templates")
	files := []string{
		filepath.Join(templatesDir, "index.html"),
		filepath.Join(templatesDir, "partials", "result.html"),
		filepath.Join(templatesDir, "partials", "metrics.html"),
		filepath.Join(templatesDir, "partials", "response.html"),
	}

	return template.New("").Funcs(template.FuncMap{
		"formatCacheHit": formatCacheHit,
		"formatSuccess":  formatSuccess,
		"recentReverse":  recentReverse,
	}).ParseFiles(files...)
}

func recentReverse(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[len(entries)-1-i] = e
	}
	return out
}

func formatCacheHit(hit *bool) string {
	if hit == nil {
		return "n/a"
	}
	if *hit {
		return "hit"
	}
	return "miss"
}

func formatSuccess(success bool) string {
	if success {
		return "ok"
	}
	return "fail"
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("render template %s: %v", name, err)
	}
}

func (s *Server) renderHTMXResponse(w http.ResponseWriter, status int, result ResultData) {
	s.render(w, status, "partials/response.html", responseData{
		Result:  result,
		Metrics: s.metrics.Snapshot(),
	})
}

func staticDir() (string, error) {
	root, err := db.RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "web", "static"), nil
}
