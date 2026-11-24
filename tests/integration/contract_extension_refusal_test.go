package integration_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const extensionRefusalSDL = "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"

const extensionRefusalViolations = `{"message":"contract validation failed","violations":[` +
	`{"code":"probe.refused","path":"provides;graphql;graph","source":"schema.graphqls","details":{"graph":"petstore_graph"}}` +
	`]}`

type refusingExtensionStore struct{}

func (this refusingExtensionStore) Protocol() model.Protocol {
	return "graphql"
}

func (this refusingExtensionStore) RefuseContractExtension(context.Context, repository.Querier, any) (model.Refusal, bool) {
	return model.Refusal{
		Code:    "probe.refused",
		Path:    "provides;graphql;graph",
		Source:  "schema.graphqls",
		Details: map[string]any{"graph": "petstore_graph"},
	}, true
}

func (this refusingExtensionStore) InsertContractExtension(context.Context, repository.Querier, int64, any) {
	panic("a refused extension is never inserted")
}

func (this refusingExtensionStore) LoadContractExtensions(context.Context, repository.Querier, []int64) map[int64]any {
	return map[int64]any{}
}

func (this *IntegrationSuite) serveWithRefusingExtensionStore() {
	graphqlModule := internal.GraphQLModule()
	graphqlModule.ContractExtensions = refusingExtensionStore{}

	served, err := internal.RunWith([]internal.Module{internal.RESTModule(), graphqlModule})
	this.Require().NoError(err)

	suiteComponents := this.Components
	this.T().Cleanup(func() {
		served.Pool.Close()
		this.Components = suiteComponents
	})

	this.Components = served
}

func (this *IntegrationSuite) refusalRequestBody(fields map[string]any) string {
	fields["contracts"] = []contractFragment{{"schema.graphqls", extensionRefusalSDL}}
	body, err := json.Marshal(fields)
	this.Require().NoError(err)

	return string(body)
}

func (this *IntegrationSuite) TestContractExtensionRefusal_Publish_AnswersTheViolationAndWritesNothing() {
	this.serveWithRefusingExtensionStore()

	status, _ := this.post("/api/participants", `{"participant":"pets_graph_api"}`)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.refusalRequestBody(map[string]any{"participant": "pets_graph_api", "version": "1"}))

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(extensionRefusalViolations, body)
	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("contract_versions"))
	this.Equal(0, this.countRows("resources"))
	this.Equal(0, this.countRows("schemas"))
}

func (this *IntegrationSuite) TestContractExtensionRefusal_Validate_AnswersTheSameViolation() {
	this.serveWithRefusingExtensionStore()

	status, _ := this.post("/api/participants", `{"participant":"pets_graph_api"}`)
	this.Require().Equal(http.StatusOK, status)
	status, _ = this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts/validate", this.refusalRequestBody(map[string]any{"participant": "pets_graph_api", "environment": "production"}))

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(extensionRefusalViolations, body)
}
