package tasks

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/radiospiel/simple-go/src/assert"
)

func TestRunExclusively_BasicExecution(t *testing.T) {
	// A simple task that returns a value
	task, err := RunExclusively("test-1", func() int {
		return 42
	})

	assert.NoError(t, err)
	assert.NotNil(t, task)

	// Wait for result
	result := <-task.Done()
	assert.Equals(t, result, 42)
}

func TestRunExclusively_RejectsSecondTaskWithSameID(t *testing.T) {
	started := make(chan struct{})
	blocker := make(chan struct{})

	// Start a long-running task
	task1, err := RunExclusively("exclusive-task", func() int {
		close(started)
		<-blocker // Block until we signal
		return 1
	})
	assert.NoError(t, err)
	assert.NotNil(t, task1)

	// Wait for task1 to start
	<-started

	// Try to start another task with the same ID
	task2, err := RunExclusively("exclusive-task", func() int {
		return 2
	})

	assert.NotNil(t, err)
	assert.Nil(t, task2)
	assert.Contains(t, err.Error(), "exclusive-task")
	assert.Contains(t, err.Error(), "already running")

	// Clean up
	close(blocker)
	<-task1.Done()
}

func TestRunExclusively_AllowsNewTaskAfterPreviousCompletes(t *testing.T) {
	// First task
	task1, err := RunExclusively("reusable-id", func() int {
		return 1
	})
	assert.NoError(t, err)
	result1 := <-task1.Done()
	assert.Equals(t, result1, 1)

	// Second task with same ID should work now
	task2, err := RunExclusively("reusable-id", func() int {
		return 2
	})
	assert.NoError(t, err)
	result2 := <-task2.Done()
	assert.Equals(t, result2, 2)
}

func TestRunExclusively_DifferentIDsCanRunConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	started1 := make(chan struct{})
	started2 := make(chan struct{})
	done := make(chan struct{})

	task1, err := RunExclusively("id-a", func() int {
		close(started1)
		wg.Done()
		<-done
		return 1
	})
	assert.NoError(t, err)

	task2, err := RunExclusively("id-b", func() int {
		close(started2)
		wg.Done()
		<-done
		return 2
	})
	assert.NoError(t, err)

	// Both should have started
	<-started1
	<-started2

	// Let them finish
	close(done)

	result1 := <-task1.Done()
	result2 := <-task2.Done()

	assert.Equals(t, result1, 1)
	assert.Equals(t, result2, 2)
}

func TestTask_Abort(t *testing.T) {
	blocker := make(chan struct{})
	taskStarted := make(chan struct{})

	task, err := RunExclusively("abortable", func() int {
		close(taskStarted)
		<-blocker // This will block forever unless aborted
		return 42
	})
	assert.NoError(t, err)

	// Wait for task to start
	<-taskStarted

	// Abort the task
	task.Abort()

	// Task should complete (with zero value since aborted)
	select {
	case <-task.Done():
		// Good - task was aborted
	case <-time.After(1 * time.Second):
		t.Fatal("task did not abort within timeout")
	}

	// Should be able to start a new task with the same ID
	task2, err := RunExclusively("abortable", func() int {
		return 99
	})
	assert.NoError(t, err)
	result := <-task2.Done()
	assert.Equals(t, result, 99)
}

func TestTask_AbortContext(t *testing.T) {
	// Test that we can check for abort inside the task using Context
	taskCompleted := make(chan bool)

	task, err := RunExclusivelyWithContext("context-abort", func(ctx *Context) int {
		for i := 0; i < 100; i++ {
			if ctx.IsAborted() {
				taskCompleted <- false
				return 0
			}
			time.Sleep(10 * time.Millisecond)
		}
		taskCompleted <- true
		return 42
	})
	assert.NoError(t, err)

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Abort
	task.Abort()

	// Task should have detected abort
	completed := <-taskCompleted
	assert.Equals(t, completed, false)
}

func TestRunExclusively_StringType(t *testing.T) {
	task, err := RunExclusively("string-task", func() string {
		return "hello world"
	})
	assert.NoError(t, err)
	result := <-task.Done()
	assert.Equals(t, result, "hello world")
}

func TestRunExclusively_StructType(t *testing.T) {
	type Result struct {
		Value int
		Err   error
	}

	task, err := RunExclusively("struct-task", func() Result {
		return Result{Value: 42, Err: errors.New("test error")}
	})
	assert.NoError(t, err)
	result := <-task.Done()
	assert.Equals(t, result.Value, 42)
	assert.NotNil(t, result.Err)
}
