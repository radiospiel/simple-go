package tasks

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Context provides a way for tasks to check if they have been aborted.
type Context struct {
	aborted atomic.Bool
}

// IsAborted returns true if the task has been aborted.
func (c *Context) IsAborted() bool {
	return c.aborted.Load()
}

// Task represents a running exclusive task.
type Task[T any] struct {
	id      string
	done    chan T
	ctx     *Context
	cleanup func()
}

// Done returns a channel that will receive the result when the task completes.
func (t *Task[T]) Done() <-chan T {
	return t.done
}

// Abort signals the task to stop and removes it from the running tasks.
// The task function should check Context.IsAborted() to respond to abort requests.
// After Abort is called, the Done channel will receive the zero value of T.
func (t *Task[T]) Abort() {
	t.ctx.aborted.Store(true)
	t.cleanup()
	// Send zero value to unblock any waiters
	var zero T
	select {
	case t.done <- zero:
	default:
		// Channel already has a value or is full
	}
}

// registry holds all currently running tasks
var registry = &taskRegistry{
	tasks: make(map[string]any),
}

type taskRegistry struct {
	mu    sync.Mutex
	tasks map[string]any
}

func (r *taskRegistry) tryRegister(id string, task any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[id]; exists {
		return fmt.Errorf("task %q is already running", id)
	}
	r.tasks[id] = task
	return nil
}

func (r *taskRegistry) unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tasks, id)
}

// RunExclusively runs a task function exclusively by ID.
// If a task with the same ID is already running, it returns an error.
// The returned Task provides a Done() channel to receive the result
// and an Abort() method to cancel the task.
func RunExclusively[T any](id string, taskFunc func() T) (*Task[T], error) {
	ctx := &Context{}

	task := &Task[T]{
		id:   id,
		done: make(chan T, 1),
		ctx:  ctx,
		cleanup: func() {
			registry.unregister(id)
		},
	}

	if err := registry.tryRegister(id, task); err != nil {
		return nil, err
	}

	go func() {
		defer task.cleanup()

		// Run the task
		result := taskFunc()

		// Only send result if not aborted
		if !ctx.IsAborted() {
			select {
			case task.done <- result:
			default:
				// Channel already has a value (from abort)
			}
		}
	}()

	return task, nil
}

// RunExclusivelyWithContext runs a task function exclusively by ID, providing
// a Context that the task can use to check for abort signals.
func RunExclusivelyWithContext[T any](id string, taskFunc func(*Context) T) (*Task[T], error) {
	ctx := &Context{}

	task := &Task[T]{
		id:   id,
		done: make(chan T, 1),
		ctx:  ctx,
		cleanup: func() {
			registry.unregister(id)
		},
	}

	if err := registry.tryRegister(id, task); err != nil {
		return nil, err
	}

	go func() {
		defer task.cleanup()

		// Run the task with context
		result := taskFunc(ctx)

		// Only send result if not aborted
		if !ctx.IsAborted() {
			select {
			case task.done <- result:
			default:
				// Channel already has a value (from abort)
			}
		}
	}()

	return task, nil
}
