package descriptor

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

type Key struct {
	ErrorCode string
	Check     func(key string) error
}

func (this Key) reject(key string, path string) *violation.Violation {
	if this.Check == nil {
		return nil
	}

	err := this.Check(key)
	if err == nil {
		return nil
	}

	return &violation.Violation{
		ErrorCode: this.ErrorCode,
		Path:      path,
		Details: map[string]string{
			"key":   key,
			"error": err.Error(),
		},
	}
}

func Map(key Key, value Node) Node {
	return mapNode{key: key, value: value}
}

type mapNode struct {
	key   Key
	value Node
}

func (this mapNode) validate(value any, path string) []violation.Violation {
	if value == nil {
		return nil
	}

	mapping, exists := value.(map[string]any)
	if !exists {
		return []violation.Violation{invalidKind(value, path, "mapping")}
	}

	var violations []violation.Violation

	for _, key := range slices.Sorted(maps.Keys(mapping)) {
		if rejected := this.key.reject(key, joinPath(path, key)); rejected != nil {
			violations = append(violations, *rejected)

			continue
		}

		violations = append(violations, this.value.validate(mapping[key], joinPath(path, key))...)
	}

	return violations
}
