package api

import (
	"sync"
	"time"
)

const maxRecentEntries = 20

type Entry struct {
	At          time.Time
	Op          string
	LatencyMs   int64
	WithKV      bool
	CacheHit    *bool
	BatchCount  int
	CacheHits   int
	CacheMisses int
	Success     bool
}

type MetricsView struct {
	Last         Entry
	TotalCount   int
	AvgLatencyMs int64
	Recent       []Entry
}

type Metrics struct {
	mu             sync.Mutex
	last           Entry
	totalCount     int
	totalLatencyMs int64
	recent         []Entry
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) Record(e Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e.BatchCount == 0 {
		e.BatchCount = 1
	}

	m.last = e
	m.totalCount++
	m.totalLatencyMs += e.LatencyMs

	m.recent = append(m.recent, e)
	if len(m.recent) > maxRecentEntries {
		m.recent = m.recent[len(m.recent)-maxRecentEntries:]
	}
}

func (m *Metrics) Snapshot() MetricsView {
	m.mu.Lock()
	defer m.mu.Unlock()

	recent := make([]Entry, len(m.recent))
	copy(recent, m.recent)

	var avg int64
	if m.totalCount > 0 {
		avg = m.totalLatencyMs / int64(m.totalCount)
	}

	return MetricsView{
		Last:         m.last,
		TotalCount:   m.totalCount,
		AvgLatencyMs: avg,
		Recent:       recent,
	}
}
