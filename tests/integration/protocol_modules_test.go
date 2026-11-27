package integration_test

import (
	"net/http"

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
