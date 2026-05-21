package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
)

// IsSubPath checks that candidate is a strict child of base (prevents path traversal).
// It computes the relative path from base to candidate via filepath.Rel. 
// 
// filepath.Rel("/data", "/data/notes/foo.md")  → "notes/foo.md", nil
// filepath.Rel("/data", "/data")               → ".", nil
// filepath.Rel("/data", "/etc/passwd")         → "../etc/passwd", nil
//
// If the result contains ".." segments, the candidate escapes the base directory. 
// If it equals ".", they are the same path (not a strict child). 
// Only a clean relative path without ".." means the candidate is genuinely nested inside base.
func IsSubPath(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
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

// rejectBadRelativePath returns an InvalidArgument error if the path is not a
// clean relative path. The rules are:
//   - empty string is allowed (represents the data-directory root)
//   - must not start with "/"
//   - must not contain "\" (backslash)
//   - must not contain ".." segments
//   - must not contain null bytes
//   - must equal its own filepath.Clean result (no redundant slashes, no trailing slash)
func rejectBadRelativePath(p string) error {
	if p == "" {
		return nil
	}
	if p[0] == '/' {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path must not start with '/'"))
	}
	if strings.ContainsRune(p, '\\') {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path must not contain backslashes"))
	}
	if strings.ContainsRune(p, 0) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path must not contain null bytes"))
	}
	cleaned := filepath.Clean(p)
	if cleaned != p {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path is not clean: use %q", cleaned))
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path must not contain '..' segments"))
		}
	}
	return nil
}

// resolveSymlinks resolves symlinks on the longest existing prefix of path.
//
// This is used by validatePath to defeat symlink-based path traversal: a symlink
// inside the data directory could point outside of it, so we need the real
// filesystem location before checking containment with IsSubPath.
//
// We can't use filepath.EvalSymlinks directly because it fails when the target
// file doesn't exist yet. Since we often validate a path before creating the
// file, this function walks up the path until it finds an existing ancestor,
// resolves that, then re-appends the non-existent tail segments.
func resolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	var tail []string
	cur := path
	for {
		parent := filepath.Dir(cur)
		tail = append(tail, filepath.Base(cur))
		if parent == cur {
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

	for i := len(tail) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, tail[i])
	}
	return resolved, nil
}

// validatePath ensures a user-supplied relative path stays within the data directory.
// It combines syntactic validation (rejectBadRelativePath) with filesystem-level
// symlink resolution to prevent both logical and symlink-based path traversal.
// When allowRoot is true, the path may resolve to the data directory itself (used
// for parent_dir validation where "" means the root). Otherwise it must be a strict child.
//
// dataDir must already be resolved via ResolveDataDir at boot time.
func validatePath(dataDir, relativePath string, allowRoot bool) (string, error) {
	// Reject obviously malformed paths before touching the filesystem.
	err := rejectBadRelativePath(relativePath)
	if err != nil {
		return "", err
	}

	// Build the absolute path by joining the data directory with the cleaned relative path.
	cleanedPath := filepath.Join(dataDir, filepath.Clean(relativePath))

	// Resolve symlinks on the candidate to get its real filesystem location.
	// dataDir is already resolved at boot, so no per-request resolution needed.
	resolvedPath, err := resolveSymlinks(cleanedPath)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve path: %w", err))
	}

	// Check containment on the resolved (real) paths so symlinks can't escape.
	if allowRoot {
		if resolvedPath != dataDir && !IsSubPath(dataDir, resolvedPath) {
			return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
		}
	} else {
		if !IsSubPath(dataDir, resolvedPath) {
			return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
		}
	}
	return resolvedPath, nil
}

// ValidatePath ensures a path doesn't escape the data directory root.
func ValidatePath(dataDir, relativePath string) (string, error) {
	return validatePath(dataDir, relativePath, false)
}

// ValidateParentDir validates a directory path, allowing the data directory root itself.
func ValidateParentDir(dataDir, relativePath string) (string, error) {
	return validatePath(dataDir, relativePath, true)
}

// ValidateName checks that a user-supplied name is safe to use as a single path component.
func ValidateName(name string) error {
	if name == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not be empty"))
	}
	if len(name) > MaxNameLen {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name too long: %d bytes exceeds %d byte limit", len(name), MaxNameLen))
	}
	if strings.ContainsAny(name, "/\\") {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not contain path separators"))
	}
	if name == "." || name == ".." {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not be '.' or '..'"))
	}
	if strings.ContainsRune(name, 0) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not contain null bytes"))
	}
	return nil
}

// RequireDir stats path and verifies it is an existing directory.
func RequireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("directory does not exist"))
	}
	if !info.IsDir() {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("path is not a directory"))
	}
	return nil
}

// ValidateContentLength returns an InvalidArgument error if len(data) exceeds max.
func ValidateContentLength(data string, max int, field string) error {
	if len(data) > max {
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%s too large: %d bytes exceeds %d byte limit", field, len(data), max))
	}
	return nil
}
