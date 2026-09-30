package jevclient

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/kataras/jev"
)

// newLogger mirrors how the client library picks a logger from
// TYPESAFE_LOG_LEVEL, but redacts key from every string attribute. The library
// redacts request headers only, so a server that echoes the key in a response
// body would otherwise leak it at debug level. It returns nil when logging is
// off.
func newLogger(out io.Writer, key string) (*slog.Logger, error) {
	raw := strings.TrimSpace(os.Getenv("TYPESAFE_LOG_LEVEL"))
	if raw == "" || strings.EqualFold(raw, "off") {
		return nil, nil
	}
	level, err := parseLogLevel(raw)
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
				a.Value = slog.StringValue(strings.ReplaceAll(a.Value.String(), key, redacted))
			}
			return a
		}
	}
	return slog.New(slog.NewTextHandler(out, opts)), nil
}

func parseLogLevel(raw string) (slog.Level, error) {
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
		return 0, fmt.Errorf("%w: TYPESAFE_LOG_LEVEL must be debug, info, warn, error, or off", jev.ErrConfig)
	}
}
