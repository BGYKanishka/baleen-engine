package logger

import (
	"io"
	"log/slog"
)

// InitLogger configures the global slog instance based on the application mode.
// Daemon mode emits structured JSON, while CLI mode emits human-readable text logs.
func InitLogger(isDaemon bool, out io.Writer) {
	var handler slog.Handler

	if isDaemon {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(out, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
