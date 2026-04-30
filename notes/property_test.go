package notes_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/database"
	"echolist-backend/notes"
	pb "echolist-backend/proto/gen/notes/v1"

	"pgregory.net/rapid"
)

// --- Generators ---

// nameGen generates valid note titles (alphanumeric + hyphens/underscores, 1-30 chars).
func nameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,29}`)
}

// contentGen generates valid note content (0-500 chars).
func contentGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9 \n]{0,500}`)
}

// longContentGen generates content with > 100 runes (101-300 runes).
func longContentGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9 ]{101,300}`)
}

// invalidUuidGen generates strings that are NOT valid UUIDv4.
func invalidUuidGen() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.Just(""),
		rapid.StringMatching(`[a-zA-Z0-9]{1,30}`),
		rapid.StringMatching(`[0-9a-f]{32}`),
	)
}

// traversalPathGen generates path traversal strings.
func traversalPathGen() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"../etc/passwd", "../../secret", "foo/../../bar", "../", "foo/../../../etc",
	})
}

// --- Property Tests ---

// Feature: test-suite-sqlite-rewrite, Property 2: Note Create-Then-Get Round Trip
// Validates: Requirements 2.1, 8.1
func TestProperty_NoteCreateThenGetRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		name := nameGen().Draw(rt, "name")
		content := contentGen().Draw(rt, "content")

		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     name,
			Content:   content,
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		getResp, err := srv.GetNote(ctx, &pb.GetNoteRequest{
			Id: createResp.Note.Id,
		})
		if err != nil {
			rt.Fatalf("GetNote failed: %v", err)
		}

		got := getResp.Note

		// Same title
		if got.Title != name {
			rt.Fatalf("title mismatch: got %q, want %q", got.Title, name)
		}

		// Same content
		if got.Content != content {
			rt.Fatalf("content mismatch: got %q, want %q", got.Content, content)
		}

		// Same ID
		if got.Id != createResp.Note.Id {
			rt.Fatalf("ID mismatch: got %q, want %q", got.Id, createResp.Note.Id)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 3: All Generated IDs Are Valid UUIDv4
// Validates: Requirements 4.1, 4.2
func TestProperty_NoteIDsAreValidUUIDv4(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		name := nameGen().Draw(rt, "name")
		content := contentGen().Draw(rt, "content")

		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     name,
			Content:   content,
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		// Verify Note.Id is valid UUIDv4
		if err := common.ValidateUuidV4(createResp.Note.Id); err != nil {
			rt.Fatalf("Note.Id is not valid UUIDv4: %v", err)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 7: Path Traversal Prevention
// Validates: Requirements 8.5
func TestProperty_NotePathTraversalPrevention(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		traversal := traversalPathGen().Draw(rt, "traversal")

		// CreateNote with traversal parent_dir
		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     "test",
			Content:   "content",
			ParentDir: traversal,
		})
		if err == nil {
			rt.Fatalf("CreateNote should reject traversal path %q", traversal)
		}
		code := connect.CodeOf(err)
		if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			rt.Fatalf("CreateNote(%q): expected InvalidArgument or NotFound, got %v", traversal, code)
		}

		// ListNotes with traversal parent_dir
		_, err = srv.ListNotes(ctx, &pb.ListNotesRequest{
			ParentDir: traversal,
		})
		if err == nil {
			rt.Fatalf("ListNotes should reject traversal path %q", traversal)
		}
		code = connect.CodeOf(err)
		if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			rt.Fatalf("ListNotes(%q): expected InvalidArgument or NotFound, got %v", traversal, code)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 8: Invalid UUID Rejection
// Validates: Requirements 8.6
func TestProperty_NoteInvalidUUIDRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		invalidID := invalidUuidGen().Draw(rt, "invalidUUID")

		// GetNote with invalid UUID
		_, err := srv.GetNote(ctx, &pb.GetNoteRequest{Id: invalidID})
		if err == nil {
			rt.Fatalf("GetNote should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("GetNote(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}

		// UpdateNote with invalid UUID
		_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      invalidID,
			Title:   "Valid",
			Content: "content",
		})
		if err == nil {
			rt.Fatalf("UpdateNote should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("UpdateNote(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}

		// DeleteNote with invalid UUID
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{Id: invalidID})
		if err == nil {
			rt.Fatalf("DeleteNote should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("DeleteNote(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 9: Note Preview Computation
// Validates: Requirements 5.1, 5.2, 5.3, 5.4
func TestProperty_NotePreviewComputation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		db := notes.NewTestDB(t)
		srv := notes.NewNotesServer(dataDir, db, notes.NopLogger())
		ctx := context.Background()

		// Test with long content (> 100 runes)
		longContent := longContentGen().Draw(rt, "longContent")

		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     nameGen().Draw(rt, "name"),
			Content:   longContent,
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		// Query the DB directly to check the preview field
		noteRow, err := db.GetNote(createResp.Note.Id)
		if err != nil {
			rt.Fatalf("db.GetNote failed: %v", err)
		}

		// Preview should be first 100 runes of content
		runes := []rune(longContent)
		expectedPreview := string(runes[:100])
		if noteRow.Preview != expectedPreview {
			rt.Fatalf("preview mismatch for long content:\ngot:  %q\nwant: %q", noteRow.Preview, expectedPreview)
		}

		// Verify the content is indeed > 100 runes
		if utf8.RuneCountInString(longContent) <= 100 {
			rt.Fatalf("longContent should have > 100 runes, got %d", utf8.RuneCountInString(longContent))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 10: Note File Path Convention
// Validates: Requirements 2.4, 12.1, 12.2, 12.3
func TestProperty_NoteFilePathConvention(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		name := nameGen().Draw(rt, "name")
		content := contentGen().Draw(rt, "content")

		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     name,
			Content:   content,
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		id := createResp.Note.Id

		// Verify file exists at expected path
		expectedPath := database.NotePath("", name, id)
		absPath := filepath.Join(dataDir, expectedPath)
		if _, err := os.Stat(absPath); err != nil {
			rt.Fatalf("file not found at expected path %q: %v", expectedPath, err)
		}

		// Update title to a new name
		newName := nameGen().Draw(rt, "newName")
		_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      id,
			Title:   newName,
			Content: content,
		})
		if err != nil {
			rt.Fatalf("UpdateNote failed: %v", err)
		}

		// Verify old file is gone (unless names are the same)
		if newName != name {
			if _, err := os.Stat(absPath); !os.IsNotExist(err) {
				rt.Fatalf("old file should not exist at %q after rename", expectedPath)
			}
		}

		// Verify new file exists at updated path
		newExpectedPath := database.NotePath("", newName, id)
		newAbsPath := filepath.Join(dataDir, newExpectedPath)
		if _, err := os.Stat(newAbsPath); err != nil {
			rt.Fatalf("file not found at new path %q: %v", newExpectedPath, err)
		}

		// Delete the note
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{Id: id})
		if err != nil {
			rt.Fatalf("DeleteNote failed: %v", err)
		}

		// Verify file no longer exists
		if _, err := os.Stat(newAbsPath); !os.IsNotExist(err) {
			rt.Fatalf("file should not exist at %q after delete", newExpectedPath)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 15: Delete-Then-Get Returns NotFound
// Validates: Requirements 1.4
func TestProperty_NoteDeleteThenGetReturnsNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		name := nameGen().Draw(rt, "name")
		content := contentGen().Draw(rt, "content")

		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:     name,
			Content:   content,
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		id := createResp.Note.Id

		// Delete
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{Id: id})
		if err != nil {
			rt.Fatalf("DeleteNote failed: %v", err)
		}

		// Get should return NotFound
		_, err = srv.GetNote(ctx, &pb.GetNoteRequest{Id: id})
		if err == nil {
			rt.Fatal("GetNote should return error after delete")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 17: ListNotes Parent Dir Filtering
// Validates: Requirements 8.1
func TestProperty_ListNotesParentDirFiltering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
		ctx := context.Background()

		// Generate 2-3 unique directory names
		numDirs := rapid.IntRange(2, 3).Draw(rt, "numDirs")
		dirs := make([]string, numDirs)
		for i := 0; i < numDirs; i++ {
			for {
				dir := nameGen().Draw(rt, fmt.Sprintf("dir-%d", i))
				// Ensure uniqueness
				unique := true
				for j := 0; j < i; j++ {
					if strings.EqualFold(dirs[j], dir) {
						unique = false
						break
					}
				}
				if unique {
					dirs[i] = dir
					break
				}
			}
		}

		// Create directories on disk
		for _, dir := range dirs {
			if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
				rt.Fatalf("failed to create dir %q: %v", dir, err)
			}
		}

		// Create notes in each directory (1-2 per dir)
		expectedCounts := make(map[string]int)
		for _, dir := range dirs {
			numNotes := rapid.IntRange(1, 2).Draw(rt, fmt.Sprintf("numNotes-%s", dir))
			for j := 0; j < numNotes; j++ {
				name := nameGen().Draw(rt, fmt.Sprintf("name-%s-%d", dir, j))
				content := contentGen().Draw(rt, fmt.Sprintf("content-%s-%d", dir, j))
				_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
					Title:     name,
					Content:   content,
					ParentDir: dir,
				})
				if err != nil {
					rt.Fatalf("CreateNote in %q failed: %v", dir, err)
				}
				expectedCounts[dir]++
			}
		}

		// Verify each directory returns only its own notes
		for _, dir := range dirs {
			listResp, err := srv.ListNotes(ctx, &pb.ListNotesRequest{
				ParentDir: dir,
			})
			if err != nil {
				rt.Fatalf("ListNotes(%q) failed: %v", dir, err)
			}
			if len(listResp.Notes) != expectedCounts[dir] {
				rt.Fatalf("ListNotes(%q): expected %d, got %d", dir, expectedCounts[dir], len(listResp.Notes))
			}
		}
	})
}
