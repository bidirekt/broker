package integration_test

import (
	"encoding/json"
	"net/http"
)

func (this *IntegrationSuite) TestGraphQLRootNameTaken_PublishAndValidate_AnswerTheViolationAndWriteNothing() {
	cases := []struct {
		name      string
		sdl       string
		violation string
	}{
		{
			name:      "query",
			sdl:       "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Query\n}\n\ntype Query {\n  id: ID!\n}\n",
			violation: `{"code": "schema.root_name_taken", "path": "", "source": "schema.graphqls", "details": {"operation": "query", "root": "RootQuery", "type": "Query"}}`,
		},
		{
			name:      "mutation",
			sdl:       "schema {\n  query: Query\n  mutation: RootMutation\n}\n\ntype Query {\n  pet: Int\n}\n\ntype RootMutation {\n  adoptPet: Mutation\n}\n\ntype Mutation {\n  id: ID!\n}\n",
			violation: `{"code": "schema.root_name_taken", "path": "", "source": "schema.graphqls", "details": {"operation": "mutation", "root": "RootMutation", "type": "Mutation"}}`,
		},
	}

	status, body := this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status, body)

	for _, refused := range cases {
		fragment := contractFragment{"schema.graphqls", refused.sdl}
		expected := `{"message":"contract validation failed","violations":[` + refused.violation + `]}`

		status, body := this.post("/api/contracts", this.publishBody("petstore_api", "1", fragment))
		this.Equal(http.StatusBadRequest, status, refused.name)
		this.JSONEq(expected, body, refused.name)

		validateBody, err := json.Marshal(struct {
			Contracts []contractFragment `json:"contracts"`
		}{[]contractFragment{fragment}})
		this.Require().NoError(err)

		status, body = this.post("/api/contracts/validate", string(validateBody))
		this.Equal(http.StatusBadRequest, status, refused.name)
		this.JSONEq(expected, body, refused.name)
	}

	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("contract_versions"))
	this.Equal(0, this.countRows("graphql_root_types"))
}
