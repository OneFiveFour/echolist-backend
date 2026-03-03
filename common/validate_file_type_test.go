package common

import (
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
)

var noteType = FileType{Prefix: "note_", Suffix: ".md", Label: "note"}
var taskType = FileType{Prefix: "tasks_", Suffix: ".md", Label: "task list"}

func TestValidateFileType_ValidFiles(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		ft       FileType
	}{
		{"valid note", "note_Hello.md", noteType},
		{"valid task list", "tasks_Shopping.md", taskType},
		{"note with spaces in title", "note_My Notes.md", noteType},
		{"task with underscores", "tasks_work_items.md", taskType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := ValidateFileType(p, tt.ft); err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateFileType_NotFound(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note_Gone.md")

	err := ValidateFileType(p, noteType)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestValidateFileType_Directory(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note_Fake.md")
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}

	err := ValidateFileType(p, noteType)
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestValidateFileType_WrongPattern(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		ft       FileType
	}{
		{"random file as note", "readme.txt", noteType},
		{"task file checked as note", "tasks_Shopping.md", noteType},
		{"note file checked as task", "note_Hello.md", taskType},
		{"missing prefix", "Hello.md", noteType},
		{"missing suffix", "note_Hello.txt", noteType},
		{"prefix only no title", "note_.md", noteType},
		{"prefix only no title task", "tasks_.md", taskType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := ValidateFileType(p, tt.ft)
			if err == nil {
				t.Error("expected error for wrong pattern")
			}
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
			}
		})
	}
}
