package pages_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/stretchr/testify/assert"
)

func TestParsePositiveIntegerAcceptsACanonicalPositiveInteger(t *testing.T) {
	for raw, expected := range map[string]int64{"1": 1, "42": 42, "9223372036854775807": 9223372036854775807} {
		value, ok := pages.ParsePositiveInteger(raw)

		assert.True(t, ok, "raw %q", raw)
		assert.Equal(t, expected, value, "raw %q", raw)
	}
}

func TestParsePositiveIntegerRejectsAnythingElse(t *testing.T) {
	for _, raw := range []string{"", "0", "00", "01", "+1", "-1", " 1", "1 ", "1.0", "1e3", "abc", "0x10", "9223372036854775808"} {
		_, ok := pages.ParsePositiveInteger(raw)

		assert.False(t, ok, "raw %q", raw)
	}
}
