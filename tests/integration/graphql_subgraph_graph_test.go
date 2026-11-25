package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const subgraphGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const subgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const subgraphEntitiesOnlySDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Pet @key(fields: "id") {
  id: ID!
  reviews: [String!]!
}
`

func (this *IntegrationSuite) createSubgraphParticipant(name string) {
	status, body := this.post("/api/participants", `{"participant":"`+name+`"}`)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) publishSubgraph(version string, fragments ...contractFragment) (status int, body string) {
	return this.post("/api/contracts", this.publishBody("petstore_api", version, fragments...))
}

func (this *IntegrationSuite) mustPublishSubgraph(version string, fragments ...contractFragment) {
	status, body := this.publishSubgraph(version, fragments...)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) validateSubgraph(environment string, fragments ...contractFragment) (status int, body string) {
	requestBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{"petstore_api", environment, fragments})
	this.Require().NoError(err)

	return this.post("/api/contracts/validate", string(requestBody))
}

func (this *IntegrationSuite) graphMembers() []string {
	rows, err := this.Pool.Query(context.Background(), `
		SELECT cv.version || ' ' || gm.graph_name
		FROM graphql_graph_members gm
		JOIN contract_versions cv ON cv.contract_id = gm.contract_id
		ORDER BY cv.version`)
	this.Require().NoError(err)
	defer rows.Close()

	var members []string
	for rows.Next() {
		var member string
		this.Require().NoError(rows.Scan(&member))
		members = append(members, member)
	}
	this.Require().NoError(rows.Err())

	return members
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_Refusals_AnswerTheViolationAndWriteNothing() {
	cases := []struct {
		name      string
		fragments []contractFragment
		violation string
	}{
		{
			name:      "a graph without the SDL",
			fragments: []contractFragment{{"graph.yaml", subgraphGraphYAML}},
			violation: `{"code":"graph.without_schema","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"petstore_graph"}}`,
		},
		{
			name:      "a federated SDL without a graph",
			fragments: []contractFragment{{"schema.graphqls", subgraphSDL}},
			violation: `{"code":"graph.required_by_federation","path":"","source":"schema.graphqls","details":{}}`,
		},
		{
			name: "a graph outside the participant name format",
			fragments: []contractFragment{
				{"graph.yaml", "provides:\n  graphql:\n    graph: Petstore-Graph\n"},
				{"schema.graphqls", subgraphSDL},
			},
			violation: `{"code":"graph.name_syntax","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"Petstore-Graph","error":"must be snake_case"}}`,
		},
		{
			name: "two YAML files with different graphs",
			fragments: []contractFragment{
				{"api.yaml", subgraphGraphYAML},
				{"graph.yaml", "provides:\n  graphql:\n    graph: orders_graph\n"},
				{"schema.graphqls", subgraphSDL},
			},
			violation: `{"code":"graph.conflict","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"orders_graph","declaredIn":"api.yaml","declared":"petstore_graph"}}`,
		},
		{
			name: "a graph named after a participant",
			fragments: []contractFragment{
				{"graph.yaml", "provides:\n  graphql:\n    graph: petstore_web\n"},
				{"schema.graphqls", subgraphSDL},
			},
			violation: `{"code":"graph.name_taken","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"petstore_web"}}`,
		},
	}

	this.createSubgraphParticipant("petstore_api")
	this.createSubgraphParticipant("petstore_web")

	for _, refused := range cases {
		status, body := this.publishSubgraph("1", refused.fragments...)

		this.Equal(http.StatusBadRequest, status, refused.name)
		this.JSONEq(`{"message":"contract validation failed","violations":[`+refused.violation+`]}`, body, refused.name)
	}

	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("graphql_graph_members"))
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_Validate_AnswersTheSameNameTaken() {
	this.createSubgraphParticipant("petstore_api")
	this.createSubgraphParticipant("petstore_web")
	status, body := this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)

	status, body = this.validateSubgraph("production",
		contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: petstore_web\n"},
		contractFragment{"schema.graphqls", subgraphSDL},
	)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"graph.name_taken","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"petstore_web"}}`+
		`]}`, body)
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_ValidateWithoutParticipantAndEnvironment_AnswersTheSameNameTaken() {
	this.createSubgraphParticipant("petstore_web")
	requestBody, err := json.Marshal(struct {
		Contracts []contractFragment `json:"contracts"`
	}{[]contractFragment{
		{"graph.yaml", "provides:\n  graphql:\n    graph: petstore_web\n"},
		{"schema.graphqls", subgraphSDL},
	}})
	this.Require().NoError(err)

	status, body := this.post("/api/contracts/validate", string(requestBody))

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"graph.name_taken","path":"provides;graphql;graph","source":"graph.yaml","details":{"graph":"petstore_web"}}`+
		`]}`, body)
	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("graphql_graph_members"))
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_Publish_WritesTheContractGraph() {
	this.createSubgraphParticipant("petstore_api")

	this.mustPublishSubgraph("1",
		contractFragment{"graph.yaml", subgraphGraphYAML},
		contractFragment{"schema.graphqls", subgraphSDL},
	)

	var contractID int64
	var graph string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id, graph_name FROM graphql_graph_members`).Scan(&contractID, &graph))

	var publishedContractID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id FROM contract_versions WHERE version = '1'`).Scan(&publishedContractID))

	this.Equal(publishedContractID, contractID)
	this.Equal("petstore_graph", graph)
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_TheSameGraphInTwoYAMLFiles_IsOneMember() {
	this.createSubgraphParticipant("petstore_api")

	this.mustPublishSubgraph("1",
		contractFragment{"api.yaml", subgraphGraphYAML},
		contractFragment{"graph.yaml", subgraphGraphYAML},
		contractFragment{"schema.graphqls", subgraphSDL},
	)

	this.Equal([]string{"1 petstore_graph"}, this.graphMembers())
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_TheSameGraphAndSDLInOtherFiles_KeepsTheChecksum() {
	this.createSubgraphParticipant("petstore_api")

	this.mustPublishSubgraph("1",
		contractFragment{"graph.yaml", subgraphGraphYAML},
		contractFragment{"schema.graphqls", subgraphSDL},
	)
	this.mustPublishSubgraph("2",
		contractFragment{"contracts/subgraph.yaml", subgraphGraphYAML},
		contractFragment{"graph/pets.graphqls", subgraphSDL},
	)

	this.Equal(1, this.countRows("contracts"))
	this.Equal([]string{"1 petstore_graph", "2 petstore_graph"}, this.graphMembers())
	this.Equal(1, this.countRows("graphql_graph_members"))
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_AVersionThatChangesOnlyTheGraph_IsANewContract() {
	this.createSubgraphParticipant("petstore_api")

	this.mustPublishSubgraph("1",
		contractFragment{"graph.yaml", subgraphGraphYAML},
		contractFragment{"schema.graphqls", subgraphSDL},
	)
	this.mustPublishSubgraph("2",
		contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: orders_graph\n"},
		contractFragment{"schema.graphqls", subgraphSDL},
	)

	this.Equal(2, this.countRows("contracts"))
	this.Equal([]string{"1 petstore_graph", "2 orders_graph"}, this.graphMembers())
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_RepublishingTheSame_ReusesTheMember() {
	this.createSubgraphParticipant("petstore_api")
	files := []contractFragment{
		{"graph.yaml", subgraphGraphYAML},
		{"schema.graphqls", subgraphSDL},
	}

	this.mustPublishSubgraph("1", files...)
	this.mustPublishSubgraph("1", files...)
	this.mustPublishSubgraph("2", files...)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("graphql_graph_members"))
}

func (this *IntegrationSuite) TestGraphQLSubgraphGraph_AnEntitiesOnlySubgraph_IsPublishedCheckedAndValidated() {
	this.createSubgraphParticipant("petstore_api")
	status, body := this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)
	files := []contractFragment{
		{"graph.yaml", subgraphGraphYAML},
		{"schema.graphqls", subgraphEntitiesOnlySDL},
	}

	this.mustPublishSubgraph("1", files...)
	status, body = this.post("/api/deployments", `{"participant":"petstore_api","version":"1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)

	canIDeployStatus, canIDeploy := this.post("/api/can-i-deploy", `{"participant":"petstore_api","version":"1","environment":"production"}`)
	validateStatus, validate := this.validateSubgraph("production", files...)

	this.Equal(0, this.countRows("resources"))
	this.Equal(http.StatusOK, canIDeployStatus, canIDeploy)
	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "petstore_api",
		"version": "1",
		"environment": "production",
		"deployable": true,
		"results": {}
	}`, canIDeploy)
	this.Equal(http.StatusOK, validateStatus, validate)
	this.JSONEq(`{
		"message": "contract validated successfully",
		"participant": "petstore_api",
		"environment": "production",
		"deployable": true,
		"results": {}
	}`, validate)
}
