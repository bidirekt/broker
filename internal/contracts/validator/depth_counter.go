package validator

import "github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"

// DepthCounter is the depth budget one schema branch carries.
type DepthCounter struct {
	levels int
}

func (this DepthCounter) Deeper() DepthCounter {
	return DepthCounter{levels: this.levels + 1}
}

func (this DepthCounter) Exceeded() bool {
	return this.levels >= schemamapper.MaxDepth
}
