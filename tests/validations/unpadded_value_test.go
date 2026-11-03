package validations_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestUnpaddedValue_AcceptsValuesWithoutWhitespaceAtEitherEnd(t *testing.T) {
	for _, value := range []string{"orders", "1.0.0", "a1b2c3d", "eu west"} {
		t.Run(value, func(t *testing.T) {
			assert.NoError(t, validations.UnpaddedValue(value))
		})
	}
}

func TestUnpaddedValue_RejectsEmptyOrPaddedValues(t *testing.T) {
	cases := map[string]string{
		"empty":                   "",
		"only a space":            " ",
		"leading space":           " orders",
		"trailing space":          "orders ",
		"leading tab":             "\torders",
		"trailing newline":        "orders\n",
		"trailing no-break space": "orders ",
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			assert.EqualError(t, validations.UnpaddedValue(value), "must not be empty or have whitespace at either end")
		})
	}
}
