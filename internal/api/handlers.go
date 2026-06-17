package api

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
)

func parseWithKV(r *http.Request) bool {
	v := strings.TrimSpace(r.FormValue("with_kv"))
	switch strings.ToLower(v) {
	case "on", "true", "1", "yes":
		return true
	default:
		return false
	}
}

func parseHardDelete(r *http.Request) bool {
	v := strings.TrimSpace(r.FormValue("hard"))
	switch strings.ToLower(v) {
	case "on", "true", "1", "yes":
		return true
	default:
		return false
	}
}

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, apidb.ErrNotFound):
		return http.StatusNotFound, "not found"
	case errors.Is(err, apidb.ErrDuplicateKey):
		return http.StatusConflict, "duplicate key"
	case errors.Is(err, kvclient.ErrServerBusy):
		return http.StatusServiceUnavailable, "kv server busy"
	default:
		return http.StatusInternalServerError, err.Error()
	}
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (s *Server) recordMetrics(op string, withKV bool, cacheHit *bool, start time.Time) {
	s.metrics.Record(Entry{
		At:        time.Now(),
		Op:        op,
		LatencyMs: time.Since(start).Milliseconds(),
		WithKV:    withKV,
		CacheHit:  cacheHit,
	})
}

func (s *Server) renderResponse(w http.ResponseWriter, status int, resultHTML string) {
	snap := s.metrics.Snapshot()
	writeHTML(w, status, resultHTML+renderMetricsHTML(snap))
}

func renderMetricsHTML(snap MetricsView) string {
	var b strings.Builder
	b.WriteString("<section><h2>Metrics</h2>")
	b.WriteString(fmt.Sprintf("<p>Last: %s | %dms | with_kv=%t",
		html.EscapeString(snap.Last.Op),
		snap.Last.LatencyMs,
		snap.Last.WithKV,
	))
	if snap.Last.CacheHit != nil {
		b.WriteString(fmt.Sprintf(" | cache_hit=%t", *snap.Last.CacheHit))
	}
	b.WriteString("</p>")
	b.WriteString(fmt.Sprintf("<p>Total ops: %d | Avg latency: %dms</p>", snap.TotalCount, snap.AvgLatencyMs))
	if len(snap.Recent) > 0 {
		b.WriteString("<ul>")
		for i := len(snap.Recent) - 1; i >= 0; i-- {
			e := snap.Recent[i]
			line := fmt.Sprintf("%s %s %dms kv=%t", e.At.Format("15:04:05"), e.Op, e.LatencyMs, e.WithKV)
			if e.CacheHit != nil {
				line += fmt.Sprintf(" hit=%t", *e.CacheHit)
			}
			b.WriteString("<li>" + html.EscapeString(line) + "</li>")
		}
		b.WriteString("</ul>")
	}
	b.WriteString("</section>")
	return b.String()
}

func renderItemHTML(item apidb.Item) string {
	return fmt.Sprintf(
		"<p>id=%d key=%s value=%s</p>",
		item.ID,
		html.EscapeString(item.Key),
		html.EscapeString(item.Value),
	)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}
	writeHTML(w, http.StatusOK, "<p>API running</p>")
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid form</p>")
		return
	}

	start := time.Now()
	withKV := parseWithKV(r)
	key := strings.TrimSpace(r.FormValue("key"))
	value := r.FormValue("value")
	if key == "" {
		writeHTML(w, http.StatusBadRequest, "<p>key is required</p>")
		return
	}

	result, err := s.service.Create(r.Context(), withKV, key, value)
	s.recordMetrics("create", withKV, result.CacheHit, start)
	if err != nil {
		status, msg := mapError(err)
		s.renderResponse(w, status, "<p>"+html.EscapeString(msg)+"</p>")
		return
	}

	s.renderResponse(w, http.StatusOK, "<section><h2>Created</h2>"+renderItemHTML(result.Item)+"</section>")
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}

	start := time.Now()
	withKV := parseWithKV(r)
	idStr := strings.TrimSpace(r.URL.Query().Get("id"))
	key := strings.TrimSpace(r.URL.Query().Get("key"))

	var result OpResult
	var err error

	switch {
	case idStr != "" && key != "":
		id, parseErr := strconv.ParseInt(idStr, 10, 64)
		if parseErr != nil {
			writeHTML(w, http.StatusBadRequest, "<p>invalid id</p>")
			return
		}
		result, err = s.service.ReadByIDAndKey(r.Context(), withKV, id, key)
	case idStr != "":
		id, parseErr := strconv.ParseInt(idStr, 10, 64)
		if parseErr != nil {
			writeHTML(w, http.StatusBadRequest, "<p>invalid id</p>")
			return
		}
		result, err = s.service.ReadByID(r.Context(), withKV, id)
	case key != "":
		result, err = s.service.ReadByKey(r.Context(), withKV, key)
	default:
		writeHTML(w, http.StatusBadRequest, "<p>id or key is required</p>")
		return
	}

	s.recordMetrics("read", withKV, result.CacheHit, start)
	if err != nil {
		status, msg := mapError(err)
		s.renderResponse(w, status, "<p>"+html.EscapeString(msg)+"</p>")
		return
	}

	s.renderResponse(w, http.StatusOK, "<section><h2>Read</h2>"+renderItemHTML(result.Item)+"</section>")
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid form</p>")
		return
	}

	start := time.Now()
	withKV := parseWithKV(r)
	idStr := strings.TrimSpace(r.FormValue("id"))
	value := r.FormValue("value")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid id</p>")
		return
	}

	result, err := s.service.Update(r.Context(), withKV, id, value)
	s.recordMetrics("update", withKV, result.CacheHit, start)
	if err != nil {
		status, msg := mapError(err)
		s.renderResponse(w, status, "<p>"+html.EscapeString(msg)+"</p>")
		return
	}

	s.renderResponse(w, http.StatusOK, "<section><h2>Updated</h2>"+renderItemHTML(result.Item)+"</section>")
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid form</p>")
		return
	}

	start := time.Now()
	withKV := parseWithKV(r)
	hard := parseHardDelete(r)
	idStr := strings.TrimSpace(r.FormValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid id</p>")
		return
	}

	err = s.service.Delete(r.Context(), withKV, id, hard)
	s.recordMetrics("delete", withKV, nil, start)
	if err != nil {
		status, msg := mapError(err)
		s.renderResponse(w, status, "<p>"+html.EscapeString(msg)+"</p>")
		return
	}

	kind := "soft deleted"
	if hard {
		kind = "hard deleted"
	}
	s.renderResponse(w, http.StatusOK, fmt.Sprintf("<section><h2>Deleted</h2><p>id=%d %s</p></section>", id, kind))
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTML(w, http.StatusMethodNotAllowed, "<p>method not allowed</p>")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeHTML(w, http.StatusBadRequest, "<p>invalid form</p>")
		return
	}

	start := time.Now()
	err := s.service.ClearCache(r.Context())
	s.recordMetrics("clear_cache", false, nil, start)
	if err != nil {
		status, msg := mapError(err)
		s.renderResponse(w, status, "<p>"+html.EscapeString(msg)+"</p>")
		return
	}

	s.renderResponse(w, http.StatusOK, "<section><h2>Cache cleared</h2></section>")
}
