package validations_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestPassword_AcceptsFifteenToOneHundredTwentyEightCharacters(t *testing.T) {
	for _, length := range []int{15, 16, 127, 128} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			assert.NoError(t, validations.Password(strings.Repeat("a", length)))
		})
	}
}

func TestPassword_RejectsFewerThanFifteenOrMoreThanOneHundredTwentyEightCharacters(t *testing.T) {
	for _, length := range []int{0, 14, 129} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			assert.EqualError(t, validations.Password(strings.Repeat("a", length)), "must have 15 to 128 characters")
		})
	}
}

func TestPassword_CountsMultiByteCharactersAsOne(t *testing.T) {
	assert.NoError(t, validations.Password(strings.Repeat("é", 15)))
	assert.NoError(t, validations.Password(strings.Repeat("🔑", 128)))
	assert.EqualError(t, validations.Password(strings.Repeat("é", 14)), "must have 15 to 128 characters")
	assert.EqualError(t, validations.Password(strings.Repeat("🔑", 129)), "must have 15 to 128 characters")
}

func TestPassword_HasNoCompositionRules(t *testing.T) {
	assert.NoError(t, validations.Password("correct horse battery staple"))
}
