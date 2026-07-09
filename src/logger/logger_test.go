package logger

import (
	"testing"
)

func TestSuccessFilteredBelowErrorLevel(t *testing.T) {
	origLevel := sharedInstance.level
	defer func() { sharedInstance.level = origLevel }()

	// Set level above ERROR — Success should be filtered out
	sharedInstance.level = FATAL

	logged := Success("should not appear")
	if logged {
		t.Error("expected Success to return false when level is FATAL")
	}
}
