package logredact

import (
	"context"
	"log/slog"
)

type handler struct {
	next slog.Handler
}

// NewHandler wraps a slog handler so messages, attributes, groups, errors, and
// structured values are redacted before the underlying handler sees them.
func NewHandler(next slog.Handler) slog.Handler {
	if next == nil {
		panic("logredact: nil handler")
	}
	return &handler{next: next}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, RedactText(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(sanitizeAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for i := range attrs {
		clean[i] = sanitizeAttr(attrs[i])
	}
	return &handler{next: h.next.WithAttrs(clean)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{next: h.next.WithGroup(name)}
}
