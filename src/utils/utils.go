package utils

import (
	"cmp"
	"slices"

	"errors"
	"github.com/radiospiel/simple-go/src/preconditions"
	"sync"
)

// Reverse reverses a slice in place
func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// SortBy returns a sorted copy of the slice, sorted by the key returned by the iteratee.
func SortBy[T any, K cmp.Ordered](collection []T, iteratee func(T) K) []T {
	result := make([]T, len(collection))
	copy(result, collection)
	slices.SortFunc(result, func(a, b T) int {
		return cmp.Compare(iteratee(a), iteratee(b))
	})
	return result
}

// Partition splits a slice into two slices based on a predicate.
// The first slice contains elements for which the predicate returns true,
// the second contains elements for which it returns false.
func Partition[T any](collection []T, predicate func(T) bool) ([]T, []T) {
	var matched, unmatched []T
	for _, item := range collection {
		if predicate(item) {
			matched = append(matched, item)
		} else {
			unmatched = append(unmatched, item)
		}
	}
	return matched, unmatched
}

func Null[T any]() T {
	var null T
	return null
}

// Iff returns ifTrue if condition is true, or T's null value
func Iff[T any](condition bool, ifTrue T) T {
	if condition {
		return ifTrue
	}
	return Null[T]()
}

// IfElse returns ifTrue if condition is true, otherwise returns ifFalse.
func IfElse[T any](condition bool, ifTrue T, ifFalse T) T {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// Clamp constrains a value to be within the range [minVal, maxVal].
func Clamp[T cmp.Ordered](value, minVal, maxVal T) T {
	preconditions.Check(minVal <= maxVal, "Clamp: minVal (%v) must be <= maxVal (%v)", minVal, maxVal)
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

// LRUCache is a simple LRU cache using a map and slice.
// It includes a creator function that is called when a key is not found.
type LRUCache[K comparable, V any] struct {
	mu         sync.RWMutex
	data       map[K]V
	usageOrder []K
	limit      int
	creator    func(K) (V, error)
}

func withMutex2[V any](mu *sync.RWMutex, fun func() (V, error)) (V, error) {
	mu.Lock()
	defer mu.Unlock()

	return fun()
}

// NewLRUCache creates a new LRU cache with the specified limit and creator function.
// The creator function is called when Get is called with a key that doesn't exist.
// Panics if limit < 1.
func NewLRUCache[K comparable, V any](limit int, creator func(K) (V, error)) *LRUCache[K, V] {
	preconditions.Check(limit >= 1, "LRUCache limit must be >= 1, got %d", limit)
	return &LRUCache[K, V]{
		data:       make(map[K]V),
		usageOrder: make([]K, 0, limit),
		limit:      limit,
		creator:    creator,
	}
}

var _keyDisappeared = errors.New("key disappeared")

// Get retrieves a value from the cache, creating it if it doesn't exist.
// If the key exists, it is moved to most recently used position.
// If the key doesn't exist, the creator function is called and the result is cached.
// If the creator returns an error, the result is not cached and the error is returned.
func (c *LRUCache[K, V]) Get(key K) (V, error) {
	// Use RLock for the initial existence check to avoid race conditions
	c.mu.RLock()
	_, exists := c.data[key]
	c.mu.RUnlock()

	if exists {
		value, err := withMutex2(&c.mu, func() (V, error) {
			if value, ok := c.data[key]; ok {
				// Move to end (most recently used) only if not already there
				// Search from end since recently used entries are more likely to be accessed
				c.moveUsedKeyToEnd(key, value)
				return value, nil
			} else {
				return value, _keyDisappeared
			}
		})
		if err == nil {
			return value, nil
		}
	}

	// Create new value
	value, err := c.creator(key)
	if err != nil {
		var zero V
		return zero, err
	}

	return withMutex2(&c.mu, func() (V, error) {
		// Add to data, evict oldest entry if at capacity
		if len(c.usageOrder) >= c.limit {
			oldest := c.usageOrder[0]
			c.usageOrder = c.usageOrder[1:]
			delete(c.data, oldest)
		}

		c.data[key] = value
		c.usageOrder = append(c.usageOrder, key)
		return value, nil
	})
}

func (c *LRUCache[K, V]) moveUsedKeyToEnd(key K, value V) {
	preconditions.Check(len(c.usageOrder) > 0, "usageOrder cannot be empty here")

	i := len(c.usageOrder) - 1
	if c.usageOrder[i] == key {
		return
	}

	for i > 0 {
		i--
		if c.usageOrder[i] == key {
			c.usageOrder = append(c.usageOrder[:i], c.usageOrder[i+1:]...)
			c.usageOrder = append(c.usageOrder, key)
			return
		}
	}

	preconditions.Fail("This should never happen")
}
