package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
)

// IsSubPath checks that resolved is a child of base (prevents path traversal).
func IsSubPath(base, resolved string) bool {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

// ValidatePath ensures a path doesn't escape the data directory root.
// Returns the cleaned absolute path or an error.
func ValidatePath(dataDir, relativePath string) (string, error) {
	cleaned := filepath.Join(dataDir, filepath.Clean(relativePath))
	if !IsSubPath(dataDir, cleaned) {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
	}
	return cleaned, nil
}
