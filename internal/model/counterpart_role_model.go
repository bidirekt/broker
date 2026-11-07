package model

type CounterpartRole string

const (
	CounterpartProvider CounterpartRole = "provider"
	CounterpartConsumer CounterpartRole = "consumer"
	CounterpartBoth     CounterpartRole = "both"
)

func (this CounterpartRole) MergedWith(other CounterpartRole) CounterpartRole {
	if this == "" || this == other {
		return other
	}

	if other == "" {
		return this
	}

	return CounterpartBoth
}
