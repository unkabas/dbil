package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSetup_WritesJSON(t *testing.T) {
	var buf bytes.Buffer
	l := Setup(slog.LevelInfo, true, &buf)
	l.Info("hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("expected JSON log line, got %q", out)
	}
	if !strings.Contains(out, `"k":"v"`) {
		t.Fatalf("expected key/value pair, got %q", out)
	}
}

func TestSetup_WritesText(t *testing.T) {
	var buf bytes.Buffer
	l := Setup(slog.LevelInfo, false, &buf)
	l.Info("hello", "k", "v")
	out := buf.String()
	if strings.HasPrefix(out, `{`) {
		t.Fatalf("expected text format, got JSON-like %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected message in output, got %q", out)
	}
}

func TestSetup_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := Setup(slog.LevelWarn, false, &buf)
	l.Info("should be filtered")
	if buf.Len() != 0 {
		t.Fatalf("Info should be filtered at Warn level, got %q", buf.String())
	}
	l.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Fatalf("Warn line missing")
	}
}

func TestRedact_KeyValueSecrets(t *testing.T) {
	cases := map[string]string{
		"password=hunter2":   "hunter2",
		"Password: hunter2":  "hunter2",
		"secret=abcd1234":    "abcd1234",
		"api_key=k_abcdefg":  "k_abcdefg",
		"API-KEY: k_abcdefg": "k_abcdefg",
		"token=t.123.xyz":    "t.123.xyz",
	}
	for input, secret := range cases {
		t.Run(input, func(t *testing.T) {
			out := Redact(input)
			if strings.Contains(out, secret) {
				t.Errorf("secret %q leaked into %q", secret, out)
			}
			if !strings.Contains(out, "<redacted>") {
				t.Errorf("missing <redacted> in %q", out)
			}
		})
	}
}

func TestRedact_BearerToken(t *testing.T) {
	out := Redact("Bearer abc.def.ghi")
	if strings.Contains(out, "abc.def.ghi") {
		t.Fatalf("bearer token leaked: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("missing <redacted> in %q", out)
	}
}

func TestRedact_PassesThroughInnocuous(t *testing.T) {
	in := "a perfectly normal log line with nothing sensitive in it"
	if got := Redact(in); got != in {
		t.Fatalf("Redact altered innocuous input: %q -> %q", in, got)
	}
}
