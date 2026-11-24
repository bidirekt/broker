package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func restResource(participant, consumedProvider, endpoint, method, status string) *model.UploadedResource {
	resource := &model.UploadedResource{Resource: model.Resource{
		ParticipantName: participant,
		Direction:       model.Provides,
		Protocol:        restmodel.Protocol,
		Body:            model.Output,
		Identity:        restmodel.Identity{Endpoint: endpoint, Method: method, Status: status},
	}}

	if status == "" {
		resource.Body = model.Input
	}

	if consumedProvider != "" {
		resource.Direction = model.Consumes
		resource.ConsumedProvider = null.StringFrom(consumedProvider)
	}

	return resource
}

func TestResource_ProviderHash_BridgesConsumerAndProvider(t *testing.T) {
	consumer := restResource("web-app", "pets-service", "/pets", "get", "200")
	provider := restResource("pets-service", "", "/pets", "get", "200")

	// A consumer derives the provider hash from the provider it consumes; the
	// provider derives it from its own participant name. They must match.
	assert.Equal(t, consumer.ProviderHash(), provider.ProviderHash())
}

func TestResource_ConsumerHash_EmptyForProvidedResource(t *testing.T) {
	provider := restResource("pets-service", "", "/pets", "get", "200")

	assert.Empty(t, provider.ConsumerHash())
}

func TestResource_PrimaryHash_ConsumerDirection_EqualsConsumerHash(t *testing.T) {
	consumer := restResource("web-app", "pets-service", "/pets", "get", "200")

	assert.Equal(t, consumer.ConsumerHash(), consumer.PrimaryHash())
}

func TestResource_ConsumerHash_RestResponse_IncludesStatusCode(t *testing.T) {
	a := restResource("web-app", "pets-service", "/pets", "get", "200")
	b := restResource("web-app", "pets-service", "/pets", "get", "404")

	assert.NotEqual(t, a.ConsumerHash(), b.ConsumerHash())
}

func TestResource_Hashes_IncludeTheProtocol(t *testing.T) {
	cases := []struct {
		name         string
		resource     *model.UploadedResource
		providerHash string
		consumerHash string
	}{
		{
			name:         "provides GET /pets 200",
			resource:     restResource("petstore_api", "", "/pets", "get", "200"),
			providerHash: "37813efdc420db42e3868749998e97ecd2808ace36c32d3b3d419eac850ee314",
		},
		{
			name:         "provides POST /pets request",
			resource:     restResource("petstore_api", "", "/pets", "post", ""),
			providerHash: "0b96b38688a352c7097de6ccb0c214ccf09005914a56503eeae2ea6c28a81aaa",
		},
		{
			name:         "consumes petstore_api GET /pets 200",
			resource:     restResource("petstore_web", "petstore_api", "/pets", "get", "200"),
			providerHash: "37813efdc420db42e3868749998e97ecd2808ace36c32d3b3d419eac850ee314",
			consumerHash: "bcf5c1d280cf9726a966b13fd9aae98751c3288fb75d99d6224d52371deb2f72",
		},
		{
			name:         "consumes petstore_api POST /pets request",
			resource:     restResource("petstore_web", "petstore_api", "/pets", "post", ""),
			providerHash: "0b96b38688a352c7097de6ccb0c214ccf09005914a56503eeae2ea6c28a81aaa",
			consumerHash: "ef0ddff5d7ab1aadb3475059d488dd3be7fa2511b09a2e2388e0ee45e0a4bee1",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.providerHash, testCase.resource.ProviderHash())
			assert.Equal(t, testCase.consumerHash, testCase.resource.ConsumerHash())
		})
	}
}
