package inventory

import "errors"

// ErrNotFound is returned when a named target does not exist in the
// inventory.
var ErrNotFound = errors.New("inventory: target not found")
