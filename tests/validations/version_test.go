package validations_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestVersion_AcceptsVersionsWithoutWhitespaceOrControlCharacters(t *testing.T) {
	for _, version := range []string{"1.0.0", "v1", "a1b2c3d", "1.0.0+build.5", "1.0.0-rc.1"} {
		t.Run(version, func(t *testing.T) {
			assert.NoError(t, validations.Version(version))
		})
	}
}

func TestVersion_RejectsAnEmptyVersion(t *testing.T) {
	assert.ErrorIs(t, validations.Version(""), validations.ErrEmptyVersion)
}

func TestVersion_RejectsWhitespaceOrAControlCharacterAnywhere(t *testing.T) {
	cases := map[string]struct{ version, message string }{
		"inner space":             {"1.0 beta", `version "1.0 beta" has whitespace or a control character`},
		"inner newline":           {"1.0\n2", `version "1.0\n2" has whitespace or a control character`},
		"trailing tab":            {"1.0\t", `version "1.0\t" has whitespace or a control character`},
		"leading space":           {" 1.0.0", `version " 1.0.0" has whitespace or a control character`},
		"only a space":            {" ", `version " " has whitespace or a control character`},
		"inner no-break space":    {"1.0\u00a0beta", "version \"1.0\\u00a0beta\" has whitespace or a control character"},
		"inner control character": {"1.0\x00", `version "1.0\x00" has whitespace or a control character`},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			assert.EqualError(t, validations.Version(testCase.version), testCase.message)
		})
	}
}
