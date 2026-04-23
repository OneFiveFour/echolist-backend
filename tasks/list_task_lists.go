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

		taskLists = append(taskLists, buildTaskList("", parentDir, listName, domainTasks, info.ModTime().UnixMilli(), false))
	}

	// Read registry and build reverse map (filePath → id)
	regPath := registryPath(s.dataDir)
	registry, err := registryRead(regPath)
	if err != nil {
		s.logger.Error("failed to read registry", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}

	reverseMap := make(map[string]registryEntry, len(registry))
	for id, entry := range registry {
		reverseMap[entry.FilePath] = registryEntry{FilePath: id, IsAutoDelete: entry.IsAutoDelete}
	}

	for i, tl := range taskLists {
		// Reconstruct the file path used as registry key
		fileName := common.TaskListFileType.Prefix + tl.Title + common.TaskListFileType.Suffix
		var filePath string
		if parentDir == "" {
			filePath = fileName
		} else {
			filePath = parentDir + "/" + fileName
		}
		if re, ok := reverseMap[filePath]; ok {
			taskLists[i].Id = re.FilePath
			taskLists[i].IsAutoDelete = re.IsAutoDelete
		}
	}

	return &pb.ListTaskListsResponse{TaskLists: taskLists}, nil
}
