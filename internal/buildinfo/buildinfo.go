package buildinfo

import "strings"

var Mode = "development"

func IsRelease() bool {
	normalized := strings.ToLower(strings.TrimSpace(Mode))
	return normalized == "production" || normalized == "release"
}
