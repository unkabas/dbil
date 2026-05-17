// Package log configures DBil's structured logging on top of log/slog and
// provides a best-effort redaction helper for credential-looking log lines.
package log

import (
	"io"
	"log/slog"
	"os"
	"regexp"
)

// Setup configures slog defaults. When jsonOutput is true, output uses the
// JSON handler; otherwise the human-readable text handler. Returns the
// configured logger and also installs it via slog.SetDefault so packages that
// already use slog.Info/Warn/Error pick it up.
func Setup(level slog.Level, jsonOutput bool, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if jsonOutput {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

// InContainer reports whether the current process appears to be running
// inside a Docker container (detected via the /.dockerenv marker). Useful
// for picking a default log format (JSON in container, text on a TTY).
func InContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

var (
	pwLike     = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api[_-]?key)\s*[:=]\s*)\S+`)
	bearerLike = regexp.MustCompile(`(?i)(bearer\s+)\S+`)
)

// Redact masks credential-looking substrings inside s. This is best-effort —
// it catches the common shapes (password=, secret=, api_key=, "Bearer xyz")
// but is not a security boundary. Sensitive values should not be logged in
// the first place; Redact is the seatbelt, not the safety system.
func Redact(s string) string {
	s = bearerLike.ReplaceAllString(s, "$1<redacted>")
	s = pwLike.ReplaceAllString(s, "$1<redacted>")
	return s
}
