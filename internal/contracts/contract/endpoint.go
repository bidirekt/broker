package contract

import (
	"strings"
)

func NormalizeEndpoint(endpoint string) string {
	path, hasTrailingSlash := strings.CutSuffix(endpoint, "/")
	closesSegment := path != "" && !strings.HasSuffix(path, "/")

	if hasTrailingSlash && closesSegment {
		return path
	}

	return endpoint
}
