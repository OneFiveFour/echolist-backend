package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) ListTaskLists(
	ctx context.Context,
	req *pb.ListTaskListsRequest,
) (*pb.ListTaskListsResponse, error) {
	dirPath := filepath.Clean(filepath.Join(s.dataDir, req.GetPath()))
	if dirPath != s.dataDir && !pathutil.IsSubPath(s.dataDir, dirPath) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
	}

	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	prefix := req.GetPath()
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var taskLists []*pb.TaskListEntry
	var entries []string

	for _, e := range dirEntries {
		name := e.Name()

		if e.IsDir() {
			entries = append(entries, prefix+name+"/")
			continue
		}

		if filepath.Ext(name) != ".md" || !strings.HasPrefix(name, "tasks_") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat %s: %w", name, err))
		}

		entryPath := prefix + name
		entries = append(entries, entryPath)
		listName := strings.TrimPrefix(strings.TrimSuffix(name, ".md"), "tasks_")

		taskLists = append(taskLists, &pb.TaskListEntry{
			FilePath:  entryPath,
			Name:      listName,
			UpdatedAt: info.ModTime().UnixMilli(),
		})
	}

	return &pb.ListTaskListsResponse{TaskLists: taskLists, Entries: entries}, nil
}
