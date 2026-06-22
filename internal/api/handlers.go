package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
)

const maxReadCount = 10000

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

func parseCount(r *http.Request) (int, error) {
	v := strings.TrimSpace(r.FormValue("count"))
	if v == "" {
		return 1, nil
	}

	count, err := strconv.Atoi(v)
	if err != nil || count < 1 {
		return 0, fmt.Errorf("invalid count")
	}
	if count > maxReadCount {
		return 0, fmt.Errorf("count exceeds %d", maxReadCount)
	}
	return count, nil
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

func (s *Server) recordMetrics(op string, withKV bool, cacheHit *bool, start time.Time, success bool) {
	s.recordMetricsEntry(Entry{
		At:         time.Now(),
		Op:         op,
		LatencyMs:  time.Since(start).Milliseconds(),
		WithKV:     withKV,
		CacheHit:   cacheHit,
		BatchCount: 1,
		Success:    success,
	})
}

func (s *Server) recordMetricsEntry(entry Entry) {
	s.metrics.Record(entry)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.render(w, http.StatusOK, "index.html", s.metrics.Snapshot())
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	if err := r.ParseForm(); err != nil {
		s.recordMetrics("create", false, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid form"})
		return
	}

	withKV := parseWithKV(r)
	key := strings.TrimSpace(r.FormValue("key"))
	value := r.FormValue("value")
	if key == "" {
		s.recordMetrics("create", withKV, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "key is required"})
		return
	}

	result, err := s.service.Create(r.Context(), withKV, key, value)
	if err != nil {
		s.recordMetrics("create", withKV, nil, start, false)
		status, msg := mapError(err)
		s.renderHTMXResponse(w, status, ResultData{Error: msg})
		return
	}

	s.recordMetrics("create", withKV, result.CacheHit, start, true)
	item := result.Item
	s.renderHTMXResponse(w, http.StatusOK, ResultData{
		Title: "Created",
		Item:  &item,
	})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	withKV := parseWithKV(r)

	count, err := parseCount(r)
	if err != nil {
		s.recordMetrics("read", withKV, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: err.Error()})
		return
	}

	idStr := strings.TrimSpace(r.URL.Query().Get("id"))
	key := strings.TrimSpace(r.URL.Query().Get("key"))

	var lastResult OpResult
	var hits, misses int

	for i := 0; i < count; i++ {
		var result OpResult
		var readErr error

		switch {
		case idStr != "" && key != "":
			id, parseErr := strconv.ParseInt(idStr, 10, 64)
			if parseErr != nil {
				s.recordMetricsEntry(Entry{
					At:         time.Now(),
					Op:         "read",
					LatencyMs:  time.Since(start).Milliseconds(),
					WithKV:     withKV,
					BatchCount: count,
					Success:    false,
				})
				s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid id"})
				return
			}
			result, readErr = s.service.ReadByIDAndKey(r.Context(), withKV, id, key)
		case idStr != "":
			id, parseErr := strconv.ParseInt(idStr, 10, 64)
			if parseErr != nil {
				s.recordMetricsEntry(Entry{
					At:         time.Now(),
					Op:         "read",
					LatencyMs:  time.Since(start).Milliseconds(),
					WithKV:     withKV,
					BatchCount: count,
					Success:    false,
				})
				s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid id"})
				return
			}
			result, readErr = s.service.ReadByID(r.Context(), withKV, id)
		case key != "":
			result, readErr = s.service.ReadByKey(r.Context(), withKV, key)
		default:
			s.recordMetricsEntry(Entry{
				At:         time.Now(),
				Op:         "read",
				LatencyMs:  time.Since(start).Milliseconds(),
				WithKV:     withKV,
				BatchCount: count,
				Success:    false,
			})
			s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "id or key is required"})
			return
		}

		if readErr != nil {
			totalMs := time.Since(start).Milliseconds()
			s.recordMetricsEntry(Entry{
				At:          time.Now(),
				Op:          "read",
				LatencyMs:   totalMs,
				WithKV:      withKV,
				BatchCount:  count,
				CacheHits:   hits,
				CacheMisses: misses,
				Success:     false,
			})
			status, msg := mapError(readErr)
			s.renderHTMXResponse(w, status, ResultData{Error: msg})
			return
		}

		if withKV && result.CacheHit != nil {
			if *result.CacheHit {
				hits++
			} else {
				misses++
			}
		}
		lastResult = result
	}

	totalMs := time.Since(start).Milliseconds()
	entry := Entry{
		At:         time.Now(),
		Op:         "read",
		LatencyMs:  totalMs,
		WithKV:     withKV,
		BatchCount: count,
		Success:    true,
	}
	if count == 1 && withKV {
		entry.CacheHit = lastResult.CacheHit
	}
	if withKV && count > 1 {
		entry.CacheHits = hits
		entry.CacheMisses = misses
	}
	s.recordMetricsEntry(entry)

	item := lastResult.Item
	resultData := ResultData{
		Title: "Read",
		Item:  &item,
	}
	if count > 1 {
		resultData.BatchCount = count
		resultData.TotalMs = totalMs
		resultData.AvgMs = float64(totalMs) / float64(count)
		resultData.CacheHits = hits
		resultData.CacheMisses = misses
	}
	s.renderHTMXResponse(w, http.StatusOK, resultData)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	if err := r.ParseForm(); err != nil {
		s.recordMetrics("update", false, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid form"})
		return
	}

	withKV := parseWithKV(r)
	idStr := strings.TrimSpace(r.FormValue("id"))
	value := r.FormValue("value")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.recordMetrics("update", withKV, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid id"})
		return
	}

	result, err := s.service.Update(r.Context(), withKV, id, value)
	if err != nil {
		s.recordMetrics("update", withKV, nil, start, false)
		status, msg := mapError(err)
		s.renderHTMXResponse(w, status, ResultData{Error: msg})
		return
	}

	s.recordMetrics("update", withKV, result.CacheHit, start, true)
	item := result.Item
	s.renderHTMXResponse(w, http.StatusOK, ResultData{
		Title: "Updated",
		Item:  &item,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	if err := r.ParseForm(); err != nil {
		s.recordMetrics("delete", false, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid form"})
		return
	}

	withKV := parseWithKV(r)
	hard := parseHardDelete(r)
	idStr := strings.TrimSpace(r.FormValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.recordMetrics("delete", withKV, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid id"})
		return
	}

	err = s.service.Delete(r.Context(), withKV, id, hard)
	if err != nil {
		s.recordMetrics("delete", withKV, nil, start, false)
		status, msg := mapError(err)
		s.renderHTMXResponse(w, status, ResultData{Error: msg})
		return
	}

	s.recordMetrics("delete", withKV, nil, start, true)
	kind := fmt.Sprintf("id=%d soft deleted", id)
	if hard {
		kind = fmt.Sprintf("id=%d hard deleted", id)
	}
	s.renderHTMXResponse(w, http.StatusOK, ResultData{
		Title: "Deleted",
		Extra: kind,
	})
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	if err := r.ParseForm(); err != nil {
		s.recordMetrics("clear_cache", false, nil, start, false)
		s.renderHTMXResponse(w, http.StatusBadRequest, ResultData{Error: "invalid form"})
		return
	}

	err := s.service.ClearCache(r.Context())
	if err != nil {
		s.recordMetrics("clear_cache", false, nil, start, false)
		status, msg := mapError(err)
		s.renderHTMXResponse(w, status, ResultData{Error: msg})
		return
	}

	s.recordMetrics("clear_cache", false, nil, start, true)
	s.renderHTMXResponse(w, http.StatusOK, ResultData{Title: "Cache cleared"})
}
