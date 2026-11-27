package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bidirekt/broker/internal"
)

const protocolModulesParticipantBody = `{"participant":"pets_service"}`

const protocolModulesSDL = "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"

const protocolModulesRESTYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const protocolModulesGraphQLYAML = `provides:
  graphql:
    query:
      pet: {arguments: PetArgs, returns: Pet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  Pet: {type: object, properties: {id: {type: string}}}
`

const protocolModulesScalarYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: scalar
`

const protocolModulesConsumerYAML = `consumes:
  pets_service:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, name: {type: string}}}
`

const protocolModulesConsumerResults = `{"pets_service": {
	"deployable": false,
	"participantVersion": "1",
	"operations": {"query": {"pet": {"returns": [
		{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerField": "Pet.name", "consumerName": "pets_web", "property": "$.name", "propertyType": "String!", "providerName": "pets_service"}}
	]}}}
}}`

type protocolModulesGraphQLFlow struct {
	publishStatus    int
	publishBody      string
	canIDeployStatus int
	canIDeploy       string
	validateStatus   int
	validate         string
	checkPage        string
}

func (this *IntegrationSuite) serveOnlyModules(modules ...internal.Module) {
	served, err := internal.RunWith(modules)
	this.Require().NoError(err)

	suiteComponents := this.Components
	this.T().Cleanup(func() {
		served.Pool.Close()
		this.Components = suiteComponents
	})

	this.Components = served
}

func (this *IntegrationSuite) TestRunWith_OnlyREST_AGraphQLFileIsAnUnsupportedContractFile() {
	this.serveOnlyModules(internal.RESTModule())

	status, _ := this.post("/api/participants", protocolModulesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	for _, source := range []string{"schema.graphql", "schema.graphqls", "schema.gql"} {
		status, body := this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{source, protocolModulesSDL}))
		this.Equal(http.StatusBadRequest, status, source)
		this.JSONEq(`{"message":"unsupported contract file: `+source+` (expected .yaml or .yml)"}`, body, source)
	}

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestRunWith_OnlyREST_YAMLIsPublishedAsWithBothProtocols() {
	this.serveOnlyModules(internal.RESTModule())

	status, _ := this.post("/api/participants", protocolModulesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{"api.yaml", protocolModulesRESTYAML}))
	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestRunWith_OnlyGraphQL_TheRESTBlockIsAnUnknownKey() {
	this.serveOnlyModules(internal.GraphQLModule())

	status, _ := this.post("/api/participants", protocolModulesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{"api.yaml", protocolModulesRESTYAML}))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;rest","source":"api.yaml","details":{"key":"rest"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestRunWith_OnlyREST_TheGraphQLBlockIsAnUnknownKey() {
	this.serveOnlyModules(internal.RESTModule())

	status, _ := this.post("/api/participants", protocolModulesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{"api.yaml", protocolModulesGraphQLYAML}))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;graphql","source":"api.yaml","details":{"key":"graphql"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestRunWith_OnlyREST_ScalarIsAnInvalidTypeAmongTheCoreTypes() {
	this.serveOnlyModules(internal.RESTModule())

	status, _ := this.post("/api/participants", protocolModulesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{"api.yaml", protocolModulesScalarYAML}))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.invalid_type","path":"schemas;Pet;properties;id;type","source":"api.yaml","details":{"value":"scalar","allowed":"object, array, map, string, integer, float, number, boolean, enum, union"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestRunWith_OnlyREST_TheGraphPageIsNotFound() {
	this.serveOnlyModules(internal.RESTModule())

	for _, route := range this.Components.Server.GetRoutes() {
		this.NotEqual("/graphs/:name", route.Path)
	}

	status, _, _ := this.getPage("/graphs/petstore_graph", this.signInAsAdmin())
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) runGraphQLFlowForProtocolModules() protocolModulesGraphQLFlow {
	var flow protocolModulesGraphQLFlow

	status, response := this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)

	for _, participant := range []string{"pets_service", "pets_web"} {
		status, response = this.post("/api/participants", `{"participant":"`+participant+`"}`)
		this.Require().Equal(http.StatusOK, status, response)
	}

	flow.publishStatus, flow.publishBody = this.post("/api/contracts", this.publishBody("pets_service", "1", contractFragment{"schema.graphqls", protocolModulesSDL}))

	status, response = this.post("/api/deployments", `{"participant":"pets_service","version":"1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)

	consumer := contractFragment{"web.yaml", protocolModulesConsumerYAML}
	status, response = this.post("/api/contracts", this.publishBody("pets_web", "1", consumer))
	this.Require().Equal(http.StatusOK, status, response)

	flow.canIDeployStatus, flow.canIDeploy = this.post("/api/can-i-deploy", `{"participant":"pets_web","version":"1","environment":"production"}`)

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{Participant: "pets_web", Environment: "production", Contracts: []contractFragment{consumer}})
	this.Require().NoError(err)
	flow.validateStatus, flow.validate = this.post("/api/contracts/validate", string(validateBody))

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))
	status, _, flow.checkPage = this.getPage("/checks/"+strconv.FormatInt(checkID, 10), this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, flow.checkPage)

	return flow
}

func (this *IntegrationSuite) assertGraphQLFlowForProtocolModules(flow protocolModulesGraphQLFlow) {
	this.Equal(http.StatusOK, flow.publishStatus, flow.publishBody)
	this.JSONEq(`{"message":"contract publish successful"}`, flow.publishBody)

	this.Equal(http.StatusOK, flow.canIDeployStatus, flow.canIDeploy)
	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "pets_web",
		"version": "1",
		"environment": "production",
		"deployable": false,
		"results": `+protocolModulesConsumerResults+`
	}`, flow.canIDeploy)

	this.Equal(http.StatusOK, flow.validateStatus, flow.validate)
	var validate struct {
		Deployable bool            `json:"deployable"`
		Results    json.RawMessage `json:"results"`
	}
	this.Require().NoError(json.Unmarshal([]byte(flow.validate), &validate))
	this.False(validate.Deployable)
	this.JSONEq(protocolModulesConsumerResults, string(validate.Results))

	this.Contains(flow.checkPage, `<link rel="stylesheet" href="/assets/protocols/operations.css">`)
	this.Contains(flow.checkPage, `<div class="op-title"><span class="op-prop">FIELD</span><span>Pet.name</span><span class="badge">1 place</span></div>`)
	this.Contains(flow.checkPage, `<li><span class="op-kind">query</span><span>pet</span><span class="at">returns</span><span>at &#34;$.name&#34;</span></li>`)
	this.NotContains(flow.checkPage, `/assets/protocols/endpoints.css`)
}

func (this *IntegrationSuite) TestRunWith_BothProtocols_AGraphQLProviderAndConsumerArePublishedCheckedAndShown() {
	this.assertGraphQLFlowForProtocolModules(this.runGraphQLFlowForProtocolModules())
}

func (this *IntegrationSuite) TestRunWith_OnlyGraphQL_AGraphQLProviderAndConsumerArePublishedCheckedAndShownAsWithBothProtocols() {
	this.serveOnlyModules(internal.GraphQLModule())

	this.assertGraphQLFlowForProtocolModules(this.runGraphQLFlowForProtocolModules())
}
