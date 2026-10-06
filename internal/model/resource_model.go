package model

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
