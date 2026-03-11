package endpoint

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type ZapFormatter struct {
	Logger *zap.Logger
}

func (z *ZapFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &zapLogEntry{
		logger: z.Logger.With(
			zap.String("method", r.Method),
			zap.String("url", r.RequestURI),
		),
	}
}

// zapLogEntry implements middleware.LogEntry
type zapLogEntry struct {
	logger *zap.Logger
}

func (l *zapLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.logger.Info(
		"request completed",
		zap.Int("status", status), zap.Int("bytes", bytes),
		zap.Int64("elapsed_us", elapsed.Microseconds()),
	)
}

func (l *zapLogEntry) Panic(v any, stack []byte) {
	l.logger.Error("panic", zap.Any("panic", v), zap.String("stack", string(stack)))
}
