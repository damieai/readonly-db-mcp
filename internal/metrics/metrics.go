package metrics

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type Recorder interface {
	Add(name string, delta int64, operation, outcome string)
	Observe(name string, duration time.Duration, operation string)
	Set(name string, value int64, operation string)
}

type Slog struct {
	mu        sync.Mutex
	logger    *slog.Logger
	counters  map[string]int64
	durations map[string]duration
}
type duration struct {
	Count      int64
	Total, Max time.Duration
}

func New(logger *slog.Logger) *Slog {
	return &Slog{logger: logger, counters: map[string]int64{}, durations: map[string]duration{}}
}
func key(name, operation, outcome string) string { return name + "|" + operation + "|" + outcome }
func (s *Slog) Add(name string, delta int64, operation, outcome string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.counters[key(name, operation, outcome)] += delta
	s.mu.Unlock()
}
func (s *Slog) Observe(name string, d time.Duration, operation string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	k := key(name, operation, "")
	v := s.durations[k]
	v.Count++
	v.Total += d
	if d > v.Max {
		v.Max = d
	}
	s.durations[k] = v
	s.mu.Unlock()
}
func (s *Slog) Set(name string, value int64, operation string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.counters[key(name, operation, "gauge")] = value
	s.mu.Unlock()
}
func (s *Slog) Run(ctx context.Context, interval time.Duration) {
	if s == nil || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flush()
		}
	}
}
func (s *Slog) flush() {
	s.mu.Lock()
	c := s.counters
	d := s.durations
	s.counters = map[string]int64{}
	s.durations = map[string]duration{}
	s.mu.Unlock()
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s.logger.Info("performance counter", "metric", k, "value", c[k])
	}
	keys = keys[:0]
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d[k]
		s.logger.Info("performance duration", "metric", k, "count", v.Count, "average_ms", (v.Total / time.Duration(v.Count)).Milliseconds(), "max_ms", v.Max.Milliseconds())
	}
}
