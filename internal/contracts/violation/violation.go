package violation

import (
	"cmp"
	"slices"
	"strings"
)

type Violation struct {
	ErrorCode string            `json:"code"`
	Path      string            `json:"path"`
	Source    string            `json:"source"`
	Details   map[string]string `json:"details"`
}

func SortedByLocation(violations []Violation) []Violation {
	sorted := slices.Clone(violations)

	slices.SortStableFunc(sorted, func(a, b Violation) int {
		return cmp.Or(strings.Compare(a.Source, b.Source), strings.Compare(a.Path, b.Path))
	})

	return sorted
}
