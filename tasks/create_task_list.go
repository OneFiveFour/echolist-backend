package tasks

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) CreateTaskList(
	ctx context.Context,
	req *pb.CreateTaskListRequest,
) (*pb.CreateTaskListResponse, error) {
	parentDir := req.GetParentDir()

	// Validate path
	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}
	for i, t := range domainTasks {
		if t.Recurrence != "" {
			next, err := ComputeNextDueDate(t.Recurrence, time.Now())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			domainTasks[i].DueDate = next.Format("2006-01-02")
		}
	}

	if err := common.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	filename := common.TaskListFileType.Prefix + title + common.TaskListFileType.Suffix
	absPath := filepath.Join(dirPath, filename)

	unlock := s.locks.Lock(absPath)
	defer unlock()

	// Use exclusive create to avoid TOCTOU race between existence check and write.
	data := PrintTaskFile(domainTasks)
	if err := common.CreateExclusive(absPath, data); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("task list already exists"))
		}
		s.logger.Error("failed to write task file", "path", absPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}

	// Generate UUIDv4
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		s.logger.Error("failed to generate UUID", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate UUID: %w", err))
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant bits
	id := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])

	// Persist id→filePath mapping in the registry
	relPath := filepath.Join(parentDir, filename)
	regPath := registryPath(s.dataDir)
	unlockReg := s.locks.Lock(regPath)
	defer unlockReg()

	if err := registryAdd(regPath, id, relPath); err != nil {
		s.logger.Error("failed to add registry entry", "id", id, "path", relPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist task list id: %w", err))
	}

	return &pb.CreateTaskListResponse{
		TaskList: buildTaskList(id, relPath, title, domainTasks, nowMillis()),
	}, nil
}
