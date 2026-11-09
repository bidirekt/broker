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

func TestResource_Hashes_KeepTheValuesFromBeforeTheProtocolRefactor(t *testing.T) {
	cases := []struct {
		name         string
		resource     *model.UploadedResource
		providerHash string
		consumerHash string
	}{
		{
			name:         "provides GET /pets 200",
			resource:     restResource("petstore_api", "", "/pets", "get", "200"),
			providerHash: "4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4",
		},
		{
			name:         "provides POST /pets request",
			resource:     restResource("petstore_api", "", "/pets", "post", ""),
			providerHash: "821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf",
		},
		{
			name:         "consumes petstore_api GET /pets 200",
			resource:     restResource("petstore_web", "petstore_api", "/pets", "get", "200"),
			providerHash: "4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4",
			consumerHash: "0c6429446ddbdc49fcbc14dd1f160a345f7d30ce6bbd63660f019df6517b2243",
		},
		{
			name:         "consumes petstore_api POST /pets request",
			resource:     restResource("petstore_web", "petstore_api", "/pets", "post", ""),
			providerHash: "821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf",
			consumerHash: "66c18366e53a4ae1638906821bab8b81376821afe29c3f1ed448dfca299e427a",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.providerHash, testCase.resource.ProviderHash())
			assert.Equal(t, testCase.consumerHash, testCase.resource.ConsumerHash())
		})
	}
}
