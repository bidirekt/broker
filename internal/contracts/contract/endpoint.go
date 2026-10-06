package contract

import (
	"strings"
)

func NormalizeEndpoint(endpoint string) string {
	if len(endpoint) > 1 && strings.HasSuffix(endpoint, "/") {
		return strings.TrimSuffix(endpoint, "/")
	}

	return endpoint
}
