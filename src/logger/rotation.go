package logger

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxLogFileSize is the maximum log file size before rotation (10 MB).
const maxLogFileSize = 10 * 1024 * 1024

// maxLogFiles is the maximum number of rotated log files to keep.
const maxLogFiles = 3

// RotateLogFile rotates the log file if it exceeds maxLogFileSize.
// Keeps up to maxLogFiles old files (e.g. critic.ndjson.log.1, critic.ndjson.log.2).
func RotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogFileSize {
		return
	}

	// Rotate files: .3 -> delete, .2 -> .3, .1 -> .2, current -> .1
	for i := maxLogFiles; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", path, i)
		if i == maxLogFiles {
			os.Remove(old)
		} else {
			newer := fmt.Sprintf("%s.%d", path, i+1)
			os.Rename(old, newer)
		}
	}

	// Move current to .1
	os.Rename(path, path+".1")
}

// defaultLogFile returns the default log file path in the system temp dir.
func defaultLogFile() string {
	return filepath.Join(os.TempDir(), "critic.ndjson.log")
}
