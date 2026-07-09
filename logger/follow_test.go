package logger

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		level     LevelT
		sessionId string
		file      string
		line      int
		topic     string
		message   string
	}{
		{
			name:    "ndjson info line",
			input:   `{"timestamp_ms":1774705804511,"timestamp":"2026/03/28 13:50:04.511","level":"INFO","file":"session_table.go","line":97,"msg":"Session created"}`,
			level:   INFO,
			file:    "session_table.go",
			line:    97,
			message: "Session created",
		},
		{
			name:      "ndjson with session id",
			input:     `{"timestamp_ms":1774705804511,"timestamp":"2026/03/28 13:50:04.511","level":"ERROR","sid":"happy-panda","file":"server.go","line":42,"msg":"something went wrong"}`,
			level:     ERROR,
			sessionId: "happy-panda",
			file:      "server.go",
			line:      42,
			message:   "something went wrong",
		},
		{
			name:    "ndjson debug line",
			input:   `{"timestamp_ms":1774705804511,"timestamp":"2026/03/28 13:50:04.511","level":"DEBUG","file":"websocket.go","line":55,"msg":"client connected"}`,
			level:   DEBUG,
			file:    "websocket.go",
			line:    55,
			message: "client connected",
		},
		{
			name:    "ndjson with topic",
			input:   `{"timestamp_ms":1774705804511,"timestamp":"2026/03/28 13:50:04.511","level":"WARN","file":"handler.go","line":10,"topic":"http","msg":"slow request"}`,
			level:   WARN,
			file:    "handler.go",
			line:    10,
			topic:   "http",
			message: "slow request",
		},
		{
			name:    "legacy text format returns nil",
			input:   `2026/03/28 13:50:04.511725 INFO: session_table.go:97 Session "fine-mammoth" created`,
			wantNil: true,
		},
		{
			name:    "invalid line",
			input:   "not a log line",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parseLogLine(tt.input)
			if tt.wantNil {
				if parsed != nil {
					t.Fatalf("expected nil, got %+v", parsed)
				}
				return
			}
			if parsed == nil {
				t.Fatal("expected non-nil")
			}
			if parsed.Level != tt.level {
				t.Errorf("level: got %v, want %v", parsed.Level, tt.level)
			}
			if parsed.SessionId != tt.sessionId {
				t.Errorf("sessionId: got %q, want %q", parsed.SessionId, tt.sessionId)
			}
			if parsed.File != tt.file {
				t.Errorf("file: got %q, want %q", parsed.File, tt.file)
			}
			if parsed.Line != tt.line {
				t.Errorf("line: got %d, want %d", parsed.Line, tt.line)
			}
			if parsed.Topic != tt.topic {
				t.Errorf("topic: got %q, want %q", parsed.Topic, tt.topic)
			}
			if tt.message != "" && parsed.Message != tt.message {
				t.Errorf("message: got %q, want %q", parsed.Message, tt.message)
			}
		})
	}
}

func TestFormatRelativeTimestamp(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "+00.000"},
		{1500 * time.Millisecond, "+01.500"},
		{65 * time.Second, "+01:05.000"},
		{3661500 * time.Millisecond, "+1:01:01.500"},
	}

	for _, tt := range tests {
		got := formatRelativeTimestamp(tt.d)
		if got != tt.want {
			t.Errorf("formatRelativeTimestamp(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatFollowLine(t *testing.T) {
	startTime := time.Date(2026, 3, 28, 13, 50, 0, 0, time.UTC)

	p := &parsedLogEntry{
		Timestamp: startTime.Add(5 * time.Second),
		Level:     INFO,
		File:      "server.go",
		Line:      42,
		Message:   "started",
	}

	result := formatLogLine(p, startTime, noColor)
	if result != "+05.000 INFO: server.go:42 started" {
		t.Errorf("unexpected format: %q", result)
	}
}

func TestFormatFollowLineWithTopic(t *testing.T) {
	startTime := time.Date(2026, 3, 28, 13, 50, 0, 0, time.UTC)

	p := &parsedLogEntry{
		Timestamp: startTime.Add(1 * time.Second),
		Level:     WARN,
		File:      "handler.go",
		Line:      10,
		Topic:     "http",
		Message:   "slow request",
	}

	result := formatLogLine(p, startTime, noColor)
	if result != "+01.000 WARN: handler.go:10 [http]: slow request" {
		t.Errorf("unexpected format: %q", result)
	}
}

func TestFormatFollowLineTruncation(t *testing.T) {
	startTime := time.Date(2026, 3, 28, 13, 50, 0, 0, time.UTC)

	longMsg := strings.Repeat("x", 300)
	recLen := 50
	p := &parsedLogEntry{
		Timestamp:      startTime,
		Level:          INFO,
		File:           "test.go",
		Line:           1,
		Message:        longMsg,
		RecommendedLen: &recLen,
	}

	result := formatLogLine(p, startTime, noColor)
	// 50 chars + "..."
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected truncated message ending with ..., got: %q", result)
	}
	// The message part should be 53 chars (50 + "...")
	parts := strings.SplitN(result, "test.go:1 ", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected format: %q", result)
	}
	if len(parts[1]) != 53 {
		t.Errorf("expected truncated message length 53, got %d: %q", len(parts[1]), parts[1])
	}
}

func TestFormatFollowLineNoSource(t *testing.T) {
	startTime := time.Date(2026, 3, 28, 13, 50, 0, 0, time.UTC)

	p := &parsedLogEntry{
		Timestamp: startTime,
		Level:     INFO,
		Message:   "no formatSource",
	}

	result := formatLogLine(p, startTime, noColor)
	if result != "+00.000 INFO: no formatSource" {
		t.Errorf("unexpected format: %q", result)
	}
}

func TestParseNDJSONWithRecommendedLen(t *testing.T) {
	line := `{"timestamp_ms":1774705804511,"timestamp":"2026/03/28 13:50:04.511","level":"INFO","file":"test.go","line":1,"msg":"hello","recommendedMsgLen":100}`
	parsed := parseLogLine(line)
	if parsed == nil {
		t.Fatal("expected non-nil")
	}
	if parsed.RecommendedLen == nil || *parsed.RecommendedLen != 100 {
		t.Errorf("recommendedLen: got %v, want 100", parsed.RecommendedLen)
	}
}

func TestFormatFileHomebrewGo(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/opt/homebrew/Cellar/go/1.24.4/libexec/src/net/http/server.go", "$HOMEBREW_GO/libexec/src/net/http/server.go"},
		{"/opt/homebrew/Cellar/go/1.23.0/libexec/src/fmt/print.go", "$HOMEBREW_GO/libexec/src/fmt/print.go"},
		{"/opt/homebrew/Cellar/go/2.0.0-rc1/libexec/src/io/io.go", "$HOMEBREW_GO/libexec/src/io/io.go"},
		{"/some/other/path/server.go", "/some/other/path/server.go"},
	}
	for _, tt := range tests {
		got := formatFile(tt.input)
		if got != tt.want {
			t.Errorf("formatFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseNDJSONInvalidJSON(t *testing.T) {
	parsed := parseLogLine(`{invalid json}`)
	if parsed != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", parsed)
	}
}

func TestParseNDJSONMissingFields(t *testing.T) {
	parsed := parseLogLine(`{"msg":"hello"}`)
	if parsed != nil {
		t.Errorf("expected nil for JSON without ts/level, got %+v", parsed)
	}
}
