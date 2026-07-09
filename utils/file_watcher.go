package utils

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/radiospiel/simple-go/logger"
)

// FileWatcher watches a single file for changes.
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	filePath    string
	debounceMs  int
	changesChan chan struct{}

	// Debouncing state
	debounceTimer *time.Timer
	debounceMu    sync.Mutex

	// Lifecycle
	stopChan chan struct{}
}

// NewFileWatcher creates a watcher that monitors a single file for changes.
func NewFileWatcher(filePath string, debounceMs int) (*FileWatcher, error) {
	logger.Debug("NewFileWatcher: Creating watcher for %s with debounce=%dms", filePath, debounceMs)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("NewFileWatcher: Failed to create fsnotify watcher: %v", err)
		return nil, err
	}

	fw := &FileWatcher{
		watcher:     w,
		filePath:    filePath,
		debounceMs:  debounceMs,
		changesChan: make(chan struct{}, 10),
		stopChan:    make(chan struct{}),
	}

	// Watch the directory containing the file (fsnotify watches directories)
	dir := filepath.Dir(filePath)
	if err := w.Add(dir); err != nil {
		w.Close()
		logger.Error("NewFileWatcher: Failed to watch directory %s: %v", dir, err)
		return nil, err
	}

	// Start the event loop
	go fw.eventLoop()

	logger.Debug("NewFileWatcher: Started watching %s", filePath)
	return fw, nil
}

// eventLoop handles fsnotify events and debounces them
func (fw *FileWatcher) eventLoop() {
	logger.Debug("FileWatcher: Event loop started for %s", fw.filePath)
	fileName := filepath.Base(fw.filePath)

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				logger.Info("FileWatcher: Events channel closed")
				return
			}

			// Only care about events for our specific file
			if filepath.Base(event.Name) != fileName {
				continue
			}

			// Only care about write, create, remove, rename events
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				logger.Debug("FileWatcher: Event: %s %s", event.Op, event.Name)
				fw.scheduleNotification()
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				logger.Info("FileWatcher: Errors channel closed")
				return
			}
			logger.Error("FileWatcher: Error: %v", err)

		case <-fw.stopChan:
			logger.Info("FileWatcher: Stop signal received")
			return
		}
	}
}

// scheduleNotification schedules a debounced change notification
func (fw *FileWatcher) scheduleNotification() {
	fw.debounceMu.Lock()
	defer fw.debounceMu.Unlock()

	// Cancel existing timer if any
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}

	// Schedule new notification
	fw.debounceTimer = time.AfterFunc(time.Duration(fw.debounceMs)*time.Millisecond, func() {
		select {
		case fw.changesChan <- struct{}{}:
			logger.Info("FileWatcher: Change notification sent for %s", fw.filePath)
		default:
			logger.Debug("FileWatcher: Notification channel full, dropping")
		}
	})
}

// Changes returns a channel that receives notifications when the file changes.
func (fw *FileWatcher) Changes() <-chan struct{} {
	return fw.changesChan
}

// Path returns the path of the file being watched.
func (fw *FileWatcher) Path() string {
	return fw.filePath
}

// Close stops the watcher and releases resources.
func (fw *FileWatcher) Close() error {
	logger.Info("FileWatcher: Closing watcher for %s", fw.filePath)

	// Stop the event loop
	close(fw.stopChan)

	// Stop any pending timer
	fw.debounceMu.Lock()
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}
	fw.debounceMu.Unlock()

	// Close the fsnotify watcher
	return fw.watcher.Close()
}
