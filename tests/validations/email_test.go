package validations_test

import (
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestEmail_AcceptsOneAtWithTextOnBothSides(t *testing.T) {
	for _, value := range []string{"ana@x.com", "a@b", "Ana.Silva+broker@Example.COM", "josé@exemplo.br"} {
		t.Run(value, func(t *testing.T) {
			assert.NoError(t, validations.Email(value))
		})
	}
}

func TestEmail_RejectsAnythingButExactlyOneAtWithTextOnBothSides(t *testing.T) {
	for _, value := range []string{"", "ana.x.com", "@x.com", "ana@", "@", "ana@@x.com", "ana@x@com"} {
		t.Run(value, func(t *testing.T) {
			assert.EqualError(t, validations.Email(value), "must have exactly one @ with text on both sides")
		})
	}
}

func TestEmail_AcceptsUpToTwoHundredFiftyFourCharacters(t *testing.T) {
	assert.NoError(t, validations.Email(strings.Repeat("a", 248)+"@x.com"))
	assert.NoError(t, validations.Email(strings.Repeat("é", 248)+"@x.com"))
}

func TestEmail_RejectsMoreThanTwoHundredFiftyFourCharacters(t *testing.T) {
	assert.EqualError(t, validations.Email(strings.Repeat("a", 249)+"@x.com"), "must have at most 254 characters")
}

func TestEmail_RejectsSpacesAndControlCharacters(t *testing.T) {
	for _, value := range []string{"ana @x.com", " ana@x.com", "ana@x.com ", "ana@x.com\t", "ana\n@x.com", "ana@x .com", "ana\x00@x.com", "ana\x7f@x.com"} {
		t.Run(value, func(t *testing.T) {
			assert.EqualError(t, validations.Email(value), "must have no space or control character")
		})
	}
}
