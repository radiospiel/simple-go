package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/radiospiel/simple-go/src/ansi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// LevelT represents a log level
type LevelT int

const (
	DEBUG LevelT = iota
	INFO
	WARN
	ERROR
	SUCCESS
	FATAL
)

var levelNames = [...]string{
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	WARN:    "WARN",
	ERROR:   "ERROR",
	SUCCESS: "SUCCESS",
	FATAL:   "FATAL",
}

func (l LevelT) String() string {
	return levelNames[l]
}

var levelColors = [...]string{
	DEBUG:   ansi.ColorCyan,
	INFO:    "",
	WARN:    ansi.ColorYellow,
	ERROR:   ansi.ColorRed,
	SUCCESS: ansi.ColorGreen,
	FATAL:   ansi.ColorMagenta,
}

func levelColor(level LevelT, fallback string) string {
	c := levelColors[level]
	if c != "" {
		return c
	} else {
		return fallback
	}
}

// SetLevelColor overrides the ANSI color for a given log level.
func SetLevelColor(level LevelT, color string) {
	levelColors[level] = color
}


// SimpleLogger is a logger with support for topics, and filenames
// Note that most calls will call directly to the sharedInstance, but a caller
// might build a customized logger via logger.WithPrefix("prefix"). ... and
// that returns a temporary instance

type logDestination struct {
	mu     sync.Mutex
	logger *log.Logger
}

type SimpleLogger struct {
	dest      *logDestination
	topic     string
	file      string // file set explicitely via WithCaller
	line      int    // line set explicitely via WithCaller
	level     LevelT
	sessionId string // session ID for demon mode session-scoped logging
	maxLength int    // max length for string args; 0 = use default, -1 = unlimited
}

var sharedDest = &logDestination{}

var sharedInstance = SimpleLogger{
	dest:  sharedDest,
	level: INFO,
}

func Level() LevelT { return sharedInstance.level }

// WithCaller returns a logger that uses the provided caller info
func (sl *SimpleLogger) deepCopy(fun func(copy *SimpleLogger)) *SimpleLogger {
	deepCopy := &SimpleLogger{
		dest:      sl.dest,
		topic:     sl.topic,
		file:      sl.file,
		line:      sl.line,
		level:     sl.level,
		sessionId: sl.sessionId,
		maxLength: sl.maxLength,
	}

	fun(deepCopy)
	return deepCopy
}

func init() {
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		logFile = defaultLogFile()
	}
	setLogFile(logFile)
}

// logMessage holds the data for a log entry
type logMessage struct {
	level     LevelT
	file      string
	line      int
	topic     string
	sessionId string
	maxLength int // max length for string args; 0 = use default, -1 = unlimited
	format    string
	args      []any
}

// logChannel is a buffered channel for log messages
var logChannel = make(chan logMessage, 1000)

// done signals the background goroutine to exit
var done = make(chan struct{})

// transformArgs converts any Inspect-implementing values to their string representation.
// maxLen controls truncation: 0 = use default, -1 = unlimited, >0 = custom limit.
func transformArgs(args []any, maxLen int) []any {
	result := make([]any, len(args))
	for i, arg := range args {
		if protoMsg, ok := arg.(proto.Message); ok {
			// Use canonical protojson for protobuf messages
			arg = protojson.Format(protoMsg)
		}
		if argStr, ok := arg.(string); ok && maxLen != -1 {
			// Truncate string arguments (skip when unlimited)
			arg = truncate(argStr, maxLen)
		}

		result[i] = arg
	}
	return result
}

const defaultMaxLogLength = 200

// truncate cuts the string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	// if maxLen is less than 0 (typically -1) don't shorten the string
	if maxLen < 0 {
		return s
	}

	// if maxLen is not set use default
	if maxLen == 0 {
		maxLen = defaultMaxLogLength
	}

	// truncate string if necessary
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ndjsonEntry is the JSON structure written to the log file.
type ndjsonEntry struct {
	TimestampMs    int64  `json:"timestamp_ms"`
	Timestamp      string `json:"timestamp"`
	Level          string `json:"level"`
	SessionID      string `json:"sid,omitempty"`
	File           string `json:"file,omitempty"`
	Line           int    `json:"line,omitempty"`
	Topic          string `json:"topic,omitempty"`
	Message        string `json:"msg"`
	RecommendedLen *int   `json:"recommendedMsgLen,omitempty"`
}

// processLogEntry formats and writes a log entry to the shared destination
// (as ndjson) and any extra targets (as formatted text).
func processLogEntry(entry logMessage) {
	// Build the full message without truncation for ndjson
	fullArgs := transformArgs(entry.args, -1)
	plainMsg := fmt.Sprintf(entry.format, fullArgs...)

	// Write ndjson to log file.
	// maxLength: 0 = not set, >0 = custom limit, -1 = unlimited.
	// Only include in payload when explicitly set (non-zero).
	var recommendedLen *int
	if entry.maxLength != 0 {
		recommendedLen = &entry.maxLength
	}

	now := time.Now()
	ndj := ndjsonEntry{
		TimestampMs:    now.UnixMilli(),
		Timestamp:      now.Format("2006/01/02 15:04:05.000"),
		Level:          entry.level.String(),
		SessionID:      entry.sessionId,
		File:           entry.file,
		Line:           entry.line,
		Topic:          entry.topic,
		Message:        plainMsg,
		RecommendedLen: recommendedLen,
	}
	jsonBytes, _ := json.Marshal(ndj)
	sharedDest.logger.Println(string(jsonBytes))
}

func init() {
	// Start background writer goroutine
	go func() {
		for {
			select {
			case entry, ok := <-logChannel:
				if !ok {
					return
				}
				processLogEntry(entry)
			case <-done:
				// Drain remaining messages before exiting
				for {
					select {
					case entry := <-logChannel:
						processLogEntry(entry)
					default:
						return
					}
				}
			}
		}
	}()
}

// Close shuts down the logger goroutine. Should be called before program exit
// if you want to ensure all log messages are flushed.
func Close() {
	close(done)
}

func Runtime[T any](msg string, fun func() T) T {
	start := time.Now()
	r := fun()

	durationMs := float64(time.Since(start).Microseconds()) / 1000.0
	Info("%s: %.1f msecs", msg, durationMs)
	return r
}

// logFilePath stores the current log file path.
var logFilePath string

// GetLogFile returns the current log file path.
func GetLogFile() string {
	sharedDest.mu.Lock()
	defer sharedDest.mu.Unlock()
	return logFilePath
}

// setLogFile opens the given path and configures the package-level fileLogger.
// It performs log rotation if the file exceeds the size limit.
func setLogFile(path string) {
	// Rotate before opening
	RotateLogFile(path)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("OpenFile(%s) failed: %s", path, err))
	}

	sharedDest.mu.Lock()
	defer sharedDest.mu.Unlock()
	logFilePath = path
	sharedDest.logger = log.New(f, "", 0)
}

// SetLogFlags sets the flags on the underlying log.Logger (e.g. 0 to disable timestamps).
func SetLogFlags(flags int) {
	sharedDest.mu.Lock()
	defer sharedDest.mu.Unlock()
	if sharedDest.logger != nil {
		sharedDest.logger.SetFlags(flags)
	}
}

// SetNullLog sets the logger to discard all output
func SetNullLog() {
	setLogFile("/dev/null")
}

// SetLevel sets the minimum log level
func SetLevel(l LevelT) {
	sharedInstance.level = l
}

// WithLevel returns a SimpleLogger that uses the provided log level
func WithLevel(level LevelT) *SimpleLogger {
	return sharedInstance.WithLevel(level)
}

// WithTopic returns a SimpleLogger that prepends [topic] to all log messages
func WithTopic(topic string) *SimpleLogger {
	return sharedInstance.WithTopic(topic)
}

// WithCaller returns a logger that uses the provided caller info
func WithCaller(file string, line int) *SimpleLogger {
	return sharedInstance.WithCaller(file, line)
}

// WithSessionId returns a logger that includes the session ID in log messages
func WithSessionId(sessionId string) *SimpleLogger {
	return sharedInstance.WithSessionId(sessionId)
}

// WithMaxLength returns a logger with a custom max length for string args.
// Use -1 for unlimited (no truncation).
func WithMaxLength(maxLength int) *SimpleLogger {
	return sharedInstance.WithMaxLength(maxLength)
}

// WithLevel returns a logger that uses the provided log level. This method allows chaining of WithXXX() calls.
func (sl *SimpleLogger) WithLevel(level LevelT) *SimpleLogger {
	return sl.deepCopy(func(copy *SimpleLogger) {
		copy.level = level
	})
}

// WithCaller returns a logger that uses the provided caller info. This method allows chaining of WithXXX() calls.
func (sl *SimpleLogger) WithCaller(file string, line int) *SimpleLogger {
	return sl.deepCopy(func(copy *SimpleLogger) {
		copy.file = file
		copy.line = line
	})
}

// WithTopic returns a SimpleLogger that prepends [topic] to all log messages. This method allows chaining of WithXXX() calls.
func (sl *SimpleLogger) WithTopic(topic string) *SimpleLogger {
	return sl.deepCopy(func(copy *SimpleLogger) {
		copy.topic = topic
	})
}

// WithSessionId returns a SimpleLogger that includes the session ID in all log messages.
func (sl *SimpleLogger) WithSessionId(sessionId string) *SimpleLogger {
	return sl.deepCopy(func(copy *SimpleLogger) {
		copy.sessionId = sessionId
	})
}

// WithMaxLength returns a logger that uses the given max length for string args.
// Use -1 for unlimited (no truncation), 0 for default.
func (sl *SimpleLogger) WithMaxLength(maxLength int) *SimpleLogger {
	return sl.deepCopy(func(copy *SimpleLogger) {
		copy.maxLength = maxLength
	})
}

// printf writes a log message with optional topic prefix
func (t *SimpleLogger) printf(level LevelT, format string, v ...any) {
	// determine caller unless set explicitly
	file, line := t.file, t.line
	if file == "" {
		_, file, line, _ = runtime.Caller(3)
	}
	logChannel <- logMessage{
		level:     level,
		file:      file,
		line:      line,
		topic:     t.topic,
		sessionId: t.sessionId,
		maxLength: t.maxLength,
		format:    format,
		args:      v,
	}
}

// Fatal writes a fatal error log message with topic prefix and exits
func (t *SimpleLogger) Fatal(format string, v ...any) {
	t.printf(FATAL, format, v...)
	os.Exit(1)
}

// FatalNoExit writes a FATAL-level log message without calling os.Exit.
// Use this when you need Follow(UntilProcessTerminates) to detect shutdown
// but still need to run cleanup code after logging.
func (t *SimpleLogger) FatalNoExit(format string, v ...any) {
	t.printf(FATAL, format, v...)
}

// Success writes a success log message at the same filtering level as Warn
func (t *SimpleLogger) Success(format string, v ...any) bool {
	if t.level > WARN {
		return false
	}
	t.printf(SUCCESS, format, v...)
	return true
}

// Error writes an error log message with topic prefix
func (t *SimpleLogger) Error(format string, v ...any) bool {
	if t.level > ERROR {
		return false
	}
	t.printf(ERROR, format, v...)
	return true
}

// Warn writes a warning log message with topic prefix
func (t *SimpleLogger) Warn(format string, v ...any) bool {
	if t.level > WARN {
		return false
	}
	t.printf(WARN, format, v...)
	return true
}

// Info writes an info log message with topic prefix
func (t *SimpleLogger) Info(format string, v ...any) bool {
	if t.level > INFO {
		return false
	}
	t.printf(INFO, format, v...)
	return true
}

// Debug writes a debug log message with topic prefix
func (t *SimpleLogger) Debug(format string, v ...any) bool {
	if t.level > DEBUG {
		return false
	}
	t.printf(DEBUG, format, v...)
	return true
}

func Fatal(format string, v ...any) {
	sharedInstance.Fatal(format, v...)
}

func FatalNoExit(format string, v ...any) {
	sharedInstance.FatalNoExit(format, v...)
}

func Success(format string, v ...any) bool {
	return sharedInstance.Success(format, v...)
}

func Error(format string, v ...any) bool {
	return sharedInstance.Error(format, v...)
}

func Warn(format string, v ...any) bool {
	return sharedInstance.Warn(format, v...)
}

func Info(format string, v ...any) bool {
	return sharedInstance.Info(format, v...)
}

func Debug(format string, v ...any) bool {
	return sharedInstance.Debug(format, v...)
}
