package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

func rootViolations(t *testing.T, files ...model.ContractFile) []violation.Violation {
	t.Helper()

	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	_, violations, readErr := interpreter.ToViolations(files)
	require.NoError(t, readErr)

	return violations
}

func TestRules_ARootNamedOtherwiseNextToATypeOfTheDefaultName_IsRefusedAtTheType(t *testing.T) {
	cases := map[string]struct {
		sdl       string
		operation string
		root      string
		typeName  string
	}{
		"query": {
			sdl:       "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Query\n}\n\ntype Query {\n  id: ID!\n}\n",
			operation: "query",
			root:      "RootQuery",
			typeName:  "Query",
		},
		"mutation": {
			sdl:       "schema {\n  query: Query\n  mutation: RootMutation\n}\n\ntype Query {\n  pet: Int\n}\n\ntype RootMutation {\n  adoptPet: Mutation\n}\n\ntype Mutation {\n  id: ID!\n}\n",
			operation: "mutation",
			root:      "RootMutation",
			typeName:  "Mutation",
		},
	}

	for name, refused := range cases {
		t.Run(name, func(t *testing.T) {
			violations := rootViolations(t, model.ContractFile{Source: "schema.graphqls", Content: refused.sdl})

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "schema.root_name_taken",
				Path:      "",
				Source:    "schema.graphqls",
				Details:   map[string]any{"operation": refused.operation, "root": refused.root, "type": refused.typeName},
			}}, violations)
		})
	}
}

func TestRules_ATypeOfTheDefaultNameInAnotherFile_IsRefusedAtItsFile(t *testing.T) {
	violations := rootViolations(t,
		model.ContractFile{Source: "schema.graphqls", Content: "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Query\n}\n"},
		model.ContractFile{Source: "types.graphqls", Content: "type Query {\n  id: ID!\n}\n"},
	)

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "schema.root_name_taken",
		Path:      "",
		Source:    "types.graphqls",
		Details:   map[string]any{"operation": "query", "root": "RootQuery", "type": "Query"},
	}}, violations)
}

func TestRules_RootsNamedOtherwiseWithoutATypeOfTheDefaultName_AreAccepted(t *testing.T) {
	violations := rootViolations(t, model.ContractFile{
		Source:  "schema.graphqls",
		Content: "schema {\n  query: RootQuery\n  mutation: RootMutation\n}\n\ntype RootQuery {\n  pet: Int\n}\n\ntype RootMutation {\n  adoptPet: Int\n}\n",
	})

	assert.Empty(t, violations)
}

func TestRules_ATypeOfTheDefaultNameThatIsTheOtherRoot_IsAccepted(t *testing.T) {
	violations := rootViolations(t, model.ContractFile{
		Source:  "schema.graphqls",
		Content: "schema {\n  query: RootQuery\n  mutation: Query\n}\n\ntype RootQuery {\n  pet: Int\n}\n\ntype Query {\n  adoptPet: Int\n}\n",
	})

	assert.Empty(t, violations)
}
