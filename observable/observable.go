// Package observable provides a data wrapper for maps and lists with path-based
// access and change subscriptions.
package observable

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/radiospiel/simple-go/fnmatch"
	"github.com/radiospiel/simple-go/logger"
	"github.com/radiospiel/simple-go/preconditions"
	"github.com/radiospiel/simple-go/utils"
)

// maxArrayIndex is the maximum allowed array index to prevent accidental huge allocations.
const maxArrayIndex = 99999

// Subscription represents a registered observer subscription.
type Subscription int

// ChangeCallback is the function signature for change notifications.
// It receives the full key path that changed.
type ChangeCallback func(key string)

// SchemaValidationError represents a schema validation failure.
type SchemaValidationError struct {
	Key     string
	Message string
}

func (e *SchemaValidationError) Error() string {
	return e.Message
}

// ErrTransactionAborted is returned when operations are attempted on an aborted transaction.
var ErrTransactionAborted = &TransactionAbortedError{}

// TransactionAbortedError indicates that a transaction was aborted.
type TransactionAbortedError struct{}

func (e *TransactionAbortedError) Error() string {
	return "transaction is aborted"
}

type subscription struct {
	pattern   string
	callback  ChangeCallback
	fnmatcher fnmatch.Matcher
}

// Observable wraps maps and lists with path-based access and change subscriptions.
type Observable struct {
	data          any
	subscriptions map[Subscription]*subscription
	nextSubID     Subscription
	mu            sync.RWMutex
}

// New creates a new Observable with an empty map as the root.
func New() *Observable {
	return &Observable{
		data:          make(map[string]any),
		subscriptions: make(map[Subscription]*subscription),
	}
}

// NewWithData creates a new Observable with the provided data as the root.
// The data must be nil, a map[string]any, or []any.
func NewWithData(data any) *Observable {
	if data != nil {
		preconditions.Check(
			isMap(data) || isSlice(data),
			"root data must be nil, map[string]any, or []any, got %T", data,
		)
	}
	return &Observable{
		data:          data,
		subscriptions: make(map[Subscription]*subscription),
	}
}

// GetValue returns the value at the given key path, or nil if not found.
// Key is a dot-separated path like "x.1.a".
func (o *Observable) GetValue(key string) any {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.getValueInternal(key)
}

// GetValueAs returns the value at the given key path, converted to type T.
// Returns the zero value if the key does not exist.
// For primitive types, performs a direct type assertion.
// For structs, uses JSON marshaling to convert from map[string]any.
// The returned value is always a copy.
// Panics if the conversion fails.
// Usage: val := observable.GetValueAs[MyStruct](obs, "config")
func GetValueAs[T any](o *Observable, key string) T {
	val := o.GetValue(key)
	if val == nil {
		var zero T
		return zero
	}

	// Try direct type assertion first (works for primitives and exact type matches)
	if typed, ok := val.(T); ok {
		return typed
	}

	// Use JSON round-trip for struct conversion (provides deep copy)
	bytes, err := json.Marshal(val)
	preconditions.Check(err == nil, "failed to marshal value at key %q: %v", key, err)

	var result T
	err = json.Unmarshal(bytes, &result)
	preconditions.Check(err == nil, "failed to unmarshal value at key %q to %T: %v", key, result, err)

	return result
}

// getValueInternal returns the value without locking (caller must hold lock).
func (o *Observable) getValueInternal(key string) any {
	if key == "" {
		return o.data
	}

	parts := strings.Split(key, ".")
	current := o.data

	for _, part := range parts {
		if current == nil {
			return nil
		}

		if idx, isNum := parseIndex(part); isNum {
			// Numeric index - expect slice
			slice, ok := current.([]any)
			if !ok {
				return nil
			}
			if idx < 0 || idx >= len(slice) {
				return nil
			}
			current = slice[idx]
		} else {
			// String key - expect map
			m, ok := current.(map[string]any)
			if !ok {
				return nil
			}
			current = m[part]
		}
	}

	return current
}

// SetValueAtKey sets the value at the given key path, creating intermediate
// structures as needed. Returns an error if the value violates a schema constraint.
//
// Key is a dot-separated path like "x.1.a".
// Numeric path segments indicate array indices, string segments indicate map keys.
//
// Panics if:
//   - The existing value at a path segment has an incompatible type
//   - An array index is negative or >= 100000
func (o *Observable) SetValueAtKey(key string, value any) error {
	return o.Transaction(func(tx *Txn) {
		tx.SetValueAtKey(key, value)
	})
}

// setValue sets the value without locking (caller must hold lock).
func (o *Observable) setValue(key string, value any) {
	logger.Warn("observable.setValue path=%v value=%v", key, value)

	preconditions.Check(key != "", "Cannot set root value")

	parts := strings.Split(key, ".")

	// Use the recursive setter that returns the modified value
	o.data = o.setAtPath(o.data, parts, value)

	// For complex types, marshal to JSON
	// json, err := json.MarshalIndent(o.data, "", "  ")
	json, err := json.Marshal(o.data)
	if err != nil {
		panic("cannot json marshal data")
	}
	logger.Warn("observable.data %v", string(json))
}

// setAtPath recursively sets a value at the given path parts and returns the modified container.
func (o *Observable) setAtPath(current any, parts []string, value any) any {
	if len(parts) == 0 {
		return value
	}

	part := parts[0]
	isLastPart := len(parts) == 1

	if idx, isNum := parseIndex(part); isNum {
		// Current part is a numeric index - current must be a slice
		preconditions.Check(isSlice(current), "expected slice at index %d, got %T", idx, current)
		preconditions.Check(idx >= 0 && idx <= maxArrayIndex, "array index must be 0-%d, got %d", maxArrayIndex, idx)

		slice := current.([]any)

		// Extend slice if needed
		for len(slice) <= idx {
			slice = append(slice, nil)
		}

		if isLastPart {
			slice[idx] = value
		} else {
			// Determine what type the next level should be
			nextPart := parts[1]
			_, nextIsNum := parseIndex(nextPart)

			if slice[idx] == nil {
				if nextIsNum {
					slice[idx] = make([]any, 0)
				} else {
					slice[idx] = make(map[string]any)
				}
			} else {
				// Validate existing type
				if nextIsNum {
					preconditions.Check(isSlice(slice[idx]), "expected slice at %q, got %T", part, slice[idx])
				} else {
					preconditions.Check(isMap(slice[idx]), "expected map at %q, got %T", part, slice[idx])
				}
			}

			slice[idx] = o.setAtPath(slice[idx], parts[1:], value)
		}

		return slice
	}

	// Current part is a string key - current must be a map
	preconditions.Check(isMap(current), "expected map at key %q, got %T", part, current)

	m := current.(map[string]any)

	if isLastPart {
		if value == nil {
			delete(m, part)
		} else {
			m[part] = value
		}
	} else {
		// Determine what type the next level should be
		nextPart := parts[1]
		_, nextIsNum := parseIndex(nextPart)

		if m[part] == nil {
			if nextIsNum {
				m[part] = make([]any, 0)
			} else {
				m[part] = make(map[string]any)
			}
		} else {
			// Validate existing type
			if nextIsNum {
				preconditions.Check(isSlice(m[part]), "expected slice at %q, got %T", part, m[part])
			} else {
				preconditions.Check(isMap(m[part]), "expected map at %q, got %T", part, m[part])
			}
		}

		m[part] = o.setAtPath(m[part], parts[1:], value)
	}

	return m
}

// DeleteValueAtKey removes the value at the given key path.
// Returns an error if validation fails.
// This is equivalent to SetValueAtKey(key, nil).
func (o *Observable) DeleteValueAtKey(key string) error {
	return o.Transaction(func(tx *Txn) {
		tx.DeleteValueAtKey(key)
	})
}

// OnKeyChange registers a callback to be notified when values at the matching path change.
// Pattern uses fnmatch-style matching where * matches a single segment (not dots).
// Returns the subscription ID for later cleanup.
func (o *Observable) OnKeyChange(pattern string, callback ChangeCallback) Subscription {
	o.mu.Lock()
	defer o.mu.Unlock()

	preconditions.Check(callback != nil, "callback must not be nil")

	// Validate pattern (fnmatch.MustCompile panics on invalid pattern)
	fnmatcher := fnmatch.MustCompile(pattern, fnmatch.Options{Separators: "."})

	id := o.nextSubID
	o.nextSubID++

	o.subscriptions[id] = &subscription{
		pattern:   pattern,
		callback:  callback,
		fnmatcher: fnmatcher,
	}

	return id
}

// ClearSubscriptions removes the specified subscriptions.
func (o *Observable) ClearSubscriptions(subs ...Subscription) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, sub := range subs {
		delete(o.subscriptions, sub)
	}
}

// parseIndex tries to parse a string as a non-negative integer.
// Returns the index and true if successful, or 0 and false otherwise.
func parseIndex(s string) (int, bool) {
	idx, err := strconv.Atoi(s)
	if err != nil || idx < 0 {
		return 0, false
	}
	return idx, true
}

// isMap checks if a value is a map[string]any.
func isMap(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

// isSlice checks if a value is a []any.
func isSlice(v any) bool {
	_, ok := v.([]any)
	return ok
}

// change represents a single key-value change in a transaction.
type change struct {
	key   string
	value any
}

// Txn represents a transaction that batches changes before applying them.
// Transactions are created via Observable.Transaction() and are
// automatically committed when the callback returns, unless Abort() is called.
type Txn struct {
	obs     *Observable
	changes []change
	err     error
}

// SetValueAtKey records a change to be applied when the transaction commits.
// Returns an error if the value violates a schema constraint.
// If the transaction has an error, the call is ignored and returns ErrTransactionAborted.
func (tx *Txn) SetValueAtKey(key string, value any) error {
	if tx.err != nil {
		return ErrTransactionAborted
	}
	tx.changes = append(tx.changes, change{key: key, value: value})
	return nil
}

// DeleteValueAtKey records a deletion to be applied when the transaction commits.
// Returns an error if validation fails.
func (tx *Txn) DeleteValueAtKey(key string) error {
	return tx.SetValueAtKey(key, nil)
}

// Err returns any error that occurred during the transaction.
func (tx *Txn) Err() error {
	return tx.err
}

// Abort cancels the transaction. All recorded changes will be discarded
// and no notifications will be sent. Subsequent SetValueAtKey calls return ErrTransactionAborted.
func (tx *Txn) Abort() {
	if tx.err == nil {
		tx.err = ErrTransactionAborted
	}
	tx.changes = nil
}

// deduplicateChanges removes changes that are overridden by later changes.
// A change is overridden if a later change sets a parent key (or the same key).
func deduplicateChanges(changes []change) []change {
	// Work backwards: for each change, check if any later change overrides it
	result := make([]change, 0, len(changes))

	for i := len(changes) - 1; i >= 0; i-- {
		current := changes[i]
		overridden := false

		// Check if any change already in result (which are later changes) overrides this one
		for _, later := range result {
			if keyOverrides(later.key, current.key) {
				overridden = true
				break
			}
		}

		if !overridden {
			result = append(result, current)
		}
	}

	// Reverse to restore original order (we built it backwards)
	utils.Reverse(result)

	return result
}

// keyOverrides returns true if setting 'parent' would override a change to 'child'.
// This is true if parent is a prefix of child (or equal).
// Examples:
//   - keyOverrides("a", "a.1.b") = true  (setting "a" overwrites "a.1.b")
//   - keyOverrides("a", "a") = true      (same key)
//   - keyOverrides("a.1", "a") = false   (setting "a.1" doesn't overwrite "a")
//   - keyOverrides("", "a") = true       (setting root overwrites everything)
func keyOverrides(parent, child string) bool {
	if parent == child {
		return true
	}
	if parent == "" {
		return true // Root overrides everything
	}
	// parent must be a prefix followed by "."
	return strings.HasPrefix(child, parent+".")
}

// Transaction executes a transaction. The callback receives a Txn object to record changes.
// Changes are automatically committed when the callback returns, unless Abort() is called
// or an error occurred during the transaction.
// On commit, all schemas are validated against the final state to catch cross-field
// constraint violations that individual updates might not detect.
// Returns an error if any schema validation fails.
// Example:
//
//	err := obs.Transaction(func(tx *Txn) {
//	    tx.SetValueAtKey("foo", "bar")
//	    tx.SetValueAtKey("baz", 123)
//	})
func (o *Observable) Transaction(fn func(*Txn)) error {
	tx := &Txn{
		obs:     o,
		changes: make([]change, 0),
	}

	fn(tx)

	if tx.err != nil {
		return tx.err
	}

	return o.setValuesAtKeys(tx.changes)
}

// setValuesAtKeys applies multiple changes atomically and notifies subscribers.
// Changes are deduplicated: later changes to parent keys override earlier child changes.
// Returns an error if final state validation against any schema fails.
func (o *Observable) setValuesAtKeys(changes []change) error {
	if len(changes) == 0 {
		return nil
	}

	// Remove changes that are overridden by later change on the same or a parent key
	// For example: ["a.1.b", "a", "a.2", "c", "a.1", "a"] becomes ["c", "a"]
	changes = deduplicateChanges(changes)

	o.mu.Lock()
	for _, c := range changes {
		o.setValue(c.key, c.value)
	}

	subs := slices.Collect(maps.Values(o.subscriptions))
	o.mu.Unlock()

	// Notify subscriptions for each change
	for _, c := range changes {
		for _, sub := range subs {
			if keyAffectsPattern(c.key, sub) {
				sub.callback(c.key)
			}
		}
	}
	return nil
}

// keyAffectsPattern returns true if setting 'key' should trigger a subscription on 'pattern'.
// This is true if:
//   - key is empty (root change affects everything)
//   - pattern matches key directly (e.g., pattern="foo" matches key="foo", pattern="*" matches key="bar")
//   - key is a parent of pattern (e.g., key="foo" affects pattern="foo.bar" or "foo.*")
func keyAffectsPattern(key string, sub *subscription) bool {
	if key == "" {
		return true // root change affects all subscriptions
	}
	if sub.fnmatcher.MatchString(key) {
		return true // pattern directly matches the changed key
	}
	// Check if key is a parent of pattern (key="foo" affects pattern="foo.bar" or "foo.*.baz")
	return strings.HasPrefix(sub.pattern, key+".")
}
