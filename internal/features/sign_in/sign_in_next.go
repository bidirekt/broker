package sign_in

import (
	"net/url"
	"strings"
	"unicode"
)

func safeNextPathFrom(next string) string {
	if !strings.HasPrefix(next, "/") || strings.Contains(next, "//") || strings.Contains(next, `\`) || strings.ContainsFunc(next, unicode.IsControl) {
		return homePath
	}

	parsed, err := url.Parse(next)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" {
		return homePath
	}

	return next
}
