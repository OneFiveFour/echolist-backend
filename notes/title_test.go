package notes

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: code-review-hardening, Property 2: Title extraction never panics
// **Validates: Requirements 2.1, 2.2, 2.3**
func TestProperty_ExtractNoteTitleNeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.String().Draw(rt, "filename")
		// Just call the function — if it panics, rapid will catch it.
		_, _ = ExtractNoteTitle(input)
	})
}

// Feature: code-review-hardening, Property 3: Title extraction round-trip
// **Validates: Requirements 11.4**
func TestProperty_TitleExtractionRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate valid titles: non-empty, no path separators, no null bytes, no ".md" suffix overlap
		title := rapid.StringMatching(`[a-zA-Z0-9 _-]{1,50}`).Draw(rt, "title")

		filename := "note_" + title + ".md"
		got, err := ExtractNoteTitle(filename)
		if err != nil {
			rt.Fatalf("ExtractNoteTitle(%q) returned error: %v", filename, err)
		}
		if got != title {
			rt.Fatalf("round-trip failed: input title %q, got %q", title, got)
		}
	})
}
