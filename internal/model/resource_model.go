package model

import "github.com/guregu/null"

const (
	Consumes Direction = "consumes"
	Provides Direction = "provides"
)

const (
	Input  BodyRole = "input"
	Output BodyRole = "output"
)

type Direction string

func (this Direction) String() string {
	return string(this)
}

type Protocol string

func (this Protocol) String() string {
	return string(this)
}

// BodyRole says whose body a resource carries: Input is the body the consumer sends,
// Output the body the provider returns.
type BodyRole string

func (this BodyRole) String() string {
	return string(this)
}

type ResourceIdentity interface {
	HashParts() []string
	Describe() string
	Location() map[string]string
}

type Resource struct {
	ParticipantName  string
	Direction        Direction
	Protocol         Protocol
	Body             BodyRole
	Identity         ResourceIdentity
	ConsumedProvider null.String
	Bodies           []ResourceBody
}

func (this *Resource) IsConsumer() bool {
	return this.Direction == Consumes
}

func (this *Resource) IsProvider() bool {
	return this.Direction == Provides
}
