package model

import (
	"strings"

	"github.com/guregu/null"
)

type UploadedResource struct {
	ParticipantName    string
	Direction          Direction
	Interaction        Interaction
	ConsumedProvider   null.String
	Endpoint           string
	Method             string
	ResponseStatusCode null.String
	Properties         map[string]Property
}

func (this *UploadedResource) IsConsumer() bool {
	return this.Direction == Consumes
}

func (this *UploadedResource) IsProvider() bool {
	return this.Direction == Provides
}

// Describe names the resource the way publish errors quote it: the same parts that
// make up its hash, in reading order.
func (this *UploadedResource) Describe() string {
	parts := []string{this.Direction.String()}

	if this.IsConsumer() && this.ConsumedProvider.String != "" {
		parts = append(parts, this.ConsumedProvider.String)
	}

	parts = append(parts, strings.ToUpper(this.Method), this.Endpoint)

	if this.Interaction == RestResponse {
		parts = append(parts, this.ResponseStatusCode.String)
	} else {
		parts = append(parts, "request")
	}

	return strings.Join(parts, " ")
}

func (this *UploadedResource) ProviderHash() string {
	providerName := this.ParticipantName
	if this.IsConsumer() {
		providerName = this.ConsumedProvider.String
	}

	parts := []string{providerName, this.Endpoint, this.Method}
	if this.Interaction == RestResponse {
		parts = append(parts, this.ResponseStatusCode.String)
	}

	return Hash(parts...)
}

func (this *UploadedResource) ConsumerHash() string {
	if !this.IsConsumer() {
		return ""
	}

	parts := []string{this.ParticipantName, this.ConsumedProvider.String, this.Endpoint, this.Method}
	if this.Interaction == RestResponse {
		parts = append(parts, this.ResponseStatusCode.String)
	}

	return Hash(parts...)
}

func (this *UploadedResource) PrimaryHash() string {
	if this.IsProvider() {
		return this.ProviderHash()
	}

	return this.ConsumerHash()
}

func NewRestRequestConsumer(
	provider, endpoint, method string,
	properties map[string]Property,
) *UploadedResource {
	resource := &UploadedResource{
		Direction:   Consumes,
		Interaction: RestRequest,
		Endpoint:    endpoint,
		Method:      method,
		Properties:  properties,
	}

	if provider != "" {
		resource.ConsumedProvider = null.StringFrom(provider)
	}

	return resource
}

func NewRestRequestProvider(
	endpoint, method string,
	properties map[string]Property,
) *UploadedResource {
	return &UploadedResource{
		Direction:   Provides,
		Interaction: RestRequest,
		Endpoint:    endpoint,
		Method:      method,
		Properties:  properties,
	}
}

func NewRestResponseConsumer(
	provider, endpoint, method, statusCode string,
	properties map[string]Property,
) *UploadedResource {
	resource := &UploadedResource{
		Direction:   Consumes,
		Interaction: RestResponse,
		Endpoint:    endpoint,
		Method:      method,
		Properties:  properties,
	}

	if provider != "" {
		resource.ConsumedProvider = null.StringFrom(provider)
	}

	if statusCode != "" {
		resource.ResponseStatusCode = null.StringFrom(statusCode)
	}

	return resource
}

func NewRestResponseProvider(
	endpoint, method, statusCode string,
	properties map[string]Property,
) *UploadedResource {
	resource := &UploadedResource{
		Direction:   Provides,
		Interaction: RestResponse,
		Endpoint:    endpoint,
		Method:      method,
		Properties:  properties,
	}

	if statusCode != "" {
		resource.ResponseStatusCode = null.StringFrom(statusCode)
	}

	return resource
}
