package temporal

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/log"
)

// slogLogger adapts log/slog to Temporal's log.Logger so the SDK emits the same
// structured JSON as the rest of the platform.
type slogLogger struct{ l *slog.Logger }

func newSlogLogger(l *slog.Logger) log.Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogLogger{l: l}
}

func (s *slogLogger) Debug(msg string, kv ...any) {
	s.l.LogAttrs(context.Background(), slog.LevelDebug, msg, group(kv))
}
func (s *slogLogger) Info(msg string, kv ...any) {
	s.l.LogAttrs(context.Background(), slog.LevelInfo, msg, group(kv))
}
func (s *slogLogger) Warn(msg string, kv ...any) {
	s.l.LogAttrs(context.Background(), slog.LevelWarn, msg, group(kv))
}
func (s *slogLogger) Error(msg string, kv ...any) {
	s.l.LogAttrs(context.Background(), slog.LevelError, msg, group(kv))
}

// group folds Temporal's variadic key/value pairs under a "temporal" attribute so
// they never collide with our own log fields.
func group(kv []any) slog.Attr {
	attrs := make([]any, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		attrs = append(attrs, slog.Any(key, kv[i+1]))
	}
	return slog.Group("temporal", attrs...)
}
