package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func Validate(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, rule := range rules {
		violations = append(violations, rule(declarations)...)
	}

	return violation.SortedByLocation(violations)
}
