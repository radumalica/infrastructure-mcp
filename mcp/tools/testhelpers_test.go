package tools

import (
	"io"
	"log/slog"
)

// testLogger discards output; logging.go's behavior is exercised
// separately in logging_test.go with a capturing handler.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
