package validations

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxEmailCharacters = 254

func Email(value string) error {
	local, domain, found := strings.Cut(value, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") {
		return errors.New("must have exactly one @ with text on both sides")
	}

	if utf8.RuneCountInString(value) > maxEmailCharacters {
		return errors.New("must have at most 254 characters")
	}

	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return errors.New("must have no space or control character")
		}
	}

	return nil
}
