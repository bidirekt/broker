package integration_test

import (
	"context"
	"net/http"
	"slices"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
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

const sdlPublishSubscriptionSDL = `type Query {
  pet: Pet
}

type Subscription {
  petAdded: Pet!
}

type Pet {
  id: ID!
  updates: Subscription
}
`

const sdlPublishSharedRootSDL = `schema {
  query: Root
  mutation: Root
}

type Root {
  pet(id: ID!): Pet
  pets: [Pet!]!
}

type Pet {
  id: ID!
}
`

const sdlPublishFirstVersionSDL = `type Query {
  pet(id: ID!): Pet
  pets: [Pet!]!
}

type Pet {
  id: ID!
}
`

const sdlPublishSecondVersionSDL = `type Query {
  pet(id: ID!): Pet
  owner: Owner
}

type Pet {
  id: ID
}

type Owner {
  name: String!
}
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

func sdlPublishResources(contract *model.PersistedContract) []string {
	resources := make([]string, 0, len(contract.Resources))
	for _, resource := range contract.Resources {
		identity := resource.Identity.(graphqlmodel.Identity)
		line := identity.Operation + " " + identity.Field + " " + identity.Interaction
		if resource.Removed {
			line += " removed"
		}

		resources = append(resources, line)
	}

	slices.Sort(resources)

	return resources
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

func (this *IntegrationSuite) TestPublishSDL_ATypePointingAtTheSubscriptionRoot_StoresTheRootAsASchema() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1", contractFragment{"schema.graphqls", sdlPublishSubscriptionSDL})

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal([]string{
		"graphql Pet",
		"graphql Query",
		"graphql Query:pet:arguments",
		"graphql Query:pet:returns",
		"graphql Subscription",
	}, this.sdlPublishStrings(`SELECT namespace || ' ' || name FROM schemas ORDER BY name COLLATE "C"`))
	this.Equal([]string{"query pet arguments", "query pet returns"}, this.sdlPublishStrings(`
		SELECT operation || ' ' || field || ' ' || interaction
		FROM graphql_resources
		ORDER BY operation || ' ' || field || ' ' || interaction COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishSDL_OneTypeAsTheQueryAndTheMutationRoot_StoresItsFieldsOnce() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1", contractFragment{"schema.graphqls", sdlPublishSharedRootSDL})

	this.Require().Equal(http.StatusOK, status, body)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal([]string{
		"query pet arguments",
		"query pet returns",
		"query pets arguments",
		"query pets returns",
	}, this.sdlPublishStrings(`
		SELECT operation || ' ' || field || ' ' || interaction
		FROM graphql_resources
		ORDER BY operation || ' ' || field || ' ' || interaction COLLATE "C"`))
	this.Equal([]string{
		"graphql Pet",
		"graphql Root",
		"graphql Root:pet:arguments",
		"graphql Root:pet:returns",
		"graphql Root:pets:arguments",
		"graphql Root:pets:returns",
	}, this.sdlPublishStrings(`SELECT namespace || ' ' || name FROM schemas ORDER BY name COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishSDL_ASecondVersionThatChangesTheSDL_IsANewSnapshot() {
	this.createSDLPublishParticipant()

	status, body := this.publishSDL("1", contractFragment{"schema.graphqls", sdlPublishFirstVersionSDL})
	this.Require().Equal(http.StatusOK, status, body)
	status, body = this.publishSDL("2", contractFragment{"schema.graphqls", sdlPublishSecondVersionSDL})
	this.Require().Equal(http.StatusOK, status, body)

	this.Equal([]string{
		"query owner arguments",
		"query owner returns",
		"query pet arguments",
		"query pet returns",
		"query pets arguments",
		"query pets returns",
	}, this.sdlPublishStrings(`
		SELECT operation || ' ' || field || ' ' || interaction
		FROM graphql_resources
		ORDER BY operation || ' ' || field || ' ' || interaction COLLATE "C"`))
	this.Equal([]string{
		"1 query pet arguments added",
		"1 query pet returns added",
		"1 query pets arguments added",
		"1 query pets returns added",
		"2 query owner arguments added",
		"2 query owner returns added",
		"2 query pets arguments removed",
		"2 query pets returns removed",
	}, this.sdlPublishStrings(`
		SELECT cv.version || ' ' || gr.operation || ' ' || gr.field || ' ' || gr.interaction || ' ' || rv.change_type
		FROM resource_versions rv
		JOIN graphql_resources gr ON gr.resource_id = rv.resource_id
		JOIN contract_versions cv ON cv.contract_id = rv.contract_id
		ORDER BY cv.version || ' ' || gr.operation || ' ' || gr.field || ' ' || gr.interaction || ' ' || rv.change_type COLLATE "C"`))
	this.Equal([]string{"2 Pet.id nullable"}, this.sdlPublishStrings(`
		SELECT cv.version || ' ' || s.name || '.' || f.path || CASE WHEN fv.nullable THEN ' nullable' ELSE '' END
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		JOIN schemas s ON s.id = f.schema_id
		JOIN contract_versions cv ON cv.contract_id = fv.contract_id
		WHERE fv.change_type = 'modified'`))

	repo := repository.NewContractRepository(this.Pool, []repository.ProtocolTable{graphqlrepository.New()})

	first, found := repo.GetContractByNameAndVersion(context.Background(), sdlPublishParticipant, "1")
	this.Require().True(found)
	this.Equal([]string{
		"query pet arguments",
		"query pet returns",
		"query pets arguments",
		"query pets returns",
	}, sdlPublishResources(first))
	this.Equal(model.Schema{"": {Type: "object"}, "id": {Type: "string"}}, first.Schemas[graphqlmodel.Namespace]["Pet"])

	second, found := repo.GetContractByNameAndVersion(context.Background(), sdlPublishParticipant, "2")
	this.Require().True(found)
	this.Equal([]string{
		"query owner arguments",
		"query owner returns",
		"query pet arguments",
		"query pet returns",
		"query pets arguments removed",
		"query pets returns removed",
	}, sdlPublishResources(second))
	this.Equal(model.Schema{"": {Type: "object"}, "id": {Type: "string", Nullable: true}}, second.Schemas[graphqlmodel.Namespace]["Pet"])
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
