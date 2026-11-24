package validator

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func Validate(declarations protocol.Declarations, protocolRules ...protocol.Rule) []violation.Violation {
	var violations []violation.Violation

	for _, rule := range slices.Concat(rules, protocolRules) {
		violations = append(violations, rule(declarations)...)
	}

	return violation.SortedByLocation(violations)
}
