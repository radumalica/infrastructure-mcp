package tools

// severityForPercent classifies a 0-100 usage percentage per the
// thresholds in README.md's tool-design example (warning/critical
// disk/memory usage).
func severityForPercent(pct float64, warnAt, critAt float64, resourceName string) (status, recommendation string) {
	switch {
	case pct >= critAt:
		return "critical", "Free " + resourceName + " immediately or expand capacity."
	case pct >= warnAt:
		return "warning", "Free " + resourceName + " or plan a capacity increase."
	default:
		return "ok", ""
	}
}
