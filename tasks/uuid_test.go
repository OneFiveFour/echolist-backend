package tasks

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
)

// Requirements: 9.1, 9.2

func TestValidateUuidV4_ValidUuid(t *testing.T) {
	valid := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-41d1-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"00000000-0000-4000-8000-000000000000",
		"ffffffff-ffff-4fff-bfff-ffffffffffff",
	}
	for _, id := range valid {
		if err := validateUuidV4(id); err != nil {
			t.Errorf("validateUuidV4(%q) = %v; want nil", id, err)
		}
	}
}

func TestValidateUuidV4_UppercaseRejected(t *testing.T) {
	id := "550E8400-E29B-41D4-A716-446655440000"
	err := validateUuidV4(id)
	if err == nil {
		t.Fatalf("validateUuidV4(%q): expected error for uppercase, got nil", id)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateUuidV4_WrongVersionRejected(t *testing.T) {
	// Version digit is '5' instead of '4'
	id := "550e8400-e29b-51d4-a716-446655440000"
	err := validateUuidV4(id)
	if err == nil {
		t.Fatalf("validateUuidV4(%q): expected error for wrong version, got nil", id)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateUuidV4_EmptyStringRejected(t *testing.T) {
	err := validateUuidV4("")
	if err == nil {
		t.Fatal("validateUuidV4(\"\"): expected error for empty string, got nil")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateUuidV4_WrongVariantRejected(t *testing.T) {
	// Variant nibble is 'c' instead of 8/9/a/b
	id := "550e8400-e29b-41d4-c716-446655440000"
	err := validateUuidV4(id)
	if err == nil {
		t.Fatalf("validateUuidV4(%q): expected error for wrong variant, got nil", id)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}
