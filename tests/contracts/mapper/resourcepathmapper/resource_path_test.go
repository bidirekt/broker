package resourcepathmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/resourcepathmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestToResourceModel_ConsumerRestRequest_Parses(t *testing.T) {
	path := contract.NewResourcePath("consumes;pets-service;rest;/pets;post;request")

	resource := resourcepathmapper.ToResourceModel(path, nil)

	assert.Equal(t, model.Consumes, resource.Direction)
	assert.Equal(t, model.RestRequest, resource.Interaction)
	assert.Equal(t, "pets-service", resource.ConsumedProvider.String)
	assert.Equal(t, "/pets", resource.Endpoint)
	assert.Equal(t, "post", resource.Method)
	assert.Empty(t, resource.ResponseStatusCode)
}

func TestToResourceModel_ProviderRestRequest_Parses(t *testing.T) {
	path := contract.NewResourcePath("provides;rest;/pets;post;request")

	resource := resourcepathmapper.ToResourceModel(path, nil)

	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.RestRequest, resource.Interaction)
	assert.Empty(t, resource.ConsumedProvider)
	assert.Equal(t, "/pets", resource.Endpoint)
	assert.Equal(t, "post", resource.Method)
}

func TestToResourceModel_ProviderRestResponse_Parses(t *testing.T) {
	path := contract.NewResourcePath("provides;rest;/pets;get;responses;200")

	resource := resourcepathmapper.ToResourceModel(path, nil)

	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.RestResponse, resource.Interaction)
	assert.Equal(t, "/pets", resource.Endpoint)
	assert.Equal(t, "get", resource.Method)
	assert.Equal(t, "200", resource.ResponseStatusCode.String)
}

func TestToResourceModel_UnrecognizedPath_Panics(t *testing.T) {
	path := contract.NewResourcePath("garbage;not;a;real;path")

	assert.Panics(t, func() { resourcepathmapper.ToResourceModel(path, nil) })
}
