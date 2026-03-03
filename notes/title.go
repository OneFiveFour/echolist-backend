package notes

import (
	"echolist-backend/common"
)

// ExtractNoteTitle extracts the title from a note filename.
func ExtractNoteTitle(filename string) (string, error) {
	return common.ExtractTitle(filename, common.NoteFileType.Prefix, common.NoteFileType.Suffix, common.NoteFileType.Label)
}
