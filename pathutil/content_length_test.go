package pathutil

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestValidateContentLength_WithinLimit(t *testing.T) {
	if err := ValidateContentLength("hello", 10, "field"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateContentLength_ExactLimit(t *testing.T) {
	if err := ValidateContentLength("12345", 5, "field"); err != nil {
		t.Fatalf("unexpected error at exact limit: %v", err)
	}
}

func TestValidateContentLength_ExceedsLimit(t *testing.T) {
	err := ValidateContentLength("too long", 3, "body")
	if err == nil {
		t.Fatal("expected error for oversized content, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
	if !strings.Contains(ce.Message(), "body") {
		t.Fatalf("error should mention field name, got %q", ce.Message())
	}
}

func TestValidateContentLength_Empty(t *testing.T) {
	if err := ValidateContentLength("", 0, "field"); err != nil {
		t.Fatalf("empty string with zero limit should pass: %v", err)
	}
}

func TestValidateName_TooLong(t *testing.T) {
	long := strings.Repeat("a", MaxNameLen+1)
	err := ValidateName(long)
	if err == nil {
		t.Fatal("expected error for name exceeding MaxNameLen")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateName_AtLimit(t *testing.T) {
	name := strings.Repeat("a", MaxNameLen)
	if err := ValidateName(name); err != nil {
		t.Fatalf("name at exact limit should pass: %v", err)
	}
}
