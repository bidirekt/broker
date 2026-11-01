package validations

import (
	"errors"
	"strings"
	"unicode"
)

func Endpoint(value string) error {
	if !strings.HasPrefix(value, "/") || strings.ContainsFunc(value, unicode.IsControl) {
		return errors.New("malformed path")
	}

	if value == "/" {
		return nil
	}

	for _, segment := range strings.Split(value[1:], "/") {
		if segment == "" || strings.Contains(segment, ";") {
			return errors.New("malformed path")
		}

		if segment == "*" {
			continue
		}

		if strings.ContainsAny(segment, "*{}") {
			return errors.New("dynamic path segments must use *")
		}
	}

	return nil
}
