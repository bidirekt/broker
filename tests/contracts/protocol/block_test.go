package protocol_test

import (
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

var consumedPetsBlock = protocol.Block{
	Source:           "web.yaml",
	Path:             contract.NewResourcePath("consumes;pets_api;rest"),
	Direction:        model.Consumes,
	ConsumedProvider: null.StringFrom("pets_api"),
	Protocol:         restmodel.Protocol,
}

func TestToResourceDeclaration_ANamedSchema_IsTheBody(t *testing.T) {
	path := contract.NewResourcePath("consumes;pets_api;rest;/pets;get;responses;200")
	identity := restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}

	declaration := consumedPetsBlock.ToResourceDeclaration(path, model.Output, identity, "Pet")

	assert.Equal(t, protocol.ResourceDeclaration{
		Source:     "web.yaml",
		Path:       path,
		SchemaName: "Pet",
		Resource: model.UploadedResource{Resource: model.Resource{
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Output,
			Identity:         identity,
			ConsumedProvider: null.StringFrom("pets_api"),
			Bodies:           []model.ResourceBody{{Source: "web.yaml", SchemaName: null.StringFrom("Pet")}},
		}},
	}, declaration)
}

func TestToResourceDeclaration_ANullSchemaRef_IsABodyWithoutSchema(t *testing.T) {
	path := contract.NewResourcePath("consumes;pets_api;rest;/pets;post;request")
	identity := restmodel.Identity{Endpoint: "/pets", Method: "post"}

	declaration := consumedPetsBlock.ToResourceDeclaration(path, model.Input, identity, nil)

	assert.True(t, declaration.WithoutSchema)
	assert.Empty(t, declaration.SchemaName)
	assert.Equal(t, []model.ResourceBody{{Source: "web.yaml"}}, declaration.Resource.Bodies)
}
