package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) ListTaskLists(
	ctx context.Context,
	req *pb.ListTaskListsRequest,
) (*pb.ListTaskListsResponse, error) {
	parentDir := req.GetParentDir()

	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		s.logger.Error("failed to read directory", "path", req.GetParentDir(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	prefix := parentDir
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var taskLists []*pb.TaskList

	for _, e := range dirEntries {
		name := e.Name()

		if e.IsDir() || filepath.Ext(name) != common.TaskListFileType.Suffix || !strings.HasPrefix(name, common.TaskListFileType.Prefix) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat %s: %w", name, err))
		}

		entryPath := prefix + name
		listName, err := ExtractTaskListTitle(name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid task list filename %s: %w", name, err))
		}

		absPath := filepath.Join(dirPath, name)
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file %s: %w", name, err))
		}

		domainTasks, err := ParseTaskFile(data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file %s: %w", name, err))
		}

		taskLists = append(taskLists, buildTaskList("", entryPath, listName, domainTasks, info.ModTime().UnixMilli()))
	}

	// Read registry and build reverse map (filePath → id)
	regPath := registryPath(s.dataDir)
	registry, err := registryRead(regPath)
	if err != nil {
		s.logger.Error("failed to read registry", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}

	reverseMap := make(map[string]string, len(registry))
	for id, fp := range registry {
		reverseMap[fp] = id
	}

	for _, tl := range taskLists {
		tl.Id = reverseMap[tl.FilePath] // empty string if not found
	}

	return &pb.ListTaskListsResponse{TaskLists: taskLists}, nil
}
