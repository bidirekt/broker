package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func graphRuleViolations(extension graphqlmodel.Extension, formats ...string) []violation.Violation {
	declarations := protocol.Declarations{Extensions: map[string]any{"graphql": extension}, Formats: formats}

	var violations []violation.Violation
	for _, rule := range graphqlcontracts.New().Rules() {
		violations = append(violations, rule(declarations)...)
	}

	return violations
}

func TestRules_AGraphWithoutTheSDL_IsRefusedAtTheGraph(t *testing.T) {
	violations := graphRuleViolations(graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}, "yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "graph.without_schema",
		Path:      "provides;graphql;graph",
		Source:    "graph.yaml",
		Details:   map[string]any{"graph": "petstore_graph"},
	}}, violations)
}

func TestRules_AGraphWithTheSDL_IsAccepted(t *testing.T) {
	violations := graphRuleViolations(graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml", LinkSource: "schema.graphqls"}, "yaml", "graphql")

	assert.Empty(t, violations)
}

func TestRules_AFederatedSDLWithoutGraph_IsRefusedAtTheLinkFile(t *testing.T) {
	violations := graphRuleViolations(graphqlmodel.Extension{LinkSource: "schema.graphqls"}, "graphql")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "graph.required_by_federation",
		Path:      "",
		Source:    "schema.graphqls",
		Details:   map[string]any{},
	}}, violations)
}

func TestRules_ASDLWithoutLinkAndWithoutGraph_IsAccepted(t *testing.T) {
	assert.Empty(t, graphRuleViolations(graphqlmodel.Extension{}, "graphql"))
}

func TestRules_WithoutGraphQLExtension_NoGraphRuleApplies(t *testing.T) {
	var violations []violation.Violation
	for _, rule := range graphqlcontracts.New().Rules() {
		violations = append(violations, rule(protocol.Declarations{Formats: []string{"yaml"}})...)
	}

	assert.Empty(t, violations)
}
