package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	filev1 "echolist-backend/proto/gen/file/v1"
)

// entryPath builds the response path for a FileEntry.
// If requestParentDir is empty, returns name.
// Otherwise returns requestParentDir + "/" + name.
func entryPath(requestParentDir, name string) string {
	if requestParentDir == "" {
		return name
	}
	if requestParentDir[len(requestParentDir)-1] == '/' {
		return requestParentDir + name
	}
	return requestParentDir + "/" + name
}

// buildFolderEntry creates a FileEntry for a directory.
// child_count = number of subdirectories (from os.ReadDir) + notes and task lists (from SQLite).
func (s *FileServer) buildFolderEntry(absPath, name, requestParentDir string) *filev1.FileEntry {
	childCount := int32(0)

	// Compute the relative path for this subdirectory.
	folderRelPath := entryPath(requestParentDir, name)

	// Count subdirectories on disk.
	entries, err := os.ReadDir(absPath)
	if err != nil {
		s.logger.Warn("failed to read subdirectory for child count", "path", absPath, "error", err)
	} else {
		for _, e := range entries {
			if e.IsDir() {
				childCount++
			}
		}
	}

	// Count notes + task lists in SQLite for this subdirectory.
	dbCount, err := s.db.CountChildrenInDir(folderRelPath)
	if err != nil {
		s.logger.Warn("failed to count DB children in dir", "path", folderRelPath, "error", err)
	} else {
		childCount += int32(dbCount)
	}

	return &filev1.FileEntry{
		Path:     entryPath(requestParentDir, name),
		Title:    name,
		ItemType: filev1.ItemType_ITEM_TYPE_FOLDER,
		Metadata: &filev1.FileEntry_FolderMetadata{
			FolderMetadata: &filev1.FolderMetadata{
				ChildCount: childCount,
			},
		},
	}
}

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

	requestParentDir := req.GetParentDir()

	parentDir, err := common.ValidateParentDir(s.dataDir, requestParentDir)
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(parentDir, "parent directory"); err != nil {
		return nil, err
	}

	// Read directory entries — only process directories (folders).
	dirEntries, err := os.ReadDir(parentDir)
	if err != nil {
		s.logger.Error("failed to read directory", "path", req.GetParentDir(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var result []*filev1.FileEntry

	// Build folder entries from filesystem directories.
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		absPath := filepath.Join(parentDir, name)
		result = append(result, s.buildFolderEntry(absPath, name, requestParentDir))
	}

	// Query SQLite for task lists in this directory.
	taskLists, err := s.db.ListTaskListsWithCounts(requestParentDir)
	if err != nil {
		s.logger.Error("failed to query task lists", "parentDir", requestParentDir, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query task lists: %w", err))
	}
	for _, row := range taskLists {
		result = append(result, &filev1.FileEntry{
			Path:     entryPath(requestParentDir, row.Title),
			Title:    row.Title,
			ItemType: filev1.ItemType_ITEM_TYPE_TASK_LIST,
			Metadata: &filev1.FileEntry_TaskListMetadata{
				TaskListMetadata: &filev1.TaskListMetadata{
					Id:             row.Id,
					UpdatedAt:      row.UpdatedAt,
					TotalTaskCount: int32(row.TotalTaskCount),
					DoneTaskCount:  int32(row.DoneTaskCount),
				},
			},
		})
	}

	// Query SQLite for notes in this directory.
	noteRows, err := s.db.ListNotes(requestParentDir)
	if err != nil {
		s.logger.Error("failed to query notes", "parentDir", requestParentDir, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query notes: %w", err))
	}
	for _, row := range noteRows {
		// Compute the note filename from the NoteRow fields.
		noteFilename := row.Title + "_" + row.Id + ".md"
		result = append(result, &filev1.FileEntry{
			Path:     entryPath(requestParentDir, noteFilename),
			Title:    row.Title,
			ItemType: filev1.ItemType_ITEM_TYPE_NOTE,
			Metadata: &filev1.FileEntry_NoteMetadata{
				NoteMetadata: &filev1.NoteMetadata{
					Id:        row.Id,
					UpdatedAt: row.UpdatedAt,
					Preview:   row.Preview,
				},
			},
		})
	}

	return &filev1.ListFilesResponse{Entries: result}, nil
}
