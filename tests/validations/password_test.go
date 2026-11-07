package validations_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestPassword_AcceptsEightToOneHundredTwentyEightCharacters(t *testing.T) {
	for _, length := range []int{8, 9, 127, 128} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			assert.NoError(t, validations.Password(strings.Repeat("a", length)))
		})
	}
}

func TestPassword_RejectsFewerThanEightOrMoreThanOneHundredTwentyEightCharacters(t *testing.T) {
	for _, length := range []int{0, 7, 129} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			assert.EqualError(t, validations.Password(strings.Repeat("a", length)), "must have 8 to 128 characters")
		})
	}
}

func TestPassword_CountsMultiByteCharactersAsOne(t *testing.T) {
	assert.NoError(t, validations.Password(strings.Repeat("é", 8)))
	assert.NoError(t, validations.Password(strings.Repeat("🔑", 128)))
	assert.EqualError(t, validations.Password(strings.Repeat("é", 7)), "must have 8 to 128 characters")
	assert.EqualError(t, validations.Password(strings.Repeat("🔑", 129)), "must have 8 to 128 characters")
}

func TestPassword_HasNoCompositionRules(t *testing.T) {
	assert.NoError(t, validations.Password("correct horse battery staple"))
}
