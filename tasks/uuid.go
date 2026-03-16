package tasks

import (
	"fmt"
	"regexp"

	"connectrpc.com/connect"
)

var uuidV4Re = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

// validateUuidV4 returns a connect InvalidArgument error if id is not a valid
// lowercase hyphenated UUIDv4 string.
func validateUuidV4(id string) error {
	if !uuidV4Re.MatchString(id) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid UUIDv4: %q", id))
	}
	return nil
}
