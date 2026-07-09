package conduit

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
)

// FileWatcher returns a Conduit[notify.EventInfo] that watches each of the
// given absolute paths (non-recursive) and forwards create, write, remove, and
// rename events. Close() stops all watches.
func FileWatcher(paths ...string) *Conduit[notify.EventInfo] {
	return fileWatcher(paths)
}

// FileWatcherRecursive is like FileWatcher but watches each path recursively,
// including all subdirectories.
func FileWatcherRecursive(paths ...string) *Conduit[notify.EventInfo] {
	recursive := make([]string, len(paths))
	for i, p := range paths {
		recursive[i] = filepath.Join(p, "...")
	}
	return fileWatcher(recursive)
}

func fileWatcher(paths []string) *Conduit[notify.EventInfo] {
	ch := make(chan notify.EventInfo, 256)
	for _, p := range paths {
		_ = notify.Watch(p, ch, notify.Create, notify.Write, notify.Remove, notify.Rename)
	}
	return &Conduit[notify.EventInfo]{ch: ch, closeFn: func() { notify.Stop(ch) }}
}

// Ticker returns a Conduit[time.Time] that emits the current time at every
// interval d. Close() stops the underlying ticker and exits the goroutine.
func Ticker(d time.Duration) *Conduit[time.Time] {
	t := time.NewTicker(d)
	ch := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() {
		defer close(ch)
		for {
			select {
			case tick := <-t.C:
				select {
				case ch <- tick:
				default:
				}
			case <-done:
				return
			}
		}
	}()
	return &Conduit[time.Time]{ch: ch, closeFn: func() {
		t.Stop()
		close(done)
	}}
}

// Null returns a Conduit that never emits. Its channel is nil, so a select
// case on Events() blocks forever — equivalent to the conduit being absent.
// Close is a no-op.
func Null[T any]() *Conduit[T] {
	return &Conduit[T]{ch: nil, closeFn: func() {}}
}

// Conduit[T] pairs a channel of T with its cleanup so callers don't have to
// hold both separately. The close function is type-specific and supplied at
// construction time.
type Conduit[T any] struct {
	ch      chan T
	closeFn func()
}

// Events returns the receive-only channel of events.
func (w *Conduit[T]) Events() <-chan T { return w.ch }

// Close runs the cleanup function supplied at construction.
func (w *Conduit[T]) Close() { w.closeFn() }

// Filter returns a new Conduit that passes through only events for which keep
// returns true. The goroutine exits when the input channel closes.
// Close() propagates to the upstream Conduit.
func (w *Conduit[T]) Filter(keep func(T) bool) *Conduit[T] {
	out := make(chan T, cap(w.ch))
	go func() {
		defer close(out)
		for ev := range w.ch {
			if !keep(ev) {
				continue
			}
			out <- ev
		}
	}()
	return &Conduit[T]{ch: out, closeFn: w.closeFn}
}

// Map returns a new Conduit that transforms each event with fn.
// The goroutine exits when the input channel closes.
// Close() propagates to the upstream Conduit.
func Map[T, U any](w *Conduit[T], fn func(T) U) *Conduit[U] {
	out := make(chan U, cap(w.ch))
	go func() {
		defer close(out)
		for ev := range w.ch {
			out <- fn(ev)
		}
	}()
	return &Conduit[U]{ch: out, closeFn: w.closeFn}
}

// MapWithState returns a Conduit[U] that transforms each event using fn, which
// carries a state value S across calls. fn receives the current state and the
// incoming event and returns the next state and the output event to forward.
func MapWithState[T, S, U any](w *Conduit[T], initial S, fn func(S, T) (S, U)) *Conduit[U] {
	out := make(chan U, cap(w.ch))
	go func() {
		defer close(out)
		state := initial
		for ev := range w.ch {
			var next U
			state, next = fn(state, ev)
			out <- next
		}
	}()
	return &Conduit[U]{ch: out, closeFn: w.closeFn}
}

// OneOf returns a Conduit[T] that forwards events from any of the given
// conduits. The output channel receives whichever event arrives first. All
// upstream conduits are closed when the returned Conduit is closed. The
// goroutine exits when all input channels close.
func OneOf[T any](conduits ...*Conduit[T]) *Conduit[T] {
	// Collect all close functions.
	closeFns := make([]func(), len(conduits))
	for i, c := range conduits {
		closeFns[i] = c.closeFn
	}
	closeAll := func() {
		for _, fn := range closeFns {
			fn()
		}
	}

	out := make(chan T, 1)
	var wg sync.WaitGroup
	wg.Add(len(conduits))

	for _, c := range conduits {
		go func(ch <-chan T) {
			defer wg.Done()
			for ev := range ch {
				out <- ev
			}
		}(c.ch)
	}

	// Close output when all inputs close.
	go func() {
		wg.Wait()
		close(out)
	}()

	return &Conduit[T]{ch: out, closeFn: closeAll}
}

// Throttle returns a Conduit[struct{}] that drains events for d after the
// first event arrives, then emits one signal (trailing-edge, fixed window).
// This absorbs the burst of filesystem events that typically follows a single
// user action before notifying the caller. The goroutine exits when the input
// channel closes. Close() propagates to the upstream Conduit.
func (w *Conduit[T]) Throttle(d time.Duration) *Conduit[struct{}] {
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		for {
			// Wait for the first event.
			_, ok := <-w.ch
			if !ok {
				return
			}
			// Drain further events for d.
			deadline := time.After(d)
		drain:
			for {
				select {
				case _, ok := <-w.ch:
					if !ok {
						break drain
					}
				case <-deadline:
					break drain
				}
			}
			// Emit one signal after the drain window.
			out <- struct{}{}
		}
	}()
	return &Conduit[struct{}]{ch: out, closeFn: w.closeFn}
}
