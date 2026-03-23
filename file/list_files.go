package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	filev1 "echolist-backend/proto/gen/file/v1"
	"echolist-backend/tasks"
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
// It reads the subdirectory and counts recognized children (folders, notes, task lists).
// On ReadDir error, logs a warning and sets child_count to 0.
func (s *FileServer) buildFolderEntry(absPath, name, requestParentDir string) *filev1.FileEntry {
	childCount := int32(0)
	
	entries, err := os.ReadDir(absPath)
	if err != nil {
		s.logger.Warn("failed to read subdirectory for child count", "path", absPath, "error", err)
	} else {
		for _, e := range entries {
			entryName := e.Name()
			if e.IsDir() {
				childCount++
			} else if common.MatchesFileType(entryName, common.NoteFileType) || common.MatchesFileType(entryName, common.TaskListFileType) {
				childCount++
			}
		}
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

// buildNoteEntry creates a FileEntry for a note file.
// It stats the file for updated_at, reads content for preview (first 100 characters, rune-safe),
// and extracts the title. On I/O errors, logs a warning and uses zero/empty values.
func (s *FileServer) buildNoteEntry(absPath, name, requestParentDir string) *filev1.FileEntry {
	var updatedAt int64
	var preview string
	
	title, err := common.ExtractTitle(name, common.NoteFileType.Prefix, common.NoteFileType.Suffix, common.NoteFileType.Label)
	if err != nil {
		s.logger.Warn("failed to extract note title", "path", absPath, "error", err)
		title = name
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.logger.Warn("failed to stat note file", "path", absPath, "error", err)
	} else {
		updatedAt = info.ModTime().UnixMilli()
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Warn("failed to read note content", "path", absPath, "error", err)
	} else {
		runes := []rune(string(content))
		if len(runes) > 100 {
			preview = string(runes[:100])
		} else {
			preview = string(runes)
		}
	}

	return &filev1.FileEntry{
		Path:     entryPath(requestParentDir, name),
		Title:    title,
		ItemType: filev1.ItemType_ITEM_TYPE_NOTE,
		Metadata: &filev1.FileEntry_NoteMetadata{
			NoteMetadata: &filev1.NoteMetadata{
				UpdatedAt: updatedAt,
				Preview:   preview,
			},
		},
	}
}

// buildTaskListEntry creates a FileEntry for a task list file.
// It stats the file for updated_at, reads and parses the file to count total and done MainTasks,
// and extracts the title. On I/O or parse errors, logs a warning and uses zero values.
func (s *FileServer) buildTaskListEntry(absPath, name, requestParentDir string) *filev1.FileEntry {
	var updatedAt int64
	var totalTaskCount, doneTaskCount int32
	
	title, err := common.ExtractTitle(name, common.TaskListFileType.Prefix, common.TaskListFileType.Suffix, common.TaskListFileType.Label)
	if err != nil {
		s.logger.Warn("failed to extract task list title", "path", absPath, "error", err)
		title = name
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.logger.Warn("failed to stat task list file", "path", absPath, "error", err)
	} else {
		updatedAt = info.ModTime().UnixMilli()
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Warn("failed to read task list content", "path", absPath, "error", err)
	} else {
		mainTasks, err := tasks.ParseTaskFile(content)
		if err != nil {
			s.logger.Warn("failed to parse task list", "path", absPath, "error", err)
		} else {
			totalTaskCount = int32(len(mainTasks))
			for _, task := range mainTasks {
				if task.Done {
					doneTaskCount++
				}
			}
		}
	}

	return &filev1.FileEntry{
		Path:     entryPath(requestParentDir, name),
		Title:    title,
		ItemType: filev1.ItemType_ITEM_TYPE_TASK_LIST,
		Metadata: &filev1.FileEntry_TaskListMetadata{
			TaskListMetadata: &filev1.TaskListMetadata{
				UpdatedAt:      updatedAt,
				TotalTaskCount: totalTaskCount,
				DoneTaskCount:  doneTaskCount,
			},
		},
	}
}

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

	parentDir, err := common.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(parentDir, "parent directory"); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		s.logger.Error("failed to read directory", "path", req.GetParentDir(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var result []*filev1.FileEntry
	for _, e := range entries {
		name := e.Name()
		absPath := filepath.Join(parentDir, name)
		
		if e.IsDir() {
			result = append(result, s.buildFolderEntry(absPath, name, req.GetParentDir()))
		} else if common.MatchesFileType(name, common.NoteFileType) {
			result = append(result, s.buildNoteEntry(absPath, name, req.GetParentDir()))
		} else if common.MatchesFileType(name, common.TaskListFileType) {
			result = append(result, s.buildTaskListEntry(absPath, name, req.GetParentDir()))
		}
		// Skip unrecognized files
	}
	
	return &filev1.ListFilesResponse{Entries: result}, nil
}
