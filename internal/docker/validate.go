package docker

import (
	"fmt"
	"regexp"

	"infrastructure-mcp/internal/toolerr"
)

// containerRefPattern matches Docker's own container/image name rules
// (alphanumeric, then alphanumeric/underscore/period/hyphen) as well as
// hex container/image IDs (a subset of the same pattern). Anchored and
// checked before any container reference is embedded into a remote
// command string, per README's "no shell interpolation" rule.
var containerRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// validateContainerRef rejects anything that isn't a plausible Docker
// container name or ID, so a malicious or malformed value can never reach
// the remote shell command.
func validateContainerRef(ref string) error {
	if !containerRefPattern.MatchString(ref) {
		return fmt.Errorf("docker: invalid container reference %q: %w", ref, toolerr.ErrInvalidInput)
	}
	return nil
}
