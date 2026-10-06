package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/goccy/go-yaml"
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

func TestToDeclarations_OrdenaRecursosPorServicoEndpointMetodoEStatus(t *testing.T) {
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

func TestToDeclarations_OrdenaFragmentosPorOrigemNaoPelaOrdemDeEntrada(t *testing.T) {
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

func TestToDeclarations_OrdenaSchemasPorOrigemENome(t *testing.T) {
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

func TestToDeclarations_CatalogoFicaComAPrimeiraDeclaracaoDeCadaNome(t *testing.T) {
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
	assert.Contains(t, consumer.Resource.Properties, "$.id")
	assert.NotContains(t, consumer.Resource.Properties, "$.name")
}

func TestToDeclarations_CapturaSchemaNameDoRequestEDasResponses(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"pets.yaml", createPetYAML})

	require.Len(t, declarations.Resources, 3)

	request := declarations.Resources[0]
	assert.Equal(t, "PetCreate", request.SchemaName)
	assert.Equal(t, model.RestRequest, request.Resource.Interaction)
	assert.Contains(t, request.Resource.Properties, "$.name")

	created := declarations.Resources[1]
	assert.Equal(t, "Pet", created.SchemaName)
	assert.Equal(t, "201", created.Resource.ResponseStatusCode.String)
	assert.Contains(t, created.Resource.Properties, "$.id")

	rejected := declarations.Resources[2]
	assert.Equal(t, "Problem", rejected.SchemaName)
	assert.Equal(t, "422", rejected.Resource.ResponseStatusCode.String)
	assert.Contains(t, rejected.Resource.Properties, "$.title")
}

func TestToDeclarations_NormalizaEndpointNoPath(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"users.yaml", slashedEndpointsYAML})

	assert.Equal(t, []string{
		"provides;rest;/;get;responses;200",
		"provides;rest;/users;get;responses;200",
	}, resourcePaths(declarations))

	require.Len(t, declarations.Resources, 2)
	assert.Equal(t, "/", declarations.Resources[0].Resource.Endpoint)
	assert.Equal(t, "/users", declarations.Resources[1].Resource.Endpoint)
}

func TestToDeclarations_NomeNaoResolvidoGeraPropriedadeDoSchemaZero(t *testing.T) {
	declarations := declarationsOf(t, declaredFile{"ghosts.yaml", unresolvedSchemaYAML})

	require.Len(t, declarations.Resources, 1)
	assert.Equal(t, "Ghost", declarations.Resources[0].SchemaName)
	assert.NotContains(t, declarations.Catalog, "Ghost")
	assert.Equal(t, map[string]model.Property{"$": {Path: "$"}}, declarations.Resources[0].Resource.Properties)
}

func TestToDeclarations_FragmentoSemDocumentoNaoDeclaraNada(t *testing.T) {
	declarations := fragmentmapper.ToDeclarations([]contract.Fragment{{Source: "empty.yaml"}})

	assert.Empty(t, declarations.Resources)
	assert.Empty(t, declarations.Schemas)
	assert.NotNil(t, declarations.Catalog)
	assert.Empty(t, declarations.Catalog)
	assert.Empty(t, fragmentmapper.ToResourceModels(declarations))
}
