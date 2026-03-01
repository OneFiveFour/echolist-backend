package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
)

// IsSubPath checks that resolved is a strict child of base (prevents path traversal).
// It checks every segment of the relative path for ".." components, not just the prefix.
func IsSubPath(base, resolved string) bool {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	if rel == "." {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == ".." {
			return false
		}
	}
	return true
}

// resolveSymlinks resolves symlinks on the longest existing prefix of path.
// For paths where trailing components don't exist yet (e.g. new nested dirs),
// it walks up until it finds an existing ancestor, resolves that, then
// re-appends the non-existent tail.
func resolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	// Walk up collecting non-existent segments until we find something real.
	var tail []string
	cur := path
	for {
		parent := filepath.Dir(cur)
		tail = append(tail, filepath.Base(cur))
		if parent == cur {
			// Reached filesystem root without finding an existing path.
			return "", fmt.Errorf("no existing ancestor for path %q", path)
		}
		cur = parent
		resolved, err = filepath.EvalSymlinks(cur)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}

	// Rebuild: resolved ancestor + tail segments in reverse order.
	for i := len(tail) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, tail[i])
	}
	return resolved, nil
}

// ValidatePath ensures a path doesn't escape the data directory root.
// Returns the cleaned, symlink-resolved absolute path or an error.
func ValidatePath(dataDir, relativePath string) (string, error) {
	cleaned := filepath.Join(dataDir, filepath.Clean(relativePath))

	resolved, err := resolveSymlinks(cleaned)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve path: %w", err))
	}

	resolvedBase, err := resolveSymlinks(dataDir)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("cannot resolve data directory: %w", err))
	}

	if !IsSubPath(resolvedBase, resolved) {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
	}
	return resolved, nil
}

// ValidateParentDir validates a directory path, allowing the data directory root itself.
// Unlike ValidatePath, this permits relativePath="" which resolves to dataDir.
func ValidateParentDir(dataDir, relativePath string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(dataDir, relativePath))

	resolved, err := resolveSymlinks(cleaned)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve path: %w", err))
	}

	resolvedBase, err := resolveSymlinks(dataDir)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("cannot resolve data directory: %w", err))
	}

	if resolved != resolvedBase && !IsSubPath(resolvedBase, resolved) {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
	}
	return resolved, nil
}
