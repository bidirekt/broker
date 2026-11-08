package pages

import "strings"

func IsPagePath(path string) bool {
	return !IsAPIPath(path) && !IsUnderPath(path, "/health") && !IsUnderPath(path, "/assets")
}

func IsAPIPath(path string) bool {
	return IsUnderPath(path, "/api")
}

func IsUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
