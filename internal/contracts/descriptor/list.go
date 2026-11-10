package descriptor

import (
	"strconv"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

func List(item Node) Node {
	return listNode{item: item}
}

type listNode struct {
	item Node
}

func (this listNode) validate(value any, path string) []violation.Violation {
	if value == nil {
		return nil
	}

	sequence, isSequence := value.([]any)
	if !isSequence {
		return []violation.Violation{invalidKind(value, path, "sequence")}
	}

	var violations []violation.Violation

	for index, item := range sequence {
		violations = append(violations, this.item.validate(item, joinPath(path, strconv.Itoa(index)))...)
	}

	return violations
}
