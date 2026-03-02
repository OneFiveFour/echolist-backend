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

// ValidateName checks that a user-supplied name is safe to use as a single
// path component.  It rejects empty strings, path separators, dot-entries,
// and null bytes.
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


// FileType represents a known file type with its naming convention.
type FileType struct {
	Prefix string // e.g. "note_", "tasks_"
	Suffix string // e.g. ".md"
	Label  string // human-readable label for error messages, e.g. "note"
}

// Predefined file types used across the application.
var (
	NoteFileType     = FileType{Prefix: "note_", Suffix: ".md", Label: "note"}
	TaskListFileType = FileType{Prefix: "tasks_", Suffix: ".md", Label: "task list"}
)

// ValidateFileType checks that the file at absPath exists, is a regular file
// (not a directory), and matches the expected naming convention.
// Returns a connect-coded error suitable for direct use in RPC handlers.
func ValidateFileType(absPath string, ft FileType) error {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("%s not found", ft.Label))
		}
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat %s: %w", ft.Label, err))
	}
	if info.IsDir() {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path is a directory, not a %s", ft.Label))
	}
	name := filepath.Base(absPath)
	if !strings.HasPrefix(name, ft.Prefix) || !strings.HasSuffix(name, ft.Suffix) || len(name) <= len(ft.Prefix)+len(ft.Suffix) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file is not a %s", ft.Label))
	}
	return nil
}
// ExtractTitle extracts the human-readable title from a filename by stripping
// the given prefix and suffix. Returns an error if the filename is too short
// or doesn't match the expected pattern.
func ExtractTitle(filename, prefix, suffix, label string) (string, error) {
	if len(filename) < len(prefix)+len(suffix)+1 {
		return "", fmt.Errorf("filename too short to extract %s title: %q", label, filename)
	}
	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
		return "", fmt.Errorf("filename does not match %s pattern: %q", label, filename)
	}
	return filename[len(prefix) : len(filename)-len(suffix)], nil
}

// RequireDir stats path and verifies it is an existing directory.
// label is used in error messages (e.g. "parent directory", "folder").
// Returns a connect-coded NotFound error if the path is missing or not a dir.
func RequireDir(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("%s does not exist", label))
	}
	if !info.IsDir() {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("%s is not a directory", label))
	}
	return nil
}
// Content size limits for per-field validation.
const (
	MaxNoteContentBytes       = 1 << 20 // 1 MiB
	MaxNameLen                = 255
	MaxTaskDescriptionBytes   = 1024
	MaxSubtaskDescriptionBytes = 1024
	MaxTasksPerList           = 1000
	MaxSubtasksPerTask        = 100
)

// ValidateContentLength returns an InvalidArgument error if len(data) exceeds max.
// field is used in the error message (e.g. "content", "description").
func ValidateContentLength(data string, max int, field string) error {
	if len(data) > max {
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%s too large: %d bytes exceeds %d byte limit", field, len(data), max))
	}
	return nil
}


