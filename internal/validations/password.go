package validations

import (
	"errors"
	"unicode/utf8"
)

const (
	minPasswordCharacters = 15
	maxPasswordCharacters = 128
)

func Password(value string) error {
	characters := utf8.RuneCountInString(value)
	if characters < minPasswordCharacters || characters > maxPasswordCharacters {
		return errors.New("must have 15 to 128 characters")
	}

	return nil
}
