package file

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	filev1 "echolist-backend/proto/gen/file/v1"
)

func TestUpdateFolder_NonExistent(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
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
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.DeleteFolder(context.Background(), &filev1.DeleteFolderRequest{
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
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.DeleteFolder(context.Background(), &filev1.DeleteFolderRequest{
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

func TestUpdateFolder_EmptyPath(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
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

func TestUpdateFolder_PathTraversal(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.UpdateFolder(context.Background(), &filev1.UpdateFolderRequest{
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
	srv := NewFileServer(dataDir, nopLogger())
	_, err := srv.DeleteFolder(context.Background(), &filev1.DeleteFolderRequest{
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
