package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newContractWithOnePetsResource(participantName string) *model.UploadedContract {
	contract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = contract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", []model.ResourceBody{
		{Source: "pets.yaml", SchemaName: null.StringFrom("PetList")},
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
	require.NoError(t, b.AddResource(model.NewRestResponseProvider("/pets/*", "get", "200", []model.ResourceBody{
		{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")},
	})))

	assert.NotEqual(t, a.Checksum(), b.Checksum())
}

func TestContract_Checksum_IsOrderIndependent(t *testing.T) {
	first := model.NewRestResponseProvider("/pets", "get", "200", []model.ResourceBody{
		{Source: "pets.yaml", SchemaName: null.StringFrom("PetList")},
	})
	second := model.NewRestResponseProvider("/pets/*", "get", "200", []model.ResourceBody{
		{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")},
	})

	a := model.NewUploadedContract(0, "pets-service", "1", "raw")
	require.NoError(t, a.AddResource(first))
	require.NoError(t, a.AddResource(second))

	b := model.NewUploadedContract(0, "pets-service", "1", "raw")
	require.NoError(t, b.AddResource(second))
	require.NoError(t, b.AddResource(first))

	assert.Equal(t, a.Checksum(), b.Checksum())
}

func newOrdersContract(version string, schemaName string, bodies []model.ResourceBody) *model.UploadedContract {
	contract := model.NewUploadedContract(0, "orders-service", version, "raw "+version)
	contract.Schemas = model.Schemas{
		schemaName: {
			"":     {Type: "object"},
			"city": {Type: "string", Optional: true},
		},
	}
	_ = contract.AddResource(model.NewRestRequestConsumer("addresses-service", "/addresses", "post", bodies))
	return contract
}

func addressBodies(schemaName string) []model.ResourceBody {
	return []model.ResourceBody{
		{Source: "a.yaml", SchemaName: null.StringFrom(schemaName)},
		{Source: "b.yaml"},
		{Source: "b.yaml", SchemaName: null.StringFrom(schemaName)},
	}
}

func TestContract_Checksum_IgnoresTheVersionAndTheRawContent(t *testing.T) {
	first := newOrdersContract("1", "Address", addressBodies("Address"))
	second := newOrdersContract("2", "Address", addressBodies("Address"))

	assert.Equal(t, first.Checksum(), second.Checksum())
}

func TestContract_Checksum_IgnoresTheOrderOfTheBodies(t *testing.T) {
	bodies := addressBodies("Address")
	reversed := []model.ResourceBody{bodies[2], bodies[1], bodies[0]}

	inOrder := newOrdersContract("1", "Address", bodies)
	outOfOrder := newOrdersContract("1", "Address", reversed)

	assert.Equal(t, inOrder.Checksum(), outOfOrder.Checksum())
	for _, resource := range outOfOrder.Resources {
		assert.Equal(t, reversed, resource.Bodies)
	}
}

func TestContract_Checksum_IgnoresTheSourcesOfTheBodies(t *testing.T) {
	inTwoFiles := newOrdersContract("1", "Address", addressBodies("Address"))
	inOtherFiles := newOrdersContract("1", "Address", []model.ResourceBody{
		{Source: "orders/a.yaml", SchemaName: null.StringFrom("Address")},
		{Source: "orders/b.yaml", SchemaName: null.StringFrom("Address")},
		{Source: "orders/c.yaml"},
	})

	assert.Equal(t, inTwoFiles.Checksum(), inOtherFiles.Checksum())
}

func TestContract_Checksum_DiffersWhenABodyIsAdded(t *testing.T) {
	bodies := addressBodies("Address")

	three := newOrdersContract("1", "Address", bodies)
	two := newOrdersContract("1", "Address", bodies[:2])

	assert.NotEqual(t, three.Checksum(), two.Checksum())
}

func TestContract_Checksum_DiffersWhenASchemaIsRenamed(t *testing.T) {
	address := newOrdersContract("1", "Address", addressBodies("Address"))
	postalAddress := newOrdersContract("1", "PostalAddress", addressBodies("PostalAddress"))

	assert.NotEqual(t, address.Checksum(), postalAddress.Checksum())
}

func TestContract_Checksum_DiffersWhenABodyBecomesUndescribed(t *testing.T) {
	described := newOrdersContract("1", "Address", []model.ResourceBody{{Source: "a.yaml", SchemaName: null.StringFrom("Address")}})
	undescribed := newOrdersContract("1", "Address", []model.ResourceBody{{Source: "a.yaml"}})

	assert.NotEqual(t, described.Checksum(), undescribed.Checksum())
}

func TestContract_Checksum_DiffersWhenAFieldChanges(t *testing.T) {
	cases := []struct {
		name  string
		field model.Field
	}{
		{name: "type", field: model.Field{Type: "integer", Optional: true}},
		{name: "optional", field: model.Field{Type: "string"}},
		{name: "nullable", field: model.Field{Type: "string", Optional: true, Nullable: true}},
		{name: "ref", field: model.Field{Ref: "City", Optional: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			original := newOrdersContract("1", "Address", addressBodies("Address"))
			changed := newOrdersContract("1", "Address", addressBodies("Address"))
			changed.Schemas["Address"]["city"] = testCase.field

			assert.NotEqual(t, original.Checksum(), changed.Checksum())
		})
	}
}

func TestContract_Checksum_DiffersWhenAFieldIsAdded(t *testing.T) {
	original := newOrdersContract("1", "Address", addressBodies("Address"))
	changed := newOrdersContract("1", "Address", addressBodies("Address"))
	changed.Schemas["Address"]["zip"] = model.Field{Type: "string"}

	assert.NotEqual(t, original.Checksum(), changed.Checksum())
}

func TestContract_AddResource_RejectsAlreadyAddedHash(t *testing.T) {
	contract := newContractWithOnePetsResource("pets-service")

	err := contract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", []model.ResourceBody{
		{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")},
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
