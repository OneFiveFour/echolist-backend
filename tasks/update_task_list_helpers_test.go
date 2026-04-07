package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
)

// ---------------------------------------------------------------------------
// readAndParseTaskFile
// ---------------------------------------------------------------------------

func TestReadAndParseTaskFile_Success(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "tasks_Test.md")
	content := PrintTaskFile([]MainTask{
		{Description: "buy milk"},
		{Description: "walk dog", Done: true},
	})
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	tasks, err := readAndParseTaskFile(absPath, "tasks_Test.md", nopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Description != "buy milk" {
		t.Fatalf("expected 'buy milk', got %q", tasks[0].Description)
	}
	if !tasks[1].Done {
		t.Fatal("expected second task to be done")
	}
}

func TestReadAndParseTaskFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "nonexistent.md")

	_, err := readAndParseTaskFile(absPath, "nonexistent.md", nopLogger())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestReadAndParseTaskFile_MalformedContent(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "tasks_Bad.md")
	if err := os.WriteFile(absPath, []byte("not a valid task line"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := readAndParseTaskFile(absPath, "tasks_Bad.md", nopLogger())
	if err == nil {
		t.Fatal("expected error for malformed content, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("expected CodeInternal, got %v", connect.CodeOf(err))
	}
}

// ---------------------------------------------------------------------------
// advanceRecurringTasks
// ---------------------------------------------------------------------------

func TestAdvanceRecurringTasks_SkipsNonRecurring(t *testing.T) {
	tasks := []MainTask{
		{Description: "plain task", Done: true},
		{Description: "open recurring", Recurrence: "FREQ=DAILY", Done: false, DueDate: "2026-04-01"},
	}
	if err := advanceRecurringTasks(tasks, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Plain done task stays done
	if !tasks[0].Done {
		t.Fatal("non-recurring task should remain done")
	}
	// Open recurring task stays open with original due date
	if tasks[1].Done {
		t.Fatal("open recurring task should remain not-done")
	}
	if tasks[1].DueDate != "2026-04-01" {
		t.Fatalf("open recurring task due date should be unchanged, got %q", tasks[1].DueDate)
	}
}

func TestAdvanceRecurringTasks_AdvancesDoneRecurring(t *testing.T) {
	tasks := []MainTask{
		{Description: "weekly", Recurrence: "FREQ=WEEKLY", Done: true, DueDate: "2026-03-01"},
	}
	existing := []MainTask{
		{Description: "weekly", Recurrence: "FREQ=WEEKLY", DueDate: "2026-03-01"},
	}

	if err := advanceRecurringTasks(tasks, existing); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tasks[0].Done {
		t.Fatal("recurring task should be reset to done=false")
	}
	if tasks[0].DueDate <= "2026-03-01" {
		t.Fatalf("due date should advance past 2026-03-01, got %q", tasks[0].DueDate)
	}
}

func TestAdvanceRecurringTasks_UsesOwnDueDateWhenNoExisting(t *testing.T) {
	tasks := []MainTask{
		{Description: "daily", Recurrence: "FREQ=DAILY", Done: true, DueDate: "2026-06-15"},
	}

	if err := advanceRecurringTasks(tasks, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tasks[0].Done {
		t.Fatal("should be reset to done=false")
	}
	if tasks[0].DueDate != "2026-06-16" {
		t.Fatalf("expected 2026-06-16, got %q", tasks[0].DueDate)
	}
}

func TestAdvanceRecurringTasks_InvalidRRuleReturnsError(t *testing.T) {
	tasks := []MainTask{
		{Description: "bad", Recurrence: "NOT_VALID", Done: true},
	}
	err := advanceRecurringTasks(tasks, nil)
	if err == nil {
		t.Fatal("expected error for invalid RRULE, got nil")
	}
}

func TestAdvanceRecurringTasks_EmptySlice(t *testing.T) {
	if err := advanceRecurringTasks(nil, nil); err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
}

// ---------------------------------------------------------------------------
// renameTaskFile
// ---------------------------------------------------------------------------

func TestRenameTaskFile_SamePathNoOp(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "tasks_A.md")
	os.WriteFile(absPath, []byte("- [ ] task"), 0644)

	rr, err := renameTaskFile(nopLogger(), renameParams{
		oldAbsPath:  absPath,
		newAbsPath:  absPath,
		oldFilePath: "tasks_A.md",
		newFilePath: "tasks_A.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.renamed {
		t.Fatal("expected renamed=false for same path")
	}
	if rr.absPath != absPath {
		t.Fatalf("expected absPath=%q, got %q", absPath, rr.absPath)
	}
}

func TestRenameTaskFile_SuccessfulRename(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)
	oldAbs := filepath.Join(dir, "tasks_Old.md")
	newAbs := filepath.Join(dir, "tasks_New.md")
	os.WriteFile(oldAbs, []byte("- [ ] task"), 0644)

	rr, err := renameTaskFile(nopLogger(), renameParams{
		oldAbsPath:   oldAbs,
		newAbsPath:   newAbs,
		oldFilePath:  "tasks_Old.md",
		newFilePath:  "tasks_New.md",
		regPath:      regPath,
		id:           "00000000-0000-4000-8000-000000000001",
		isAutoDelete: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rr.renamed {
		t.Fatal("expected renamed=true")
	}
	if rr.absPath != newAbs {
		t.Fatalf("expected absPath=%q, got %q", newAbs, rr.absPath)
	}
	if rr.filePath != "tasks_New.md" {
		t.Fatalf("expected filePath=tasks_New.md, got %q", rr.filePath)
	}

	// Old file should be gone, new file should exist
	if _, err := os.Stat(oldAbs); !os.IsNotExist(err) {
		t.Fatal("old file should not exist after rename")
	}
	if _, err := os.Stat(newAbs); err != nil {
		t.Fatalf("new file should exist: %v", err)
	}

	// Registry should have the new path
	entry, found, err := registryLookup(regPath, "00000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatalf("registry lookup failed: %v", err)
	}
	if !found {
		t.Fatal("expected registry entry to exist")
	}
	if entry.FilePath != "tasks_New.md" {
		t.Fatalf("expected registry FilePath=tasks_New.md, got %q", entry.FilePath)
	}
	if !entry.IsAutoDelete {
		t.Fatal("expected IsAutoDelete=true in registry")
	}
}

func TestRenameTaskFile_CollisionReturnsAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	oldAbs := filepath.Join(dir, "tasks_Old.md")
	newAbs := filepath.Join(dir, "tasks_New.md")
	os.WriteFile(oldAbs, []byte("- [ ] old"), 0644)
	os.WriteFile(newAbs, []byte("- [ ] existing"), 0644)

	_, err := renameTaskFile(nopLogger(), renameParams{
		oldAbsPath:  oldAbs,
		newAbsPath:  newAbs,
		oldFilePath: "tasks_Old.md",
		newFilePath: "tasks_New.md",
	})
	if err == nil {
		t.Fatal("expected error for collision, got nil")
	}
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("expected CodeAlreadyExists, got %v", connect.CodeOf(err))
	}
}

func TestRenameTaskFile_SourceMissingReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	oldAbs := filepath.Join(dir, "tasks_Gone.md")
	newAbs := filepath.Join(dir, "tasks_New.md")

	_, err := renameTaskFile(nopLogger(), renameParams{
		oldAbsPath:  oldAbs,
		newAbsPath:  newAbs,
		oldFilePath: "tasks_Gone.md",
		newFilePath: "tasks_New.md",
	})
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

// ---------------------------------------------------------------------------
// persistTaskFile
// ---------------------------------------------------------------------------

func TestPersistTaskFile_WritesSuccessfully(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "tasks_Test.md")

	tasks := []MainTask{
		{Description: "task one"},
		{Description: "task two", Done: true},
	}

	err := persistTaskFile(nopLogger(), persistParams{
		absPath:  absPath,
		filePath: "tasks_Test.md",
		tasks:    tasks,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	parsed, err := ParseTaskFile(data)
	if err != nil {
		t.Fatalf("failed to parse written file: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(parsed))
	}
	if parsed[0].Description != "task one" {
		t.Fatalf("expected 'task one', got %q", parsed[0].Description)
	}
}

func TestPersistTaskFile_BadPathReturnsError(t *testing.T) {
	// Point to a directory that doesn't exist so the atomic write fails
	absPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "tasks_X.md")

	err := persistTaskFile(nopLogger(), persistParams{
		absPath:  absPath,
		filePath: "tasks_X.md",
		tasks:    []MainTask{{Description: "task"}},
	})
	if err == nil {
		t.Fatal("expected error for bad path, got nil")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInternal {
		t.Fatalf("expected CodeInternal, got %v", ce.Code())
	}
}

func TestPersistTaskFile_RollsBackRenameOnFailure(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)

	// Set up: simulate a rename already happened (old file gone, new file exists)
	origAbs := filepath.Join(dir, "tasks_Old.md")

	// Write the "renamed" file at the original location so rollback has something to restore to
	os.WriteFile(origAbs, []byte("- [ ] original"), 0644)
	// Rename it to simulate the rename step
	renamedDir := filepath.Join(dir, "renamed")
	os.MkdirAll(renamedDir, 0755)
	renamedAbsReal := filepath.Join(renamedDir, "tasks_New.md")
	os.Rename(origAbs, renamedAbsReal)

	// Put the new path in the registry
	registryAdd(regPath, "00000000-0000-4000-8000-aaaaaaaaaaaa", registryEntry{
		FilePath: "renamed/tasks_New.md", IsAutoDelete: false,
	})

	// Now call persistTaskFile with a path that will fail (nonexistent parent dir)
	err := persistTaskFile(nopLogger(), persistParams{
		absPath:      filepath.Join(dir, "no_such_dir", "tasks_New.md"),
		filePath:     "no_such_dir/tasks_New.md",
		origAbsPath:  renamedAbsReal,
		origFilePath: "renamed/tasks_New.md",
		regPath:      regPath,
		id:           "00000000-0000-4000-8000-aaaaaaaaaaaa",
		isAutoDelete: false,
		renamed:      true,
		tasks:        []MainTask{{Description: "task"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Registry should have been rolled back to the original path
	entry, found, _ := registryLookup(regPath, "00000000-0000-4000-8000-aaaaaaaaaaaa")
	if !found {
		t.Fatal("expected registry entry to still exist after rollback")
	}
	if entry.FilePath != "renamed/tasks_New.md" {
		t.Fatalf("expected registry to roll back to 'renamed/tasks_New.md', got %q", entry.FilePath)
	}
}
