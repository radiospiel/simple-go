package must

import (
	"errors"
	"testing"
)

func TestDepanic_NoPanic(t *testing.T) {
	val, err := Depanic(func() int { return 42 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestDepanic_PanicWithError(t *testing.T) {
	origErr := errors.New("boom")
	val, err := Depanic(func() int { panic(origErr) })
	if err != origErr {
		t.Fatalf("expected original error, got %v", err)
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %d", val)
	}
}

func TestDepanic_PanicWithString(t *testing.T) {
	val, err := Depanic(func() int { panic("oops") })
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "panic: oops" {
		t.Fatalf("expected 'panic: oops', got %q", err.Error())
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %d", val)
	}
}
