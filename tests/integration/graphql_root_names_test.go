package integration_test

import (
	"net/http"
)

func (this *IntegrationSuite) TestGraphQLRootNames_ARootNamedOtherwiseNextToATypeOfTheDefaultName_IsPublished() {
	cases := []struct {
		version string
		sdl     string
	}{
		{"1", "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Query\n}\n\ntype Query {\n  id: ID!\n}\n"},
		{"2", "schema {\n  query: Query\n  mutation: RootMutation\n}\n\ntype Query {\n  pet: Int\n}\n\ntype RootMutation {\n  adoptPet: Mutation\n}\n\ntype Mutation {\n  id: ID!\n}\n"},
	}

	status, body := this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status, body)

	for _, published := range cases {
		status, body := this.post("/api/contracts", this.publishBody("petstore_api", published.version, contractFragment{"schema.graphqls", published.sdl}))
		this.Equal(http.StatusOK, status, body)
	}

	this.Equal(2, this.countRows("contracts"))
}
