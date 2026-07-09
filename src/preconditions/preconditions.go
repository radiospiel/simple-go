package preconditions

import (
	"fmt"
	"runtime"

	"github.com/radiospiel/simple-go/src/logger"
)

// Check ensures that a condition is true.
// If the condition is false, it panics with the formatted error message.
//
// Use this to validate arguments, state, or any other precondition.
//
// Example:
//
//	Check(count >= 0, "count must be non-negative: %d", count)
//	Check(config != nil, "config must not be nil")
//	Check(isInitialized, "must call Init() before use")
func Check(condition bool, format string, args ...interface{}) {
	if !condition {
		msg := fmt.Sprintf(format, args...)
		_, file, line, _ := runtime.Caller(1)
		logger.WithCaller(file, line).Error("Precondition failed: %s", msg)
		panic(msg)
	}
}

func Fail(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, file, line, _ := runtime.Caller(1)
	logger.WithCaller(file, line).Error("Precondition failed: %s", msg)
	panic(msg)
}
