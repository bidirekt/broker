package rest

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	literalSegment     = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
	placeholderSegment = regexp.MustCompile(`^(?:\{.+\}|:.+|<.+>|\[.+\])$`)
	errDynamicSegment  = errors.New("dynamic path segments must use *")
)

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
			return errDynamicSegment
		}

		if !literalSegment.MatchString(segment) {
			return errors.New("malformed path")
		}
	}

	return nil
}

func SuggestEndpoint(endpoint string) string {
	normalized := NormalizeEndpoint(endpoint)
	if !errors.Is(CheckEndpoint(normalized), errDynamicSegment) {
		return ""
	}

	segments := strings.Split(normalized, "/")
	for index, segment := range segments {
		if placeholderSegment.MatchString(segment) {
			segments[index] = "*"
		}
	}

	suggestion := strings.Join(segments, "/")
	if CheckEndpoint(suggestion) != nil {
		return ""
	}

	return suggestion
}
