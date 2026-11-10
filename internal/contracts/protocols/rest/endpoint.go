package rest

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var literalSegment = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

func NormalizeEndpoint(endpoint string) string {
	path, hasTrailingSlash := strings.CutSuffix(endpoint, "/")
	closesSegment := path != "" && !strings.HasSuffix(path, "/")

	if hasTrailingSlash && closesSegment {
		return path
	}

	return endpoint
}

func CheckEndpoint(value string) error {
	if !strings.HasPrefix(value, "/") || strings.ContainsFunc(value, unicode.IsControl) {
		return errors.New("malformed path")
	}

	if value == "/" {
		return nil
	}

	for _, segment := range strings.Split(value[1:], "/") {
		if segment == "*" {
			continue
		}

		if strings.ContainsAny(segment, "*:{}<>[]") {
			return errors.New("dynamic path segments must use *")
		}

		if !literalSegment.MatchString(segment) {
			return errors.New("malformed path")
		}
	}

	return nil
}
