package pages

import "strings"

func IsPagePath(path string) bool {
	return !isUnderPath(path, "/api") && !isUnderPath(path, "/health") && !isUnderPath(path, "/assets")
}

func isUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
