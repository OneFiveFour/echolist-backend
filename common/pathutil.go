package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
)

// IsSubPath checks that resolved is a strict child of base (prevents path traversal).
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
// NormalizeRelativePath strips leading slashes and cleans a relative path so
// that the data-directory root is always represented as "" rather than "/" or ".".
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

func validatePath(dataDir, relativePath string, allowRoot bool) (string, error) {
	if err := rejectBadRelativePath(relativePath); err != nil {
		return "", err
	}

	cleaned := filepath.Join(dataDir, filepath.Clean(relativePath))

	resolved, err := resolveSymlinks(cleaned)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot resolve path: %w", err))
	}

	resolvedBase, err := resolveSymlinks(dataDir)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("cannot resolve data directory: %w", err))
	}

	if allowRoot {
		if resolved != resolvedBase && !IsSubPath(resolvedBase, resolved) {
			return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
		}
	} else {
		if !IsSubPath(resolvedBase, resolved) {
			return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
		}
	}
	return resolved, nil
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

// FileType represents a known file type with its naming convention.
type FileType struct {
	Prefix string
	Suffix string
	Label  string
}

// Predefined file types used across the application.
var (
	NoteFileType     = FileType{Prefix: "note_", Suffix: ".md", Label: "note"}
	TaskListFileType = FileType{Prefix: "tasks_", Suffix: ".md", Label: "task list"}
)

// ValidateFileType checks that the file at absPath exists, is a regular file,
// and matches the expected naming convention.
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

// MatchesFileType returns true if name matches the prefix/suffix convention
// of the given FileType with at least one character between them.
func MatchesFileType(name string, ft FileType) bool {
	return strings.HasPrefix(name, ft.Prefix) &&
		filepath.Ext(name) == ft.Suffix &&
		len(name) > len(ft.Prefix)+len(ft.Suffix)
}

// ExtractTitle extracts the human-readable title from a filename by stripping prefix and suffix.
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
	MaxNoteContentBytes        = 1 << 20 // 1 MiB
	MaxNameLen                 = 255
	MaxTaskDescriptionBytes    = 1024
	MaxSubtaskDescriptionBytes = 1024
	MaxTasksPerList            = 1000
	MaxSubtasksPerTask         = 100
)

// ValidateContentLength returns an InvalidArgument error if len(data) exceeds max.
func ValidateContentLength(data string, max int, field string) error {
	if len(data) > max {
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%s too large: %d bytes exceeds %d byte limit", field, len(data), max))
	}
	return nil
}
