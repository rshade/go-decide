package clientkit

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/kataras/jev"
)

// Redacted replaces a credential wherever it would otherwise be printed.
const Redacted = "[REDACTED]"

// NewLogger mirrors how the client library picks a logger from the level in
// environment variable levelEnv, but redacts key from every string attribute. The library
// redacts request headers only, so a server that echoes the key in a response
// body would otherwise leak it at debug level. It returns nil when logging is
// off.
func NewLogger(out io.Writer, levelEnv, key string) (*slog.Logger, error) {
	raw := strings.TrimSpace(os.Getenv(levelEnv))
	if raw == "" || strings.EqualFold(raw, "off") {
		return nil, nil
	}
	level, err := parseLogLevel(levelEnv, raw)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: level}
	if key != "" {
		opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() == slog.KindString {
				a.Value = slog.StringValue(strings.ReplaceAll(a.Value.String(), key, Redacted))
			}
			return a
		}
	}
	return slog.New(slog.NewTextHandler(out, opts)), nil
}

func parseLogLevel(levelEnv, raw string) (slog.Level, error) {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %s must be debug, info, warn, error, or off", jev.ErrConfig, levelEnv)
	}
}
