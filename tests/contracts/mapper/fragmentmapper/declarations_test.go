package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/goccy/go-yaml"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const orderedContractYAML = `consumes:
  zoo_api:
    rest:
      "/animals":
        get:
          responses:
            "200": Animal
  billing_api:
    rest:
      "/invoices":
        post:
          request: InvoiceCreate
          responses:
            "400": Problem
            "201": Invoice
provides:
  rest:
    "/pets/*":
      delete:
        responses:
          "204": Empty
      get:
        responses:
          "200": Pet
    "/pets":
      put:
        request: PetUpdate
      post:
        request: PetCreate
        responses:
          "201": Pet
      get:
        responses:
          "200": PetList
`

const petByIdYAML = `provides:
  rest:
    "/pets":
      get:
        responses:
          "200": Pet
schemas:
  Problem:
    type: object
    properties:
      title:
        type: string
  Pet:
    type: object
    properties:
      id:
        type: string
`

const petByNameYAML = `consumes:
  pets_api:
    rest:
      "/pets":
        get:
          responses:
            "200": Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const createPetYAML = `provides:
  rest:
    "/pets":
      post:
        request: PetCreate
        responses:
          "201": Pet
          "422": Problem
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
  PetCreate:
    type: object
    properties:
      name:
        type: string
  Problem:
    type: object
    properties:
      title:
        type: string
`

const slashedEndpointsYAML = `provides:
  rest:
    "/users/":
      get:
        responses:
          "200": User
    "/":
      get:
        responses:
          "200": Home
schemas:
  User:
    type: object
    properties:
      id:
        type: string
  Home:
    type: string
`

const unresolvedSchemaYAML = `provides:
  rest:
    "/ghosts":
      get:
        responses:
          "200": Ghost
`

const patchRoutesYAML = `consumes:
  pets_api:
    rest:
      "/pets/*":
        patch:
          request: PetPatch
          responses:
            "200": Pet
provides:
  rest:
    "/pets/*":
      patch:
        request: PetPatch
        responses:
          "200": Pet
schemas:
  PetPatch:
    type: object
    properties:
      name:
        type: string
  Pet:
    type: object
    properties:
      id:
        type: string
`

const everyMethodYAML = `provides:
  rest:
    "/pets/*":
      delete:
        responses:
          "204": Pet
      patch:
        responses:
          "200": Pet
      put:
        responses:
          "200": Pet
      post:
        responses:
          "201": Pet
      get:
        responses:
          "200": Pet
`

const undescribedBodiesYAML = `provides:
  rest:
    "/files/*":
      get:
        responses:
          "200": ~
      delete:
        responses:
          "204":
    "/files":
      post:
        request: ~
        responses:
          "201": File
      put:
        responses:
          "200": File
schemas:
  File:
    type: object
    properties:
      id:
        type: string
`

const emptySchemaNamesYAML = `provides:
  rest:
    "/files":
      post:
        request: ""
        responses:
          "204": ""
`

type declaredFile struct {
	source string
	raw    string
}

func declarationsOf(t *testing.T, files ...declaredFile) fragmentmapper.Declarations {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		var document any
		require.NoError(t, yaml.Unmarshal([]byte(file.raw), &document))

		fragments = append(fragments, contract.Fragment{Source: file.source, Document: document})
	}

	return fragmentmapper.ToDeclarations(fragments)
}

func resourcePaths(declarations fragmentmapper.Declarations) []string {
	paths := make([]string, 0, len(declarations.Resources))
	for _, declaration := range declarations.Resources {
		paths = append(paths, declaration.Path.String())
	}

	return paths
}

func TestToDeclarations_SortsResourcesByServiceEndpointMethodAndStatus(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"api.yaml", orderedContractYAML})

	assert.Equal(t, []string{
		"consumes;billing_api;rest;/invoices;post;request",
		"consumes;billing_api;rest;/invoices;post;responses;201",
		"consumes;billing_api;rest;/invoices;post;responses;400",
		"consumes;zoo_api;rest;/animals;get;responses;200",
		"provides;rest;/pets;get;responses;200",
		"provides;rest;/pets;post;request",
		"provides;rest;/pets;post;responses;201",
		"provides;rest;/pets;put;request",
		"provides;rest;/pets/*;get;responses;200",
		"provides;rest;/pets/*;delete;responses;204",
	}, resourcePaths(declarations))
}

func TestToDeclarations_SortsFragmentsBySourceNotByInputOrder(t *testing.T) {
	declarations := declarationsOf(t,
		declaredFile{"b.yaml", petByNameYAML},
		declaredFile{"a.yaml", petByIdYAML},
	)

	require.Len(t, declarations.Resources, 2)
	assert.Equal(t, "a.yaml", declarations.Resources[0].Source)
	assert.Equal(t, "provides;rest;/pets;get;responses;200", declarations.Resources[0].Path.String())
	assert.Equal(t, "b.yaml", declarations.Resources[1].Source)
	assert.Equal(t, "consumes;pets_api;rest;/pets;get;responses;200", declarations.Resources[1].Path.String())
}

func TestToDeclarations_SortsSchemasBySourceAndName(t *testing.T) {
	declarations := declarationsOf(t,
		declaredFile{"b.yaml", petByNameYAML},
		declaredFile{"a.yaml", petByIdYAML},
	)

	assert.Equal(t, []fragmentmapper.SchemaDeclaration{
		{Source: "a.yaml", Name: "Pet", Schema: contract.Schema{Type: "object", Properties: map[string]contract.Schema{"id": {Type: "string"}}}},
		{Source: "a.yaml", Name: "Problem", Schema: contract.Schema{Type: "object", Properties: map[string]contract.Schema{"title": {Type: "string"}}}},
		{Source: "b.yaml", Name: "Pet", Schema: contract.Schema{Type: "object", Properties: map[string]contract.Schema{"name": {Type: "string"}}}},
	}, declarations.Schemas)
}

func TestToDeclarations_CatalogKeepsTheFirstDeclarationOfEachName(t *testing.T) {
	declarations := declarationsOf(t,
		declaredFile{"b.yaml", petByNameYAML},
		declaredFile{"a.yaml", petByIdYAML},
	)

	assert.Equal(t, contract.SchemasMap{
		"Pet":     {Type: "object", Properties: map[string]contract.Schema{"id": {Type: "string"}}},
		"Problem": {Type: "object", Properties: map[string]contract.Schema{"title": {Type: "string"}}},
	}, declarations.Catalog)

	require.Len(t, declarations.Resources, 2)
	consumer := declarations.Resources[1]
	assert.Equal(t, "b.yaml", consumer.Source)
	assert.Equal(t, []model.ResourceBody{{Source: "b.yaml", SchemaName: null.StringFrom("Pet")}}, consumer.Resource.Bodies)
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string"}}, fragmentmapper.ToSchemaModels(declarations)["Pet"])
}

func TestToDeclarations_CapturesTheSchemaNameOfTheRequestAndOfEachResponse(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"pets.yaml", createPetYAML})

	require.Len(t, declarations.Resources, 3)

	request := declarations.Resources[0]
	assert.Equal(t, "PetCreate", request.SchemaName)
	assert.Equal(t, model.RestRequest, request.Resource.Interaction)
	assert.Equal(t, []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("PetCreate")}}, request.Resource.Bodies)

	created := declarations.Resources[1]
	assert.Equal(t, "Pet", created.SchemaName)
	assert.Equal(t, "201", created.Resource.ResponseStatusCode.String)
	assert.Equal(t, []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")}}, created.Resource.Bodies)

	rejected := declarations.Resources[2]
	assert.Equal(t, "Problem", rejected.SchemaName)
	assert.Equal(t, "422", rejected.Resource.ResponseStatusCode.String)
	assert.Equal(t, []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("Problem")}}, rejected.Resource.Bodies)
}

func TestToDeclarations_NormalizesTheEndpointInThePath(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"users.yaml", slashedEndpointsYAML})

	assert.Equal(t, []string{
		"provides;rest;/;get;responses;200",
		"provides;rest;/users;get;responses;200",
	}, resourcePaths(declarations))

	require.Len(t, declarations.Resources, 2)
	assert.Equal(t, "/", declarations.Resources[0].Resource.Endpoint)
	assert.Equal(t, "/users", declarations.Resources[1].Resource.Endpoint)
}

func TestToDeclarations_UnresolvedSchemaName_StaysInTheBody(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"ghosts.yaml", unresolvedSchemaYAML})

	require.Len(t, declarations.Resources, 1)
	assert.Equal(t, "Ghost", declarations.Resources[0].SchemaName)
	assert.NotContains(t, declarations.Catalog, "Ghost")
	assert.Equal(t, []model.ResourceBody{{Source: "ghosts.yaml", SchemaName: null.StringFrom("Ghost")}}, declarations.Resources[0].Resource.Bodies)
}

func TestToDeclarations_NullBody_DeclaresTheResourceWithABodyWithoutSchema(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"files.yaml", undescribedBodiesYAML})

	assert.Equal(t, []string{
		"provides;rest;/files;post;request",
		"provides;rest;/files;post;responses;201",
		"provides;rest;/files;put;responses;200",
		"provides;rest;/files/*;get;responses;200",
		"provides;rest;/files/*;delete;responses;204",
	}, resourcePaths(declarations))

	withoutSchema := map[string]bool{}
	for _, declaration := range declarations.Resources {
		withoutSchema[declaration.Path.String()] = declaration.WithoutSchema
	}

	assert.Equal(t, map[string]bool{
		"provides;rest;/files;post;request":           true,
		"provides;rest;/files;post;responses;201":     false,
		"provides;rest;/files;put;responses;200":      false,
		"provides;rest;/files/*;get;responses;200":    true,
		"provides;rest;/files/*;delete;responses;204": true,
	}, withoutSchema)

	undescribed := []model.ResourceBody{{Source: "files.yaml"}}

	assert.Equal(t, *model.NewRestRequestProvider("/files", "post", undescribed), declarations.Resources[0].Resource)
	assert.Empty(t, declarations.Resources[0].SchemaName)
	assert.Equal(t, *model.NewRestResponseProvider("/files/*", "get", "200", undescribed), declarations.Resources[3].Resource)
	assert.Equal(t, *model.NewRestResponseProvider("/files/*", "delete", "204", undescribed), declarations.Resources[4].Resource)
}

func TestToDeclarations_EmptyStringBodyIsAnUnresolvedSchemaName(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"files.yaml", emptySchemaNamesYAML})

	require.Len(t, declarations.Resources, 2)

	for _, declaration := range declarations.Resources {
		assert.False(t, declaration.WithoutSchema, declaration.Path.String())
		assert.Empty(t, declaration.SchemaName, declaration.Path.String())
		assert.Equal(t, []model.ResourceBody{{Source: "files.yaml", SchemaName: null.StringFrom("")}}, declaration.Resource.Bodies, declaration.Path.String())
	}

	assert.Equal(t, []string{
		"provides;rest;/files;post;request",
		"provides;rest;/files;post;responses;204",
	}, resourcePaths(declarations))
}

func TestToDeclarations_PatchRoute_DeclaresRequestAndResponseInProvidesAndConsumes(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"pets.yaml", patchRoutesYAML})

	assert.Equal(t, []string{
		"consumes;pets_api;rest;/pets/*;patch;request",
		"consumes;pets_api;rest;/pets/*;patch;responses;200",
		"provides;rest;/pets/*;patch;request",
		"provides;rest;/pets/*;patch;responses;200",
	}, resourcePaths(declarations))

	requestBodies := []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("PetPatch")}}
	responseBodies := []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")}}

	resources := make([]model.UploadedResource, 0, len(declarations.Resources))
	for _, declaration := range declarations.Resources {
		resources = append(resources, declaration.Resource)
	}

	assert.Equal(t, []model.UploadedResource{
		*model.NewRestRequestConsumer("pets_api", "/pets/*", "patch", requestBodies),
		*model.NewRestResponseConsumer("pets_api", "/pets/*", "patch", "200", responseBodies),
		*model.NewRestRequestProvider("/pets/*", "patch", requestBodies),
		*model.NewRestResponseProvider("/pets/*", "patch", "200", responseBodies),
	}, resources)
}

func TestToDeclarations_PatchSortsBetweenPutAndDelete(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"pets.yaml", everyMethodYAML})

	assert.Equal(t, []string{
		"provides;rest;/pets/*;get;responses;200",
		"provides;rest;/pets/*;post;responses;201",
		"provides;rest;/pets/*;put;responses;200",
		"provides;rest;/pets/*;patch;responses;200",
		"provides;rest;/pets/*;delete;responses;204",
	}, resourcePaths(declarations))
}

func TestToDeclarations_FragmentWithoutDocument_DeclaresNothing(t *testing.T) {
	declarations := fragmentmapper.ToDeclarations([]contract.Fragment{{Source: "empty.yaml"}})

	assert.Empty(t, declarations.Resources)
	assert.Empty(t, declarations.Schemas)
	assert.NotNil(t, declarations.Catalog)
	assert.Empty(t, declarations.Catalog)
	assert.Empty(t, fragmentmapper.ToResourceModels(declarations))
	assert.Empty(t, fragmentmapper.ToSchemaModels(declarations))
}
