package contract

import "slices"

type RefTrail struct {
	names []string
}

func (this RefTrail) Enter(name string) RefTrail {
	return RefTrail{names: append(slices.Clone(this.names), name)}
}

func (this RefTrail) Contains(name string) bool {
	return slices.Contains(this.names, name)
}

func (this RefTrail) Names() []string {
	return slices.Clone(this.names)
}
