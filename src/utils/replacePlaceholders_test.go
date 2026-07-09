package utils

import (
	"strings"
	"testing"
)

const placeholderRe = `\{\{([^{]+)\}\}`

func TestReplacePlaceholders(t *testing.T) {
	args := map[string]string{"a": "{{b}}", "b": "bar"}
	result, err := ReplacePlaceholders("{{a}} {{b}}", args, placeholderRe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "{{b}} bar" {
		t.Errorf("expected %q, got %q", "{{b}} bar", result)
	}
}

func TestReplacePlaceholders_MissingKey(t *testing.T) {
	args := map[string]string{"b": "bar"}
	_, err := ReplacePlaceholders("{{a}} {{b}}", args, placeholderRe)
	if err == nil {
		t.Fatal("expected error for missing placeholder")
	}
	if !strings.Contains(err.Error(), "required placeholder missing: a") {
		t.Errorf("unexpected error message: %v", err)
	}
}
