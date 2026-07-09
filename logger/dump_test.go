package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestLog writes the given ndjson lines to a temp file and points the
// package-level log path at it, returning the file path.
func writeTestLog(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "critic.ndjson.log")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	sharedDest.mu.Lock()
	logFilePath = path
	sharedDest.mu.Unlock()
	return path
}

func logLine(tsMs int64, level, msg string) string {
	return fmt.Sprintf(`{"timestamp_ms":%d,"timestamp":"x","level":%q,"file":"server.go","line":1,"msg":%q}`, tsMs, level, msg)
}

func TestDumpFiltersByLevel(t *testing.T) {
	writeTestLog(t, []string{
		logLine(1000, "INFO", "first info"),
		logLine(1001, "WARN", "a warning"),
		logLine(1002, "INFO", "second info"),
	})

	var sb strings.Builder
	dumpTo(&sb, WARN, 0, noColor)
	out := sb.String()

	if !strings.Contains(out, "a warning") {
		t.Errorf("expected warning in output, got:\n%s", out)
	}
	if strings.Contains(out, "first info") || strings.Contains(out, "second info") {
		t.Errorf("INFO lines should be filtered out at WARN level, got:\n%s", out)
	}
}

func TestDumpWholeFile(t *testing.T) {
	writeTestLog(t, []string{
		logLine(1000, "INFO", "alpha"),
		logLine(1001, "INFO", "beta"),
	})

	var sb strings.Builder
	dumpTo(&sb, INFO, 0, noColor)
	out := sb.String()

	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("expected both lines, got:\n%s", out)
	}
}

func TestDumpLastBytesSkipsPartialLine(t *testing.T) {
	lines := []string{
		logLine(1000, "INFO", "oldest entry that should be dropped"),
		logLine(2000, "INFO", "newest entry"),
	}
	writeTestLog(t, lines)

	// Choose a window smaller than the full file so the seek lands inside
	// the first line; the partial first line must be discarded, leaving the
	// last complete line intact.
	full := len(strings.Join(lines, "\n")) + 1
	window := int64(len(lines[1]) + 5)
	if window >= int64(full) {
		t.Fatalf("test setup: window %d not smaller than file %d", window, full)
	}

	var sb strings.Builder
	dumpTo(&sb, INFO, window, noColor)
	out := sb.String()

	if !strings.Contains(out, "newest entry") {
		t.Errorf("expected newest entry in windowed dump, got:\n%s", out)
	}
	if strings.Contains(out, "oldest entry") {
		t.Errorf("partial first line should have been skipped, got:\n%s", out)
	}
}

func TestDumpRelativeTimestampAnchored(t *testing.T) {
	writeTestLog(t, []string{
		logLine(1000, "INFO", "anchor"),
		logLine(1500, "INFO", "later"),
	})

	var sb strings.Builder
	dumpTo(&sb, INFO, 0, noColor)
	out := sb.String()

	// First entry anchors the window at +00.000; the second is 500ms later.
	if !strings.Contains(out, "+00.000") {
		t.Errorf("expected first entry anchored at +00.000, got:\n%s", out)
	}
	if !strings.Contains(out, "+00.500") {
		t.Errorf("expected second entry at +00.500, got:\n%s", out)
	}
}
