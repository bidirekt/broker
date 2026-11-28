package validations_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestVersion_AcceptsVersionsWithoutWhitespaceAtEitherEnd(t *testing.T) {
	for _, version := range []string{"1.0.0", "v1", "a1b2c3d", "1.0.0+build.5"} {
		t.Run(version, func(t *testing.T) {
			assert.NoError(t, validations.Version(version))
		})
	}
}

func TestVersion_RejectsEmptyOrPaddedVersions(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"leading space":    " 1.0.0",
		"trailing tab":     "1.0.0\t",
		"trailing newline": "1.0.0\n",
	}

	for name, version := range cases {
		t.Run(name, func(t *testing.T) {
			assert.EqualError(t, validations.Version(version), "must not be empty or have whitespace at either end")
		})
	}
}
