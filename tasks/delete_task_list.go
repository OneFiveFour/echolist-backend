package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) DeleteTaskList(
	ctx context.Context,
	req *pb.DeleteTaskListRequest,
) (*pb.DeleteTaskListResponse, error) {

	// Validate the id field before any filesystem operations (Req 9.1, 9.2)
	if err := validateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	// Acquire registry lock first, then resolve ID (Req 6.1, 6.2)
	regPath := registryPath(s.dataDir)
	unlockReg := s.locks.Lock(regPath)
	defer unlockReg()

	filePath, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
	}

	// Validate the resolved path doesn't escape the data directory
	absPath, err := common.ValidatePath(s.dataDir, filePath)
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.TaskListFileType); err != nil {
		return nil, err
	}

	// Acquire task list file lock
	unlockFile := s.locks.Lock(absPath)
	defer unlockFile()

	// Delete the file
	if err := os.Remove(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found: %w", err))
		}
		s.logger.Error("failed to delete task file", "id", req.GetId(), "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete task file: %w", err))
	}

	// Remove the registry entry (Req 2.2)
	if err := registryRemove(regPath, req.GetId()); err != nil {
		s.logger.Error("failed to remove registry entry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove registry entry: %w", err))
	}

	return &pb.DeleteTaskListResponse{}, nil
}
