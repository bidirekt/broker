package descriptor

import (
	"sync"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

// Go rejects a var that refers to itself in its own initializer, even through a closure;
// a function may refer to itself, so a recursive grammar is a function resolved once, at the first validation.
func Lazy(grammar func() Node) Node {
	return lazyNode{grammar: sync.OnceValue(grammar)}
}

type lazyNode struct {
	grammar func() Node
}

func (this lazyNode) validate(value any, path string) []violation.Violation {
	return this.grammar().validate(value, path)
}
