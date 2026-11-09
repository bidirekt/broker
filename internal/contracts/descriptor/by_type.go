package descriptor

import (
	"github.com/bidirekt/broker/internal/contracts/violation"
)

// ByType is the node of an Object field that depends on its sibling "type". Under a type it does not
// list, the field is left unvalidated: keyBelongsTo decides whether it may be there at all.
type ByType map[string]Node

func (this ByType) validate(any, string) []violation.Violation {
	return nil
}

func (this ByType) chosenBy(siblings map[string]any) Node {
	declaredType, _ := siblings["type"].(string)
	if chosen, listed := this[declaredType]; listed {
		return chosen
	}

	return this
}
