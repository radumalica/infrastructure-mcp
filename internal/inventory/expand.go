package inventory

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv replaces every ${VAR} reference in raw with the value of the
// matching environment variable. Secrets therefore never live in the
// inventory file itself. It fails closed: any referenced variable that is
// unset (as opposed to set-but-empty) is reported as an error rather than
// silently substituted with an empty string.
func expandEnv(raw []byte) ([]byte, error) {
	missing := map[string]struct{}{}

	expanded := envVarPattern.ReplaceAllStringFunc(string(raw), func(match string) string {
		name := envVarPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(name)
		if !ok {
			missing[name] = struct{}{}
			return match
		}
		return val
	})

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("inventory: missing environment variables: %s", strings.Join(names, ", "))
	}

	return []byte(expanded), nil
}
