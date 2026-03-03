package common

import (
	"testing"

	"connectrpc.com/connect"
)

func TestValidateName_NullByte(t *testing.T) {
	err := ValidateName("hello\x00world")
	if err == nil {
		t.Fatal("expected error for name containing null byte")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestValidateName_Empty(t *testing.T) {
	err := ValidateName("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestValidateName_PathSeparators(t *testing.T) {
	for _, name := range []string{"a/b", "a\\b"} {
		err := ValidateName(name)
		if err == nil {
			t.Errorf("expected error for name %q with path separator", name)
		}
	}
}

func TestValidateName_DotEntries(t *testing.T) {
	for _, name := range []string{".", ".."} {
		err := ValidateName(name)
		if err == nil {
			t.Errorf("expected error for dot entry %q", name)
		}
	}
}

func TestValidateName_ValidNames(t *testing.T) {
	for _, name := range []string{"hello", "my file", "notes_2026", ".hidden"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) unexpected error: %v", name, err)
		}
	}
}
