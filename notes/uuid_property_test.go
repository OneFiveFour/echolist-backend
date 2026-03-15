package notes

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"pgregory.net/rapid"
)

// invalidUuidGen generates strings that are NOT valid UUIDv4.
// Strategy: sample from several categories of invalid inputs —
// empty strings, random alphanumeric, uppercase UUIDs, wrong version digit,
// wrong variant nibble, missing hyphens, and wrong length.
func invalidUuidGen() *rapid.Generator[string] {
	return rapid.OneOf(
		// Empty string
		rapid.Just(""),
		// Random short string (not UUID-shaped at all)
		rapid.StringMatching(`[a-zA-Z0-9]{1,30}`),
		// Correct shape but uppercase hex (UUIDv4 must be lowercase)
		rapid.Custom(func(rt *rapid.T) string {
			a := rapid.StringMatching(`[0-9A-F]{8}`).Draw(rt, "a")
			b := rapid.StringMatching(`[0-9A-F]{4}`).Draw(rt, "b")
			c := rapid.StringMatching(`[0-9A-F]{3}`).Draw(rt, "c")
			d := rapid.StringMatching(`[0-9A-F]{3}`).Draw(rt, "d")
			e := rapid.StringMatching(`[0-9A-F]{12}`).Draw(rt, "e")
			return a + "-" + b + "-4" + c + "-a" + d + "-" + e
		}),
		// Correct format but wrong version digit (not 4)
		rapid.Custom(func(rt *rapid.T) string {
			a := rapid.StringMatching(`[0-9a-f]{8}`).Draw(rt, "a")
			b := rapid.StringMatching(`[0-9a-f]{4}`).Draw(rt, "b")
			ver := rapid.SampledFrom([]string{"0", "1", "2", "3", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}).Draw(rt, "ver")
			c := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "c")
			variant := rapid.SampledFrom([]string{"8", "9", "a", "b"}).Draw(rt, "variant")
			d := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "d")
			e := rapid.StringMatching(`[0-9a-f]{12}`).Draw(rt, "e")
			return a + "-" + b + "-" + ver + c + "-" + variant + d + "-" + e
		}),
		// Correct format but wrong variant nibble (not 8/9/a/b)
		rapid.Custom(func(rt *rapid.T) string {
			a := rapid.StringMatching(`[0-9a-f]{8}`).Draw(rt, "a")
			b := rapid.StringMatching(`[0-9a-f]{4}`).Draw(rt, "b")
			c := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "c")
			variant := rapid.SampledFrom([]string{"0", "1", "2", "3", "4", "5", "6", "7", "c", "d", "e", "f"}).Draw(rt, "variant")
			d := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "d")
			e := rapid.StringMatching(`[0-9a-f]{12}`).Draw(rt, "e")
			return a + "-" + b + "-4" + c + "-" + variant + d + "-" + e
		}),
		// UUID-like but with hyphens removed
		rapid.StringMatching(`[0-9a-f]{32}`),
	)
}

// Feature: note-stable-ids, Property 8: Invalid UUID returns InvalidArgument
// For any string that is not a valid UUIDv4, validateUuidV4 returns a Connect
// InvalidArgument error.
// **Validates: Requirements 9.1**
func TestProperty8_InvalidUuidReturnsInvalidArgument(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := invalidUuidGen().Draw(rt, "invalidUuid")

		err := validateUuidV4(input)

		if err == nil {
			rt.Fatalf("validateUuidV4(%q): expected error, got nil", input)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("validateUuidV4(%q): expected connect.Error, got %T: %v", input, err, err)
		}
		if connErr.Code() != connect.CodeInvalidArgument {
			rt.Fatalf("validateUuidV4(%q): expected CodeInvalidArgument, got %v", input, connErr.Code())
		}
	})
}
