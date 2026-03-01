package notes

import (
	"fmt"
	"strings"
)

// ExtractNoteTitle extracts the title from a note filename.
// Returns an error if the filename is too short or doesn't match the expected pattern.
func ExtractNoteTitle(filename string) (string, error) {
	const prefix = "note_"
	const suffix = ".md"
	if len(filename) < len(prefix)+len(suffix)+1 {
		return "", fmt.Errorf("filename too short to extract title: %q", filename)
	}
	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
		return "", fmt.Errorf("filename does not match note pattern: %q", filename)
	}
	return filename[len(prefix) : len(filename)-len(suffix)], nil
}
