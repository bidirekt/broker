package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const readingProviderSDL = `interface Node {
  id: ID!
  name: String
}

type Person implements Node {
  id: ID!
  name: String
  age: Int
}

type Org implements Node {
  id: ID!
  name: String
}

type Pet {
  id: ID!
}

union SearchResult = Pet | Person

type Query {
  node(id: ID!): Node
  pet(id: ID!, weight: Float, name: String): Pet
  search: SearchResult
}
`

const readingProviderWithoutNodeIDSDL = `interface Node {
  name: String
}

type Person implements Node {
  id: ID!
  name: String
  age: Int
}

type Org implements Node {
  id: ID!
  name: String
}

type Query {
  node(id: ID!): Node
}
`

const readingNodeAsObjectContract = `consumes:
  petstore_api:
    graphql:
      query:
        node: {arguments: NodeArgs, returns: MaybeNode}
schemas:
  NodeArgs: {type: object, properties: {id: {type: string}}}
  MaybeNode: {ref: Node, nullable: true}
  Node: {type: object, properties: {id: {type: string}}}
`

const readingNodeAsUnionContract = `consumes:
  petstore_api:
    graphql:
      query:
        node: {arguments: NodeArgs, returns: MaybeNode}
schemas:
  NodeArgs: {type: object, properties: {id: {type: string}}}
  MaybeNode: {ref: Node, nullable: true}
  Node: {type: union, members: {Person: {ref: Person}}}
  Person:
    type: object
    properties:
      id: {type: string}
      age: {type: integer, nullable: true}
`

type readingCheckResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

func (this *IntegrationSuite) mustPostForReading(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) deployReadingProvider(sdl string) {
	this.mustPostForReading("/api/environments", `{"environment":"production"}`)
	this.mustPostForReading("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForReading("/api/contracts", this.publishBody("petstore_api", "1.0.0", contractFragment{"schema.graphqls", sdl}))
	this.mustPostForReading("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) assertReadingResults(consumer, contract string, deployable bool, results string) {
	contracts := []contractFragment{{"web.yaml", contract}}

	this.mustPostForReading("/api/participants", `{"participant":"`+consumer+`"}`)
	this.mustPostForReading("/api/contracts", this.publishBody(consumer, "1.0.0", contracts...))

	var canIDeploy, validate readingCheckResponse

	status, response := this.post("/api/can-i-deploy", `{"participant":"`+consumer+`","version":"1.0.0","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &canIDeploy))

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{Participant: consumer, Environment: "production", Contracts: contracts})
	this.Require().NoError(err)

	status, response = this.post("/api/contracts/validate", string(validateBody))
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &validate))

	this.Equal(deployable, canIDeploy.Deployable, consumer)
	this.Equal(deployable, validate.Deployable, consumer)
	this.JSONEq(results, string(canIDeploy.Results), consumer)
	this.JSONEq(results, string(validate.Results), consumer)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerReadingAnInterfaceAsAnObjectOrAsAUnion_IsDeployable() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", readingNodeAsObjectContract, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
	this.assertReadingResults("petstore_admin", readingNodeAsUnionContract, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AFieldRemovedOnlyFromTheInterface_BreaksOnlyTheObjectRead() {
	this.deployReadingProvider(readingProviderWithoutNodeIDSDL)

	this.assertReadingResults("petstore_web", readingNodeAsObjectContract, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"node": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$.id", "propertyType": "String!", "consumerField": "Node.id"}}
		]}}}
	}}`)
	this.assertReadingResults("petstore_admin", readingNodeAsUnionContract, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerReadingAnOptionalFieldTheProviderLacks_Breaks() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      id: {type: string}
      bornAt: {type: string, optional: true, nullable: true}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$.bornAt", "propertyType": "String", "consumerField": "Pet.bornAt"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingWhatTheSpecCoerces_IsDeployable() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs:
    type: object
    properties:
      id: {type: integer}
      weight: {type: integer}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertReadingResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs:
    type: object
    properties:
      id: {type: string}
      weight: {type: number}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingAnIntegerToAString_BreaksUnderArguments() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs:
    type: object
    properties:
      id: {type: string}
      name: {type: integer}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "name", "consumerPropertyType": "Int!", "providerPropertyType": "String", "consumerField": "PetArgs.name", "providerField": "Query.pet(name:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerReadingTypenames_IsDeployable() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        search: {returns: MaybeSearchResult}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      __typename: {type: string}
      id: {type: string}
  MaybeSearchResult: {ref: SearchResult, nullable: true}
  SearchResult: {type: object, properties: {__typename: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertReadingResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        search: {returns: MaybeSearchResult}
schemas:
  MaybeSearchResult: {ref: SearchResult, nullable: true}
  SearchResult: {type: union, members: {Pet: {ref: Pet}}}
  Pet:
    type: object
    properties:
      __typename: {type: string}
      id: {type: string}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerOfTheIntrospectionFields_IsDeployableAndNeverStoresThem() {
	this.deployReadingProvider(readingProviderSDL)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        __schema: {returns: Schema}
        __type: {arguments: TypeArgs, returns: MaybeType}
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
  Schema:
    type: object
    properties:
      queryType: {ref: Type}
  TypeArgs: {type: object, properties: {name: {type: string}}}
  MaybeType: {ref: Type, nullable: true}
  Type:
    type: object
    properties:
      name: {type: string, nullable: true}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	rows, err := this.Pool.Query(context.Background(), `
		SELECT DISTINCT gr.field FROM resources r JOIN graphql_resources gr ON gr.resource_id = r.id
		WHERE r.consumed_provider IS NOT NULL ORDER BY gr.field`)
	this.Require().NoError(err)
	defer rows.Close()

	var storedFields []string
	for rows.Next() {
		var field string
		this.Require().NoError(rows.Scan(&field))
		storedFields = append(storedFields, field)
	}
	this.Require().NoError(rows.Err())
	this.Equal([]string{"pet"}, storedFields)
}

const readingMixedNodeSDL = `interface Node {
  id: ID!
}

type User implements Node {
  id: ID!
  name: String
}

type Pet implements Node {
  id: ID!
  species: String
}

type Query {
  node(id: ID!): Node
}
`

const readingMixedUserWithoutNameSDL = `interface Node {
  id: ID!
}

type User implements Node {
  id: ID!
}

type Pet implements Node {
  id: ID!
  species: String
}

type Query {
  node(id: ID!): Node
}
`

const readingMixedNodeContract = `consumes:
  petstore_api:
    graphql:
      query:
        node: {arguments: NodeArgs, returns: MaybeNode}
schemas:
  NodeArgs: {type: object, properties: {id: {type: string}}}
  MaybeNode: {ref: Node, nullable: true}
  Node:
    type: union
    properties:
      __typename: {type: string}
      id: {type: string}
    members:
      User: {ref: UserName}
      Pet: {ref: PetSpecies}
  UserName: {type: object, properties: {name: {type: string, nullable: true}}}
  PetSpecies: {type: object, properties: {species: {type: string, nullable: true}}}
`

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerReadingAnInterfaceThroughItsFieldsAndItsImplementations_IsDeployable() {
	this.deployReadingProvider(readingMixedNodeSDL)

	this.assertReadingResults("petstore_web", readingMixedNodeContract, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AFieldRemovedFromAnImplementation_BreaksTheInterfaceReadAtItsMember() {
	this.deployReadingProvider(readingMixedUserWithoutNameSDL)

	this.assertReadingResults("petstore_web", readingMixedNodeContract, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"node": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$(User).name", "propertyType": "String", "consumerField": "UserName.name"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingASingleValueToAListArgument_IsDeployable() {
	this.deployReadingProvider(`type Pet {
  id: ID!
}

type Query {
  pets(tags: [String!], matrix: [[Int!]!]): [Pet!]!
}
`)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pets: {arguments: PetsArgs, returns: PetList}
schemas:
  PetsArgs:
    type: object
    properties:
      tags: {type: string}
      matrix: {type: integer}
  PetList: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {id: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLBreak_WritesTheTypesAsGraphQLDoes() {
	this.deployReadingProvider(`scalar Date

type Pet {
  id: ID!
  reviews: String!
  bornAt: Date!
}

type Query {
  pet(id: ID!): Pet
}
`)

	this.assertReadingResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      reviews: {type: array, items: {type: string}}
      bornAt: {type: scalar, name: DateTime}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$.bornAt", "consumerPropertyType": "DateTime!", "providerPropertyType": "Date!", "consumerField": "Pet.bornAt", "providerField": "Pet.bornAt"}},
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$.reviews", "consumerPropertyType": "[String!]!", "providerPropertyType": "String!", "consumerField": "Pet.reviews", "providerField": "Pet.reviews"}}
		]}}}
	}}`)
}
