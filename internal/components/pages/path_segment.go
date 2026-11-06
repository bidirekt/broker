package pages

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

func ParsePathSegment(raw string) (string, bool) {
	value, err := url.PathUnescape(raw)

	return value, err == nil && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
