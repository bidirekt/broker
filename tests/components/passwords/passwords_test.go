package passwords_test

import (
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const passwordUnderTest = "correct horse battery staple"

func TestHashPasswordThenVerifyPasswordAcceptsTheSamePassword(t *testing.T) {
	hash, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)

	assert.True(t, passwords.Verify(passwordUnderTest, hash))
}

func TestVerifyPasswordRejectsAWrongPassword(t *testing.T) {
	hash, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)

	assert.False(t, passwords.Verify("correct horse battery stapler", hash))
}

func TestHashPasswordSaltsEveryHash(t *testing.T) {
	first, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)
	second, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestHashPasswordUsesTheArgon2idParameters(t *testing.T) {
	hash, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$"), hash)
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	hash, err := passwords.Hash(passwordUnderTest)
	require.NoError(t, err)
	parts := strings.Split(hash, "$")
	salt, key := parts[4], parts[5]

	malformed := map[string]string{
		"empty":             "",
		"garbage":           "not-a-hash",
		"missing key":       "$argon2id$v=19$m=19456,t=2,p=1$" + salt,
		"extra segment":     hash + "$extra",
		"other algorithm":   "$argon2i$v=19$m=19456,t=2,p=1$" + salt + "$" + key,
		"other version":     "$argon2id$v=16$m=19456,t=2,p=1$" + salt + "$" + key,
		"unparsable params": "$argon2id$v=19$m=abc,t=2,p=1$" + salt + "$" + key,
		"trailing params":   "$argon2id$v=19$m=19456,t=2,p=1,x=1$" + salt + "$" + key,
		"zero time":         "$argon2id$v=19$m=19456,t=0,p=1$" + salt + "$" + key,
		"zero threads":      "$argon2id$v=19$m=19456,t=2,p=0$" + salt + "$" + key,
		"threads overflow":  "$argon2id$v=19$m=19456,t=2,p=256$" + salt + "$" + key,
		"invalid salt":      "$argon2id$v=19$m=19456,t=2,p=1$!!!$" + key,
		"empty salt":        "$argon2id$v=19$m=19456,t=2,p=1$$" + key,
		"invalid key":       "$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$!!!",
		"empty key":         "$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$",
		"padded key":        "$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$" + key + "=",
	}

	for name, encodedHash := range malformed {
		t.Run(name, func(t *testing.T) {
			assert.False(t, passwords.Verify(passwordUnderTest, encodedHash))
		})
	}
}

func TestDummyPasswordHashIsAValidHashThatRejectsPasswords(t *testing.T) {
	assert.True(t, strings.HasPrefix(passwords.DummyHash, "$argon2id$v=19$m=19456,t=2,p=1$"))
	assert.NotPanics(t, func() {
		assert.False(t, passwords.Verify(passwordUnderTest, passwords.DummyHash))
	})
}
