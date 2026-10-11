package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newContractWithOnePetsResource(participantName string) *model.UploadedContract {
	contract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = contract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false, false),
		"$.id": model.NewProperty("$.id", "string", false, false),
	}))
	return contract
}

func TestContract_Checksum_IsStableForEquivalentContracts(t *testing.T) {
	a := newContractWithOnePetsResource("pets-service")
	b := newContractWithOnePetsResource("pets-service")

	assert.Equal(t, a.Checksum(), b.Checksum())
}

func TestContract_Checksum_DiffersWhenResourceAdded(t *testing.T) {
	a := newContractWithOnePetsResource("pets-service")

	b := newContractWithOnePetsResource("pets-service")
	require.NoError(t, b.AddResource(model.NewRestResponseProvider("/pets/*", "get", "200", map[string]model.Property{
		"$": model.NewProperty("$", "object", false, false),
	})))

	assert.NotEqual(t, a.Checksum(), b.Checksum())
}

func TestContract_Checksum_IsOrderIndependent(t *testing.T) {
	first := model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false, false),
		"$.id": model.NewProperty("$.id", "string", false, false),
	})
	second := model.NewRestResponseProvider("/pets/*", "get", "200", map[string]model.Property{
		"$": model.NewProperty("$", "object", false, false),
	})

	a := model.NewUploadedContract(0, "pets-service", "1", "raw")
	require.NoError(t, a.AddResource(first))
	require.NoError(t, a.AddResource(second))

	b := model.NewUploadedContract(0, "pets-service", "1", "raw")
	require.NoError(t, b.AddResource(second))
	require.NoError(t, b.AddResource(first))

	assert.Equal(t, a.Checksum(), b.Checksum())
}

func TestContract_AddResource_RejectsAlreadyAddedHash(t *testing.T) {
	contract := newContractWithOnePetsResource("pets-service")

	err := contract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$": model.NewProperty("$", "object", false, false),
	}))

	require.EqualError(
		t,
		err,
		"resource already added: provides GET /pets 200",
	)
	assert.Len(t, contract.Resources, 1)
}

func TestContract_AddResource_DescribesEachResourceShapeInTheInvariant(t *testing.T) {
	cases := []struct {
		name     string
		resource *model.UploadedResource
		want     string
	}{
		{
			name:     "provides response",
			resource: model.NewRestResponseProvider("/pets", "get", "200", nil),
			want:     "resource already added: provides GET /pets 200",
		},
		{
			name:     "provides request",
			resource: model.NewRestRequestProvider("/pets", "post", nil),
			want:     "resource already added: provides POST /pets request",
		},
		{
			name:     "consumes response",
			resource: model.NewRestResponseConsumer("payments", "/invoices", "get", "200", nil),
			want:     "resource already added: consumes payments GET /invoices 200",
		},
		{
			name:     "consumes request",
			resource: model.NewRestRequestConsumer("payments", "/invoices", "post", nil),
			want:     "resource already added: consumes payments POST /invoices request",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			contract := model.NewUploadedContract(0, "pets-service", "1", "raw")
			require.NoError(t, contract.AddResource(testCase.resource))

			require.EqualError(t, contract.AddResource(testCase.resource), testCase.want)
		})
	}
}
