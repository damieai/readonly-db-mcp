package audit

import (
	"context"
	"log/slog"
	"time"
)

type Event struct {
	QueryID     string
	Target      string
	Operation   string
	Fingerprint string
	Tables      []string
	Decision    string
	Reason      string
	Rows        int
	Truncated   bool
	Duration    time.Duration
}

type Auditor interface {
	Record(context.Context, Event)
}

type SlogAuditor struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *SlogAuditor {
	return &SlogAuditor{logger: logger}
}

func (a *SlogAuditor) Record(_ context.Context, event Event) {
	a.logger.Info("database tool audit",
		"query_id", event.QueryID,
		"target", event.Target,
		"operation", event.Operation,
		"fingerprint", event.Fingerprint,
		"tables", event.Tables,
		"decision", event.Decision,
		"reason", event.Reason,
		"rows", event.Rows,
		"truncated", event.Truncated,
		"duration_ms", event.Duration.Milliseconds(),
	)
}
