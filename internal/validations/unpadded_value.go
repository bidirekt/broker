package validations

import (
	"errors"
	"strings"
)

func UnpaddedValue(value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return errors.New("must not be empty or have whitespace at either end")
	}

	return nil
}
