package descriptor

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

type Key struct {
	ErrorCode string
	Check     func(key string) error
	Suggest   func(key string) string
	Details   func(key string) map[string]any
}

func (this Key) reject(key string, path string) *violation.Violation {
	if this.Check == nil {
		return nil
	}

	err := this.Check(key)
	if err == nil {
		return nil
	}

	details := map[string]any{"key": key, "error": err.Error()}
	if this.Details != nil {
		details = this.Details(key)
	}

	if this.Suggest != nil {
		if suggestion := this.Suggest(key); suggestion != "" {
			details["suggestion"] = suggestion
		}
	}

	return &violation.Violation{ErrorCode: this.ErrorCode, Path: path, Details: details}
}

func Map(key Key, value Node) Node {
	return mapNode{key: key, value: value}
}

type mapNode struct {
	key   Key
	value Node
}

func (this mapNode) validate(value any, path string) []violation.Violation {
	return validateMapping(value, path, this.key, func(_ string, entry any, entryPath string) []violation.Violation {
		return this.value.validate(entry, entryPath)
	})
}

func validateMapping(
	value any,
	path string,
	key Key,
	validateEntry func(key string, entry any, entryPath string) []violation.Violation,
) []violation.Violation {
	if value == nil {
		return nil
	}

	mapping, isMapping := value.(map[string]any)
	if !isMapping {
		return []violation.Violation{invalidKind(value, path, "mapping")}
	}

	var violations []violation.Violation

	for _, name := range slices.Sorted(maps.Keys(mapping)) {
		entryPath := joinPath(path, name)
		if rejected := key.reject(name, entryPath); rejected != nil {
			violations = append(violations, *rejected)

			continue
		}

		violations = append(violations, validateEntry(name, mapping[name], entryPath)...)
	}

	return violations
}
