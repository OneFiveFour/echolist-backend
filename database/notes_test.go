package database_test

import (
	"testing"

	"echolist-backend/database"
)

func TestInsertNote(t *testing.T) {
	db := newTestDB(t)

	params := database.InsertNoteParams{
		Id:        "note-insert-001",
		Title:     "My First Note",
		ParentDir: "Work",
		Preview:   "This is a preview",
		CreatedAt: 1000,
		UpdatedAt: 1000,
	}

	err := db.InsertNote(params)
	if err != nil {
		t.Fatalf("InsertNote: %v", err)
	}
}

func TestGetNote(t *testing.T) {
	db := newTestDB(t)

	params := database.InsertNoteParams{
		Id:        "note-get-001",
		Title:     "Get Note Test",
		ParentDir: "Personal",
		Preview:   "Preview content here",
		CreatedAt: 2000,
		UpdatedAt: 2500,
	}

	if err := db.InsertNote(params); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	note, err := db.GetNote("note-get-001")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if note.Id != params.Id {
		t.Errorf("Id: expected %q, got %q", params.Id, note.Id)
	}
	if note.Title != params.Title {
		t.Errorf("Title: expected %q, got %q", params.Title, note.Title)
	}
	if note.ParentDir != params.ParentDir {
		t.Errorf("ParentDir: expected %q, got %q", params.ParentDir, note.ParentDir)
	}
	if note.Preview != params.Preview {
		t.Errorf("Preview: expected %q, got %q", params.Preview, note.Preview)
	}
	if note.CreatedAt != params.CreatedAt {
		t.Errorf("CreatedAt: expected %d, got %d", params.CreatedAt, note.CreatedAt)
	}
	if note.UpdatedAt != params.UpdatedAt {
		t.Errorf("UpdatedAt: expected %d, got %d", params.UpdatedAt, note.UpdatedAt)
	}
}

func TestGetNote_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetNote("non-existent-id")
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestUpdateNote(t *testing.T) {
	db := newTestDB(t)

	params := database.InsertNoteParams{
		Id:        "note-update-001",
		Title:     "Original Title",
		ParentDir: "",
		Preview:   "Original preview",
		CreatedAt: 3000,
		UpdatedAt: 3000,
	}

	if err := db.InsertNote(params); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	err := db.UpdateNote("note-update-001", "Updated Title", "Updated preview", 4000)
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	note, err := db.GetNote("note-update-001")
	if err != nil {
		t.Fatalf("GetNote after update: %v", err)
	}

	if note.Title != "Updated Title" {
		t.Errorf("Title: expected %q, got %q", "Updated Title", note.Title)
	}
	if note.Preview != "Updated preview" {
		t.Errorf("Preview: expected %q, got %q", "Updated preview", note.Preview)
	}
	if note.UpdatedAt != 4000 {
		t.Errorf("UpdatedAt: expected 4000, got %d", note.UpdatedAt)
	}
	// CreatedAt should remain unchanged.
	if note.CreatedAt != 3000 {
		t.Errorf("CreatedAt: expected 3000 (unchanged), got %d", note.CreatedAt)
	}
}

func TestUpdateNote_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.UpdateNote("non-existent-id", "Title", "Preview", 5000)
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestDeleteNote(t *testing.T) {
	db := newTestDB(t)

	params := database.InsertNoteParams{
		Id:        "note-delete-001",
		Title:     "To Delete",
		ParentDir: "",
		Preview:   "Will be deleted",
		CreatedAt: 6000,
		UpdatedAt: 6000,
	}

	if err := db.InsertNote(params); err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	// First delete returns true.
	deleted, err := db.DeleteNote("note-delete-001")
	if err != nil {
		t.Fatalf("first DeleteNote: %v", err)
	}
	if !deleted {
		t.Error("expected first delete to return true")
	}

	// Second delete returns false.
	deleted, err = db.DeleteNote("note-delete-001")
	if err != nil {
		t.Fatalf("second DeleteNote: %v", err)
	}
	if deleted {
		t.Error("expected second delete to return false")
	}

	// GetNote returns ErrNotFound.
	_, err = db.GetNote("note-delete-001")
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestListNotes(t *testing.T) {
	db := newTestDB(t)

	// Insert 3 notes in the same parent_dir.
	for i, id := range []string{"note-list-001", "note-list-002", "note-list-003"} {
		params := database.InsertNoteParams{
			Id:        id,
			Title:     "Note " + id,
			ParentDir: "SharedDir",
			Preview:   "Preview for " + id,
			CreatedAt: int64(7000 + i),
			UpdatedAt: int64(7000 + i),
		}
		if err := db.InsertNote(params); err != nil {
			t.Fatalf("InsertNote %s: %v", id, err)
		}
	}

	notes, err := db.ListNotes("SharedDir")
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
}

func TestListNotes_FiltersByParentDir(t *testing.T) {
	db := newTestDB(t)

	dirs := []string{"", "Work", "Personal"}
	for i, dir := range dirs {
		params := database.InsertNoteParams{
			Id:        "note-filter-" + dir + "-001",
			Title:     "Note in " + dir,
			ParentDir: dir,
			Preview:   "Preview",
			CreatedAt: int64(8000 + i),
			UpdatedAt: int64(8000 + i),
		}
		if err := db.InsertNote(params); err != nil {
			t.Fatalf("InsertNote in %q: %v", dir, err)
		}
	}

	// Add a second note in "Work".
	params := database.InsertNoteParams{
		Id:        "note-filter-Work-002",
		Title:     "Another Work Note",
		ParentDir: "Work",
		Preview:   "Preview 2",
		CreatedAt: 8010,
		UpdatedAt: 8010,
	}
	if err := db.InsertNote(params); err != nil {
		t.Fatalf("InsertNote second Work: %v", err)
	}

	// ListNotes("Work") should return only the Work ones.
	notes, err := db.ListNotes("Work")
	if err != nil {
		t.Fatalf("ListNotes(Work): %v", err)
	}

	if len(notes) != 2 {
		t.Fatalf("expected 2 notes in Work, got %d", len(notes))
	}

	for _, n := range notes {
		if n.ParentDir != "Work" {
			t.Errorf("expected ParentDir %q, got %q", "Work", n.ParentDir)
		}
	}
}
