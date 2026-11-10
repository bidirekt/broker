package protocol

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

type Registry struct {
	protocols []Protocol
	grammar   descriptor.Node
}

func NewRegistry(protocols []Protocol) *Registry {
	sorted := slices.SortedFunc(slices.Values(protocols), func(a, b Protocol) int {
		return cmp.Compare(a.Protocol(), b.Protocol())
	})

	blocks := make(descriptor.Fields, len(sorted))
	for _, protocol := range sorted {
		name := protocol.Protocol().String()
		if _, registered := blocks[name]; registered {
			panic(fmt.Errorf("protocol %q is registered twice", name))
		}

		blocks[name] = protocol.Grammar()
	}

	return &Registry{protocols: sorted, grammar: descriptor.Contract(blocks)}
}

func (this *Registry) Protocols() []Protocol {
	return this.protocols
}

func (this *Registry) Grammar() descriptor.Node {
	return this.grammar
}
