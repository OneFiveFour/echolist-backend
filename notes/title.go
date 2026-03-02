package notes

import (
	"echolist-backend/pathutil"
)

// ExtractNoteTitle extracts the title from a note filename.
// Returns an error if the filename is too short or doesn't match the expected pattern.
func ExtractNoteTitle(filename string) (string, error) {
	return pathutil.ExtractTitle(filename, pathutil.NoteFileType.Prefix, pathutil.NoteFileType.Suffix, pathutil.NoteFileType.Label)
}
