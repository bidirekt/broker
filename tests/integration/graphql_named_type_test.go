package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
)

const namedTypeAPISDL = `scalar DateTime
scalar Instant

type Query {
  pet(id: ID!): Pet
  pets(ids: [ID!]!): [Pet!]!
  near(id: ID): Pet
}

type Mutation {
  addPet(input: AddPetInput!): Pet
  tagPets(status: PetStatus!): [Pet!]!
}

input AddPetInput {
  name: String!
  ownerId: ID!
  age: Int
  bornAt: DateTime
}

enum PetStatus {
  AVAILABLE
  SOLD
}

type Pet {
  id: ID!
  reviews(by: ID): [String!]!
}
`

const namedTypeWebContract = `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        pets: {arguments: PetsArgs, returns: TaggedPets}
        near: {arguments: NearArgs, returns: MaybeAddedPet}
      mutation:
        addPet: {arguments: AddPetArgs, returns: MaybeAddedPet}
        tagPets: {arguments: TagPetsArgs, returns: TaggedPets}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  PetsArgs: {type: object, properties: {ids: {type: array, items: {type: string}}}}
  NearArgs: {type: object, properties: {id: {type: string, nullable: true}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}}}
  ReviewsArgs: {type: object, properties: {by: {type: string}}}
  AddPetArgs: {type: object, properties: {input: {ref: NewPet}}}
  NewPet: {type: object, properties: {name: {type: string}, ownerId: {type: string}, age: {type: integer}, bornAt: {type: scalar, name: DateTime}}}
  MaybeAddedPet: {ref: AddedPet, nullable: true}
  AddedPet: {type: object, properties: {id: {type: string}}}
  TagPetsArgs: {type: object, properties: {status: {ref: Status}}}
  Status: {type: enum, members: [AVAILABLE]}
  TaggedPets: {type: array, items: {ref: AddedPet}}
`

const namedTypeGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const namedTypeLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])
`

const namedTypeSubgraphSDL = namedTypeLink + `
type Query {
  pet(id: ID!): Pet
  pets(ids: [ID!]!): [Pet!]!
  near(id: ID): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const namedTypeGraphWebContract = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        pets: {arguments: PetsArgs, returns: Pets}
        near: {arguments: NearArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  PetsArgs: {type: object, properties: {ids: {type: array, items: {type: string}}}}
  NearArgs: {type: object, properties: {id: {type: string, nullable: true}}}
  MaybePet: {ref: Pet, nullable: true}
  Pets: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {name: {type: string}}}
`

type namedTypeResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

func (this *IntegrationSuite) mustPostForNamedType(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishAndDeployForNamedType(participant, version string, fragments ...contractFragment) {
	this.mustPostForNamedType("/api/contracts", this.publishBody(participant, version, fragments...))
	this.mustPostForNamedType("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) deployNamedTypeProviderAndWeb() {
	this.mustPostForNamedType("/api/environments", `{"environment":"production"}`)
	this.mustPostForNamedType("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForNamedType("/api/participants", `{"participant":"petstore_web"}`)
	this.publishAndDeployForNamedType("petstore_api", "1.0.0", contractFragment{"schema.graphqls", namedTypeAPISDL})
	this.publishAndDeployForNamedType("petstore_web", "1.0.0", contractFragment{"web.yaml", namedTypeWebContract})
}

func (this *IntegrationSuite) checkForNamedType(participant, version string, fragments ...contractFragment) (canIDeploy, validate namedTypeResponse) {
	this.mustPostForNamedType("/api/contracts", this.publishBody(participant, version, fragments...))

	status, body := this.post("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)
	this.Require().NoError(json.Unmarshal([]byte(body), &canIDeploy))

	requestBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{participant, "production", fragments})
	this.Require().NoError(err)

	status, body = this.post("/api/contracts/validate", string(requestBody))
	this.Require().Equal(http.StatusOK, status, body)
	this.Require().NoError(json.Unmarshal([]byte(body), &validate))

	return canIDeploy, validate
}

func (this *IntegrationSuite) assertNamedTypeCheck(participant, version string, fragments []contractFragment, deployable bool, results string) {
	canIDeploy, validate := this.checkForNamedType(participant, version, fragments...)

	this.Equal(deployable, canIDeploy.Deployable)
	this.JSONEq(results, string(canIDeploy.Results))
	this.Equal(deployable, validate.Deployable)
	this.JSONEq(results, string(validate.Results))
}

func (this *IntegrationSuite) assertNamedTypeProviderCheck(oldText, newText string, deployable bool, results string) {
	this.deployNamedTypeProviderAndWeb()

	candidate := strings.Replace(namedTypeAPISDL, oldText, newText, 1)
	this.Require().NotEqual(namedTypeAPISDL, candidate)

	this.assertNamedTypeCheck("petstore_api", "1.1.0", []contractFragment{{"schema.graphqls", candidate}}, deployable, results)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingTheNamedTypeOfAnArgumentAConsumerSends_BreaksThatConsumer() {
	this.assertNamedTypeProviderCheck("pet(id: ID!)", "pet(id: String!)", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "id", "consumerPropertyType": "ID", "providerPropertyType": "String", "consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingTheNamedTypeOfAnInputFieldAConsumerSends_BreaksThatConsumer() {
	this.assertNamedTypeProviderCheck("ownerId: ID!", "ownerId: String!", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"addPet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "input.ownerId", "consumerPropertyType": "ID", "providerPropertyType": "String", "consumerField": "NewPet.ownerId", "providerField": "AddPetInput.ownerId"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingAnArgumentAConsumerSendsFromIDToInt_BreaksThatConsumerOnlyByTheShape() {
	this.assertNamedTypeProviderCheck("pet(id: ID!)", "pet(id: Int!)", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "id", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingAnInputFieldAConsumerSendsFromIntToFloat_BreaksThatConsumerByTheName() {
	this.assertNamedTypeProviderCheck("age: Int", "age: Float", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"addPet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "input.age", "consumerPropertyType": "Int", "providerPropertyType": "Float", "consumerField": "NewPet.age", "providerField": "AddPetInput.age"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderRenamingACustomScalarAConsumerNames_BreaksThatConsumerOnce() {
	this.assertNamedTypeProviderCheck("bornAt: DateTime", "bornAt: Instant", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"addPet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "input.bornAt", "consumerPropertyType": "DateTime!", "providerPropertyType": "Instant", "consumerField": "NewPet.bornAt", "providerField": "AddPetInput.bornAt"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderRenamingAnInputAConsumerSends_BreaksThatConsumer() {
	this.deployNamedTypeProviderAndWeb()

	candidate := strings.ReplaceAll(namedTypeAPISDL, "AddPetInput", "NewPetInput")

	this.assertNamedTypeCheck("petstore_api", "1.1.0", []contractFragment{{"schema.graphqls", candidate}}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"addPet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "input", "consumerPropertyType": "AddPetInput", "providerPropertyType": "NewPetInput", "consumerField": "Mutation.addPet(input:)", "providerField": "Mutation.addPet(input:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderRenamingAnEnumAConsumerSends_BreaksThatConsumer() {
	this.deployNamedTypeProviderAndWeb()

	candidate := strings.ReplaceAll(namedTypeAPISDL, "PetStatus", "AdoptionStatus")

	this.assertNamedTypeCheck("petstore_api", "1.1.0", []contractFragment{{"schema.graphqls", candidate}}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"tagPets": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "status", "consumerPropertyType": "PetStatus", "providerPropertyType": "AdoptionStatus", "consumerField": "Mutation.tagPets(status:)", "providerField": "Mutation.tagPets(status:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingTheNamedTypeOfANestedFieldArgumentAConsumerSends_BreaksThatConsumer() {
	this.assertNamedTypeProviderCheck("reviews(by: ID)", "reviews(by: String)", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$.reviews", "argument": "by", "consumerPropertyType": "ID", "providerPropertyType": "String", "consumerField": "ReviewsArgs.by", "providerField": "Pet.reviews(by:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderKeepingTheNamedTypes_DoesNotBreakTheConsumer() {
	this.assertNamedTypeProviderCheck("  name: String!\n  ownerId: ID!\n", "  name: String!\n  ownerId: ID!\n  nickname: String\n", true,
		`{"petstore_web": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AConsumerAgainstTheDeployedProviderThatChangedANamedType_DoesNotBreak() {
	this.deployNamedTypeProviderAndWeb()
	this.publishAndDeployForNamedType("petstore_api", "1.1.0", contractFragment{"schema.graphqls", strings.Replace(namedTypeAPISDL, "pet(id: ID!)", "pet(id: String!)", 1)})

	this.assertNamedTypeCheck("petstore_web", "1.1.0", []contractFragment{{"web.yaml", namedTypeWebContract}}, true,
		`{"petstore_api": {"deployable": true, "participantVersion": "1.1.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderChangingAListArgumentWhoseItemsNoLongerFit_BreaksThatConsumerOnlyByTheShape() {
	this.assertNamedTypeProviderCheck("pets(ids: [ID!]!)", "pets(ids: [Int!]!)", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pets": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "ids[]", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Query.pets(ids:)[]", "providerField": "Query.pets(ids:)[]"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderRenamingTheTypeOfANullableArgumentToANonNullOne_BreaksThatConsumerOnlyByTheNullability() {
	this.assertNamedTypeProviderCheck("near(id: ID)", "near(id: String!)", false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"near": {"arguments": [
			{"reason": "property_nullable_in_consumer_not_nullable_in_provider", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "id", "propertyType": "String", "consumerField": "Query.near(id:)", "providerField": "Query.near(id:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AProviderRenamingAnEnumWithoutAValueTheConsumerSends_BreaksThatConsumerOnlyByTheValue() {
	this.deployNamedTypeProviderAndWeb()

	candidate := strings.Replace(strings.ReplaceAll(namedTypeAPISDL, "PetStatus", "AdoptionStatus"), "  AVAILABLE\n", "", 1)
	this.Require().NotContains(candidate, "AVAILABLE")

	this.assertNamedTypeCheck("petstore_api", "1.1.0", []contractFragment{{"schema.graphqls", candidate}}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"mutation": {"tagPets": {"arguments": [
			{"reason": "property_member_missing_in_provider", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "status", "member": "AVAILABLE", "consumerField": "Mutation.tagPets(status:)", "providerField": "Mutation.tagPets(status:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) assertNamedTypeSubgraphCheck(oldText, newText, results string) {
	this.mustPostForNamedType("/api/environments", `{"environment":"production"}`)
	this.mustPostForNamedType("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForNamedType("/api/participants", `{"participant":"petstore_web"}`)
	this.publishAndDeployForNamedType("petstore_api", "1.4.0", contractFragment{"graph.yaml", namedTypeGraphYAML}, contractFragment{"api.graphqls", namedTypeSubgraphSDL})
	this.publishAndDeployForNamedType("petstore_web", "3.0.0", contractFragment{"web.yaml", namedTypeGraphWebContract})

	candidate := strings.Replace(namedTypeSubgraphSDL, oldText, newText, 1)
	this.Require().NotEqual(namedTypeSubgraphSDL, candidate)

	this.assertNamedTypeCheck("petstore_api", "1.5.0", []contractFragment{{"graph.yaml", namedTypeGraphYAML}, {"api.graphqls", candidate}}, false, results)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphChangingTheNamedTypeOfAnArgumentTheGraphConsumerSends_BreaksThatConsumer() {
	this.assertNamedTypeSubgraphCheck("pet(id: ID!)", "pet(id: String!)", `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$", "argument": "id", "consumerPropertyType": "ID", "providerPropertyType": "String", "consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)", "graph": "petstore_graph"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphChangingAnArgumentTheGraphConsumerSendsFromIDToInt_BreaksThatConsumerOnlyByTheShape() {
	this.assertNamedTypeSubgraphCheck("pet(id: ID!)", "pet(id: Int!)", `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$", "argument": "id", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)", "graph": "petstore_graph"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphChangingAListArgumentWhoseItemsNoLongerFit_BreaksTheGraphConsumerOnlyByTheShape() {
	this.assertNamedTypeSubgraphCheck("pets(ids: [ID!]!)", "pets(ids: [Int!]!)", `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pets": {"arguments": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$", "argument": "ids[]", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Query.pets(ids:)[]", "providerField": "Query.pets(ids:)[]", "graph": "petstore_graph"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphRenamingTheTypeOfANullableArgumentToANonNullOne_BreaksTheGraphConsumerOnlyByTheNullability() {
	this.assertNamedTypeSubgraphCheck("near(id: ID)", "near(id: String!)", `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"near": {"arguments": [
			{"reason": "property_nullable_in_consumer_not_nullable_in_provider", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$", "argument": "id", "propertyType": "String", "consumerField": "Query.near(id:)", "providerField": "Query.near(id:)", "graph": "petstore_graph"}}
		]}}}
	}}`)
}
