package pages_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/stretchr/testify/assert"
)

func TestParsePathSegmentUnescapesTheSegment(t *testing.T) {
	for raw, expected := range map[string]string{
		"1.4.0":         "1.4.0",
		"release%2F1.0": "release/1.0",
		"v%201":         "v 1",
		"%C3%A9t%C3%A9": "été",
		"100%25":        "100%",
		"a+b":           "a+b",
	} {
		value, ok := pages.ParsePathSegment(raw)

		assert.True(t, ok, "raw %q", raw)
		assert.Equal(t, expected, value, "raw %q", raw)
	}
}

func TestParsePathSegmentRejectsABadEscapeInvalidUTF8OrNUL(t *testing.T) {
	for _, raw := range []string{"%00", "a%00b", "%FF", "%C3", "%", "%G1", "100%"} {
		_, ok := pages.ParsePathSegment(raw)

		assert.False(t, ok, "raw %q", raw)
	}
}
