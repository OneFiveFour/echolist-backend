package folder

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	folderv1 "echolist-backend/proto/gen/folder/v1"
)

// Unit tests for error conditions
// **Validates: Requirements 1.5, 2.3, 3.2, 3.3**

func TestRenameFolder_NonExistent(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.RenameFolder(context.Background(), &folderv1.RenameFolderRequest{
		FolderPath: "nonexistent",
		NewName:    "newname",
	})
	if err == nil {
		t.Fatal("expected error for non-existent folder")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeNotFound {
			t.Fatalf("expected NotFound, got %v", connErr.Code())
		}
	}
}

func TestDeleteFolder_NonExistent(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.DeleteFolder(context.Background(), &folderv1.DeleteFolderRequest{
		FolderPath: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent folder")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeNotFound {
			t.Fatalf("expected NotFound, got %v", connErr.Code())
		}
	}
}

func TestDeleteFolder_EmptyPath(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.DeleteFolder(context.Background(), &folderv1.DeleteFolderRequest{
		FolderPath: "",
	})
	if err == nil {
		t.Fatal("expected error for empty folder path (deleting root)")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", connErr.Code())
		}
	}
}

func TestRenameFolder_EmptyPath(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.RenameFolder(context.Background(), &folderv1.RenameFolderRequest{
		FolderPath: "",
		NewName:    "newname",
	})
	if err == nil {
		t.Fatal("expected error for empty folder path")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", connErr.Code())
		}
	}
}

func TestRenameFolder_PathTraversal(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.RenameFolder(context.Background(), &folderv1.RenameFolderRequest{
		FolderPath: "../../etc",
		NewName:    "hacked",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", connErr.Code())
		}
	}
}

func TestDeleteFolder_PathTraversal(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFolderServer(dataDir)

	_, err := srv.DeleteFolder(context.Background(), &folderv1.DeleteFolderRequest{
		FolderPath: "../../etc",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		if connErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", connErr.Code())
		}
	}
}
