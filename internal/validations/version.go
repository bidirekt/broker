package validations

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrEmptyVersion = errors.New("version must not be empty")

func Version(value string) error {
	if value == "" {
		return ErrEmptyVersion
	}

	if strings.ContainsFunc(value, func(character rune) bool { return unicode.IsSpace(character) || unicode.IsControl(character) }) {
		return fmt.Errorf("version %q has whitespace or a control character", value)
	}

	return nil
}
