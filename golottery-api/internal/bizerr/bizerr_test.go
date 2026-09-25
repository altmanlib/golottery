package bizerr

import (
	"strings"
	"testing"
)

func TestAllCodesHaveMessages(t *testing.T) {
	if len(All()) != 26 {
		t.Fatalf("codes = %d", len(All()))
	}
	for _, code := range All() {
		msg := MessageOf(code)
		if strings.TrimSpace(msg) == "" {
			t.Fatalf("code %s has empty message", code)
		}
		if strings.HasSuffix(msg, "。") {
			t.Fatalf("code %s message ends with Chinese period: %q", code, msg)
		}
		if StatusOf(code) == 0 {
			t.Fatalf("code %s has no status", code)
		}
	}
}

func TestNewFormatsArgs(t *testing.T) {
	err := New(CodeTooManyAttempts, 10)
	if !strings.Contains(err.Message, "10") {
		t.Fatalf("message = %q", err.Message)
	}
	if StatusOf(err.Code) != 429 {
		t.Fatalf("status = %d", StatusOf(err.Code))
	}
}
