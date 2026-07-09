package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/radiospiel/simple-go/src/ansi"
	"golang.org/x/term"
)

// homebrewGoRe matches Homebrew's Go install path with any version (e.g. "1.24.4", "2.0.0-rc1").
var homebrewGoRe = regexp.MustCompile(`^/opt/homebrew/Cellar/go/[\d.][\w.\-]*/`)

var (
	// wd is the working directory at process start, used to shorten file paths.
	wd   = (func() string { dir, _ := os.Getwd(); return dir })()
	home = (func() string { dir, _ := os.UserHomeDir(); return dir })()
)

// parsedLogEntry holds the parsed components of a log line, as read from the log file
type parsedLogEntry struct {
	Timestamp      time.Time
	Level          LevelT
	SessionId      string
	File           string // e.g. "/abs/path/server.go"
	Line           int    // e.g. 42
	Topic          string
	Message        string
	RecommendedLen *int   // recommended max message length; nil = use default
	Raw            string // original line
}

func formatFile(file string) string {
	if strings.HasPrefix(file, wd+"/") {
		file = file[len(wd)+1:]
	} else if home != "" && strings.HasPrefix(file, home+"/") {
		file = "~/" + file[len(home)+1:]
	}
	file = homebrewGoRe.ReplaceAllLiteralString(file, "$HOMEBREW_GO/")
	return file
}

// parseLogLevel converts a level string to LevelT.
func parseLogLevel(s string) LevelT {
	switch s {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "SUCCESS":
		return SUCCESS
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// parseLogLine parses a single ndjson log line into its components.
func parseLogLine(line string) *parsedLogEntry {
	line = strings.TrimRight(line, "\n\r")
	if line == "" {
		return nil
	}

	var entry ndjsonEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil
	}
	if entry.TimestampMs == 0 || entry.Level == "" {
		return nil
	}

	ts := time.UnixMilli(entry.TimestampMs)

	return &parsedLogEntry{
		Timestamp:      ts,
		Level:          parseLogLevel(entry.Level),
		SessionId:      entry.SessionID,
		File:           entry.File,
		Line:           entry.Line,
		Topic:          entry.Topic,
		Message:        entry.Message,
		RecommendedLen: entry.RecommendedLen,
		Raw:            line,
	}
}

// formatRelativeTimestamp formats a duration as +[[HH:]MM:]SS.millis
func formatRelativeTimestamp(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalMs := d.Milliseconds()
	millis := totalMs % 1000
	totalSecs := totalMs / 1000
	secs := totalSecs % 60
	totalMins := totalSecs / 60
	mins := totalMins % 60
	hours := totalMins / 60

	if hours > 0 {
		return fmt.Sprintf("+%d:%02d:%02d.%03d", hours, mins, secs, millis)
	} else if mins > 0 {
		return fmt.Sprintf("+%02d:%02d.%03d", mins, secs, millis)
	} else {
		return fmt.Sprintf("+%02d.%03d", secs, millis)
	}
}

// ColorizeFunc wraps a string in the given ANSI color code.
// Receives the ANSI escape (e.g. "\033[33m") and the text to wrap.
type ColorizeFunc func(color, str string) string

// noColor returns the string unchanged (no ANSI escapes).
func noColor(_, str string) string { return str }

// formatLogLine formats a parsed log line for follow-log output with
// relative timestamps and reapplied colors.
func formatLogLine(p *parsedLogEntry, startTime time.Time, colorize ColorizeFunc) string {
	relTs := formatRelativeTimestamp(p.Timestamp.Sub(startTime))

	var sb strings.Builder

	// Timestamp
	sb.WriteString(relTs)
	sb.WriteString(" ")

	// Level with color
	levelStr := p.Level.String()
	lvlColor := levelColor(p.Level, "")
	sb.WriteString(colorize(lvlColor, levelStr))
	sb.WriteString(": ")

	// Source file (colored yellow)
	if p.File != "" && p.Line != 0 {
		source := fmt.Sprintf("%s:%d", formatFile(p.File), p.Line)
		sb.WriteString(colorize(ansi.ColorYellow, source))
		sb.WriteString(" ")
	}

	// Build display message: prepend topic, then truncate.
	// RecommendedLen: nil = use default, non-nil = use that value.
	msg := p.Message
	if p.Topic != "" {
		msg = "[" + p.Topic + "]: " + msg
	}
	maxLen := defaultMaxLogLength
	if p.RecommendedLen != nil {
		maxLen = *p.RecommendedLen
	}
	msg = truncate(msg, maxLen)

	// Wrap the entire tail (level onward) in the level color
	sb.WriteString(colorize(lvlColor, msg))

	return sb.String()
}

// Dump writes the current log file to out, filtered by minimum level, and
// returns without following. When lastBytes > 0 only the final lastBytes of
// the file are scanned; the first (likely partial) line of that window is
// discarded so parsing starts on a record boundary. Relative timestamps are
// anchored to the first entry in the scanned window.
//
// This is the non-interactive counterpart to Follow: it terminates instead
// of tailing, which makes it usable from scripts and agents that just want
// to read the recent log.
func Dump(out *os.File, level LevelT, lastBytes int64) {
	colorize := noColor
	if term.IsTerminal(int(out.Fd())) {
		colorize = ansi.Colorize
	}
	dumpTo(out, level, lastBytes, colorize)
}

// dumpTo is the io.Writer-based core of Dump, split out for testing.
func dumpTo(out io.Writer, level LevelT, lastBytes int64, colorize ColorizeFunc) {
	logFile := GetLogFile()

	f, err := os.Open(logFile)
	if err != nil {
		Warn("Dump: failed to open log file %s: %v", logFile, err)
		return
	}
	defer f.Close()

	skipPartial := false
	if lastBytes > 0 {
		size, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			Warn("Dump: failed to seek to end: %v", err)
			return
		}
		if lastBytes < size {
			if _, err := f.Seek(size-lastBytes, io.SeekStart); err != nil {
				Warn("Dump: failed to seek: %v", err)
				return
			}
			skipPartial = true
		} else if _, err := f.Seek(0, io.SeekStart); err != nil {
			Warn("Dump: failed to seek to start: %v", err)
			return
		}
	}

	reader := bufio.NewReader(f)
	if skipPartial {
		// Discard the first partial line so parsing starts on a boundary.
		if _, err := reader.ReadString('\n'); err != nil {
			return
		}
	}

	writer := bufio.NewWriter(out)
	defer writer.Flush()

	var startTime time.Time
	haveStart := false

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if parsed := parseLogLine(line); parsed != nil {
				if !haveStart {
					startTime = parsed.Timestamp
					haveStart = true
				}
				if parsed.Level >= level {
					fmt.Fprintln(writer, formatLogLine(parsed, startTime, colorize))
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// FollowMode controls when Follow stops tailing the log.
type FollowMode int

const (
	// UntilProcessTerminates stops following when a FATAL message is seen.
	UntilProcessTerminates FollowMode = iota
	// Forever keeps following regardless of log content.
	Forever
)

// Follow tails the current log file to the given writer, auto-detecting
// color support. Filters by minimum level. The mode controls when to stop.
func Follow(out *os.File, level LevelT, mode FollowMode) {
	var colorize ColorizeFunc
	if term.IsTerminal(int(out.Fd())) {
		colorize = ansi.Colorize
	} else {
		colorize = noColor
	}

	logFile := GetLogFile()

	f, err := os.Open(logFile)
	if err != nil {
		Warn("Follow: failed to open log file %s: %v", logFile, err)
		return
	}
	defer f.Close()

	// Seek to end to only follow new lines
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		Warn("Follow: failed to seek to end: %v", err)
		return
	}

	var startTime = time.Now()

	reader := bufio.NewReader(f)
	writer := bufio.NewWriter(out)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				writer.Flush()
				time.Sleep(100 * time.Millisecond)
				continue
			}
			Warn("Follow: error reading log file: %v", err)
			return
		}

		parsed := parseLogLine(line)
		if parsed == nil {
			continue
		}

		// only log for matching log level
		if parsed.Level < level {
			continue
		}

		formatted := formatLogLine(parsed, startTime, colorize)
		fmt.Fprintln(writer, formatted)
		writer.Flush()

		// Stop following when the server terminates (FATAL level).
		if mode == UntilProcessTerminates && parsed.Level >= FATAL {
			return
		}
	}
}
