package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create a file that exceeds the rotation threshold
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write enough data to trigger rotation
	data := make([]byte, maxLogFileSize+1)
	f.Write(data)
	f.Close()

	RotateLogFile(logPath)

	// Original file should have been moved to .1
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Error("expected rotated file .1 to exist")
	}

	// Original file should not exist
	if _, err := os.Stat(logPath); err == nil {
		t.Error("expected original file to be removed after rotation")
	}
}

func TestRotateLogFileNoRotationNeeded(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create a small file
	os.WriteFile(logPath, []byte("small"), 0644)

	RotateLogFile(logPath)

	// File should still exist (no rotation)
	if _, err := os.Stat(logPath); err != nil {
		t.Error("file should still exist")
	}

	// No rotated file should exist
	if _, err := os.Stat(logPath + ".1"); err == nil {
		t.Error("no rotation should have happened")
	}
}

func TestDefaultLogFile(t *testing.T) {
	path := defaultLogFile()
	if path == "" {
		t.Error("default log file path should not be empty")
	}
	if filepath.Base(path) != "critic.ndjson.log" {
		t.Errorf("expected critic.ndjson.log, got %s", filepath.Base(path))
	}
}
