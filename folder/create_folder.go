package folder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	folderv1 "echolist-backend/proto/gen/folder/v1"
)

func (s *FolderServer) CreateFolder(
	ctx context.Context,
	req *folderv1.CreateFolderRequest,
) (*folderv1.CreateFolderResponse, error) {
	if err := validateName(req.GetName()); err != nil {
		return nil, err
	}

	// Resolve parent directory
	parentDir := filepath.Join(s.dataDir, req.GetDomain(), req.GetParentPath())
	parentDir = filepath.Clean(parentDir)
	domainRoot := filepath.Join(s.dataDir, req.GetDomain())

	// Ensure parent is within the domain root (or is the root itself)
	if parentDir != domainRoot && !isSubPath(domainRoot, parentDir) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parent path escapes domain root"))
	}

	// Check parent exists
	info, err := os.Stat(parentDir)
	if err != nil || !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("parent directory does not exist"))
	}

	// Check case-insensitive duplicates
	existing, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read parent directory: %w", err))
	}
	for _, e := range existing {
		if strings.EqualFold(e.Name(), req.GetName()) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists (case-insensitive)"))
		}
	}

	// Create the folder
	newDir := filepath.Join(parentDir, req.GetName())
	if err := os.Mkdir(newDir, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create folder: %w", err))
	}

	// Return parent listing
	entries, err := listDirectory(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &folderv1.CreateFolderResponse{Entries: entries}, nil
}
