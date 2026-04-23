package database

// NotePath computes the relative file path for a note from its metadata.
// Returns "<title>_<id>.md" when parentDir is "" (root), or
// "<parentDir>/<title>_<id>.md" otherwise.
func NotePath(parentDir, title, id string) string {
	filename := title + "_" + id + ".md"
	if parentDir == "" {
		return filename
	}
	return parentDir + "/" + filename
}
