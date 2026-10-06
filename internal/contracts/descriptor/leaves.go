package descriptor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

func String() Node {
	return stringNode{}
}

type stringNode struct{}

func (stringNode) validate(value any, path string) []violation.Violation {
	if _, ok := value.(string); ok || value == nil {
		return nil
	}

	return []violation.Violation{invalidKind(value, path, "string")}
}

func Bool() Node {
	return boolNode{}
}

type boolNode struct{}

func (boolNode) validate(value any, path string) []violation.Violation {
	if _, ok := value.(bool); ok || value == nil {
		return nil
	}

	return []violation.Violation{invalidKind(value, path, "boolean")}
}

type Enum struct {
	ErrorCode string
	Allowed   []string
}

func (this Enum) validate(value any, path string) []violation.Violation {
	if value == nil {
		return nil
	}

	if !isScalar(value) {
		return []violation.Violation{invalidKind(value, path, "string")}
	}

	text := fmt.Sprint(value)
	if slices.Contains(this.Allowed, text) {
		return nil
	}

	return []violation.Violation{{
		ErrorCode: this.ErrorCode,
		Path:      path,
		Details: map[string]string{
			"value":   text,
			"allowed": strings.Join(this.Allowed, ", "),
		},
	}}
}

func isScalar(value any) bool {
	switch value.(type) {
	case string, bool, int, int64, uint64, float64:
		return true
	default:
		return false
	}
}
