package model

import "github.com/guregu/null"

const (
	Consumes     Direction   = "consumes"
	Provides     Direction   = "provides"
	RestRequest  Interaction = "rest_request"
	RestResponse Interaction = "rest_response"
)

type Direction string

func (this Direction) String() string {
	return string(this)
}

type Interaction string

func (this Interaction) String() string {
	return string(this)
}

type Resource struct {
	ParticipantName    string
	Direction          Direction
	Interaction        Interaction
	ConsumedProvider   null.String
	Endpoint           string
	Method             string
	ResponseStatusCode null.String
	Bodies             []ResourceBody
}

func (this *Resource) IsConsumer() bool {
	return this.Direction == Consumes
}

func (this *Resource) IsProvider() bool {
	return this.Direction == Provides
}
