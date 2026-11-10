package validator

import (
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func Validate(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, rule := range rules {
		violations = append(violations, rule(declarations)...)
	}

	return violation.SortedByLocation(violations)
}
