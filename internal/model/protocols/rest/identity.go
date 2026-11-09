package rest

import "strings"

// Identity is a REST resource: Endpoint normalized, Method lower case, Status empty for
// the request.
type Identity struct {
	Endpoint string
	Method   string
	Status   string
}

func (this Identity) HashParts() []string {
	parts := []string{this.Endpoint, this.Method}
	if this.Status != "" {
		parts = append(parts, this.Status)
	}

	return parts
}

func (this Identity) Interaction() string {
	if this.Status == "" {
		return RequestInteraction
	}

	return this.Status
}

func (this Identity) Describe() string {
	return strings.ToUpper(this.Method) + " " + this.Endpoint + " " + this.Interaction()
}

func (this Identity) Location() map[string]string {
	return map[string]string{
		EndpointKey:    this.Endpoint,
		MethodKey:      this.Method,
		InteractionKey: this.Interaction(),
	}
}
