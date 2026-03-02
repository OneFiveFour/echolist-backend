package tasks

import (
	"io"
	"log/slog"
)

// nopLogger returns a logger that discards all output, for use in tests.
func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
