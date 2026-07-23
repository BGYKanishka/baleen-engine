package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

type ideaHandler struct {
	out   io.Writer
	level slog.Level
	mu    *sync.Mutex
	attrs []slog.Attr
}

func (h *ideaHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *ideaHandler) Handle(_ context.Context, r slog.Record) error {
	timeStr := r.Time.Format("2006-01-02 15:04:05.000")
	levelStr := r.Level.String()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Print base format similar to IntelliJ
	fmt.Fprintf(h.out, "%s %-5s - %s", timeStr, levelStr, r.Message)

	// Print attributes
	for _, a := range h.attrs {
		fmt.Fprintf(h.out, " [%s=%v]", a.Key, a.Value.Any())
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.out, " [%s=%v]", a.Key, a.Value.Any())
		return true
	})
	fmt.Fprintln(h.out)

	return nil
}

func (h *ideaHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr{}, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &ideaHandler{out: h.out, level: h.level, mu: h.mu, attrs: newAttrs}
}

func (h *ideaHandler) WithGroup(name string) slog.Handler {
	return h
}

func InitLogger(isDaemon bool, out io.Writer) {
	handler := &ideaHandler{
		out:   out,
		level: slog.LevelInfo,
		mu:    &sync.Mutex{},
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
