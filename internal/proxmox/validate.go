package proxmox

import (
	"errors"
	"fmt"
)

// ErrInvalidGuestType indicates a guest type other than "qemu" or "lxc"
// was supplied.
var ErrInvalidGuestType = errors.New("proxmox: invalid guest type")

// validateGuestType rejects anything other than Proxmox's two guest kinds,
// so a malformed value can never be embedded into the request path.
func validateGuestType(guestType string) error {
	if guestType != "qemu" && guestType != "lxc" {
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidGuestType, guestType, "qemu", "lxc")
	}
	return nil
}
