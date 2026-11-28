package graphql_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
)

const (
	apolloSets  = "testdata/apollo"
	apolloGraph = "apollo_graph"
)

// An SDL publish can't read is what Apollo calls invalid GraphQL.
const apolloInvalidGraphQL = "INVALID_GRAPHQL"

// A subgraph is published with the YAML that puts it in its graph, as publish requires.
var apolloGraphFile = model.ContractFile{Source: "graph.yaml", Content: "provides:\n  graphql:\n    graph: " + apolloGraph + "\n"}

type apolloResult struct {
	Composes  bool     `json:"composes"`
	Codes     []string `json:"codes"`
	Exception string   `json:"exception"`
}

type parityResult struct {
	composes bool
	codes    []string
	reasons  []string
}

var parityInterpreter = contractfiles.NewInterpreter(
	protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
	[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
)

func apolloSetNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(apolloSets)
	require.NoError(t, err)

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return names
}

func knownDivergences(t *testing.T) []string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(apolloSets, "known_divergences.txt"))
	require.NoError(t, err)

	return strings.Fields(string(content))
}

func readApolloResult(t *testing.T, set string) apolloResult {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(apolloSets, set, "apollo.json"))
	require.NoError(t, err)

	var result apolloResult
	require.NoError(t, json.Unmarshal(content, &result))

	return result
}

// Each subgraph is published on its own, its participant named after its file, then the graph is composed.
func composeApolloSet(t *testing.T, set string) parityResult {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(apolloSets, set, "*.graphql"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	result := parityResult{}
	members := make([]*model.PersistedContract, 0, len(paths))

	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)

		source := filepath.Base(path)
		participant := &model.Participant{ID: 1, Name: strings.TrimSuffix(source, filepath.Ext(source))}

		uploaded, violations, readErr, err := parityInterpreter.ToUploadedContract([]model.ContractFile{apolloGraphFile, {Source: source, Content: string(content)}}, participant, "1.0.0")
		require.NoError(t, err)

		if readErr != nil {
			result.codes = append(result.codes, apolloInvalidGraphQL)
			result.reasons = append(result.reasons, readErr.Error())

			continue
		}

		for _, refused := range violations {
			result.codes = append(result.codes, refused.ErrorCode)
			result.reasons = append(result.reasons, source+": "+refused.ErrorCode)
		}

		if len(violations) == 0 {
			members = append(members, &model.PersistedContract{
				ParticipantName: uploaded.ParticipantName,
				Version:         uploaded.Version,
				Schemas:         uploaded.Schemas,
				Extensions:      uploaded.Extensions,
			})
		}
	}

	if len(result.codes) > 0 {
		return result
	}

	_, errors := graphqlcontracts.ToComposedGraph(apolloGraph, members)
	for _, composed := range errors {
		result.codes = append(result.codes, composed.Code)
		result.reasons = append(result.reasons, composed.Message)
	}

	result.composes = len(errors) == 0

	return result
}

func (this parityResult) agreesWith(apollo apolloResult) bool {
	if this.composes != apollo.Composes {
		return false
	}

	return this.composes || slices.ContainsFunc(this.codes, func(code string) bool { return slices.Contains(apollo.Codes, code) })
}

func TestApolloParity_EverySet_ComposesOrFailsAsApolloDoes(t *testing.T) {
	sets := apolloSetNames(t)
	divergences := knownDivergences(t)

	for _, known := range divergences {
		assert.Contains(t, sets, known, "known_divergences.txt names a set testdata/apollo lacks")
	}

	for _, set := range sets {
		t.Run(set, func(t *testing.T) {
			apollo := readApolloResult(t, set)
			if apollo.Exception != "" {
				t.Skipf("Apollo throws instead of answering: %s", apollo.Exception)
			}

			ours := composeApolloSet(t, set)
			if slices.Contains(divergences, set) {
				assert.False(t, ours.agreesWith(apollo), "now agrees with Apollo: remove it from known_divergences.txt")

				return
			}

			assert.True(t, ours.agreesWith(apollo), "Apollo composes: %t, codes %v; the broker composes: %t, codes %v:\n%s",
				apollo.Composes, apollo.Codes, ours.composes, ours.codes, strings.Join(ours.reasons, "\n"))
		})
	}
}
