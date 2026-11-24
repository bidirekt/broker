package integration_test

import (
	"context"
	"net/http"
)

const sdlPublishParticipant = "petstore_api"

const sdlPublishQuerySDL = `type Query {
  pet(id: ID!): Pet
  pets(first: Int! = 10): [Pet!]!
}

type Mutation {
  adoptPet(id: ID!): Pet!
}
`

const sdlPublishTypesSDL = `type Pet {
  id: ID!
  owner: Owner
}

type Owner {
  name: String!
}

type Product {
  sku: String!
}
`

const sdlPublishRESTYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

func (this *IntegrationSuite) createSDLPublishParticipant() {
	status, body := this.post("/api/participants", `{"participant":"`+sdlPublishParticipant+`"}`)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) publishSDL(version string, files ...contractFragment) (status int, body string) {
	return this.post("/api/contracts", this.publishBody(sdlPublishParticipant, version, files...))
}

func (this *IntegrationSuite) sdlPublishStrings(query string) []string {
	rows, err := this.Pool.Query(context.Background(), query)
	this.Require().NoError(err)
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		this.Require().NoError(rows.Scan(&value))
		values = append(values, value)
	}
	this.Require().NoError(rows.Err())

	return values
}

func (this *IntegrationSuite) assertNothingPublishedFromSDL() {
	for _, table := range []string{"contracts", "contract_versions", "resources", "graphql_resources", "schemas", "fields"} {
		this.Equal(0, this.countRows(table), table)
	}
}

func (this *IntegrationSuite) TestPublishSDL_TwoFilesThatReferenceEachOther_StoreTwoResourcesPerRootField() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1",
		contractFragment{"graph/query.graphqls", sdlPublishQuerySDL},
		contractFragment{"graph/types.graphqls", sdlPublishTypesSDL},
	)

	this.Require().Equal(http.StatusOK, status, body)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal([]string{
		"mutation adoptPet arguments",
		"mutation adoptPet returns",
		"query pet arguments",
		"query pet returns",
		"query pets arguments",
		"query pets returns",
	}, this.sdlPublishStrings(`
		SELECT operation || ' ' || field || ' ' || interaction
		FROM graphql_resources
		ORDER BY operation || ' ' || field || ' ' || interaction COLLATE "C"`))
	this.Equal([]string{
		"input Mutation:adoptPet:arguments",
		"output Mutation:adoptPet:returns",
		"input Query:pet:arguments",
		"output Query:pet:returns",
		"input Query:pets:arguments",
		"output Query:pets:returns",
	}, this.sdlPublishStrings(`
		SELECT r.body_role || ' ' || s.name
		FROM resources r
		JOIN graphql_resources gr ON gr.resource_id = r.id
		JOIN resource_bodies rb ON rb.resource_id = r.id
		JOIN schemas s ON s.id = rb.schema_id
		WHERE r.direction = 'provides' AND s.namespace = 'graphql'
		ORDER BY gr.operation COLLATE "C", gr.field COLLATE "C", gr.interaction COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishSDL_EveryType_IsStoredInTheGraphQLNamespace_EvenOneNoRootFieldReaches() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1",
		contractFragment{"graph/query.graphqls", sdlPublishQuerySDL},
		contractFragment{"graph/types.graphqls", sdlPublishTypesSDL},
	)
	this.Require().Equal(http.StatusOK, status, body)

	this.Equal([]string{
		"graphql Mutation",
		"graphql Mutation:adoptPet:arguments",
		"graphql Mutation:adoptPet:returns",
		"graphql Owner",
		"graphql Pet",
		"graphql Product",
		"graphql Query",
		"graphql Query:pet:arguments",
		"graphql Query:pet:returns",
		"graphql Query:pets:arguments",
		"graphql Query:pets:returns",
	}, this.sdlPublishStrings(`SELECT namespace || ' ' || name FROM schemas ORDER BY name COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishSDL_TheSameSDLRepublished_ReusesTheContract() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1", contractFragment{"schema.graphqls", sdlPublishQuerySDL + sdlPublishTypesSDL})
	this.Require().Equal(http.StatusOK, status, body)

	status, body = this.publishSDL("2",
		contractFragment{"graph/query.graphqls", sdlPublishQuerySDL},
		contractFragment{"graph/types.graphqls", sdlPublishTypesSDL},
	)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))
	this.Equal(6, this.countRows("graphql_resources"))
}

func (this *IntegrationSuite) TestPublishSDL_APetInTheYAMLAndATypePetInTheSDL_AreBothStored() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1",
		contractFragment{"api.yaml", sdlPublishRESTYAML},
		contractFragment{"schema.graphqls", sdlPublishQuerySDL + sdlPublishTypesSDL},
	)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal([]string{"graphql", "yaml"}, this.sdlPublishStrings(
		`SELECT namespace FROM schemas WHERE name = 'Pet' ORDER BY namespace COLLATE "C"`))
	this.Equal([]string{"graphql id", "graphql owner", "yaml name"}, this.sdlPublishStrings(`
		SELECT s.namespace || ' ' || f.path
		FROM fields f
		JOIN schemas s ON s.id = f.schema_id
		WHERE s.name = 'Pet' AND f.path <> ''
		ORDER BY s.namespace || ' ' || f.path COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishSDL_AnInvalidSDL_IsA400WithTheParserTextAndNothingStored() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1", contractFragment{"schema.graphqls", "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  id: ID!\n  name: String!\n  bornAt: String\n  tags: [String!]!\n  age: Int\n  weight: Float\n  owner: Owner\n}\n"})

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:12:10: Undefined type Owner."}`, body)
	this.assertNothingPublishedFromSDL()
}

func (this *IntegrationSuite) TestPublishSDL_ADocumentOfOperations_IsA400AndNothingStored() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1",
		contractFragment{"schema.graphqls", sdlPublishQuerySDL + sdlPublishTypesSDL},
		contractFragment{"queries.graphql", "query {\n  pet(id: 1) {\n    id\n  }\n}\n"},
	)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"queries.graphql: holds operations; publish expects the GraphQL schema"}`, body)
	this.assertNothingPublishedFromSDL()
}

func (this *IntegrationSuite) TestCanIDeploy_AGraphQLProviderWithoutConsumers_IsDeployable() {
	this.createSDLPublishParticipant()

	status, body := this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)
	status, body = this.publishSDL("1", contractFragment{"schema.graphqls", sdlPublishQuerySDL + sdlPublishTypesSDL})
	this.Require().Equal(http.StatusOK, status, body)

	status, body = this.post("/api/can-i-deploy", `{"participant":"petstore_api","version":"1","environment":"production"}`)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "petstore_api",
		"version": "1",
		"environment": "production",
		"deployable": true,
		"results": {}
	}`, body)
}
