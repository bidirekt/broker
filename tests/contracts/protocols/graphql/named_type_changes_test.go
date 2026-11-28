package graphql_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const (
	namedTypeProviderID = 1
	namedTypeConsumerID = 2
	namedTypeDeployedID = 10
	namedTypeChangedID  = 11
	namedTypeWebID      = 20
	namedTypeGraphName  = "petstore_graph"
)

const namedTypeOperations = `type Query {
  pet(id: ID!): Pet
  pets(ids: [ID!]!): [Pet!]!
  near(id: ID): Pet
  tag(status: PetStatus!): Pet
  one(ids: [ID!]!): Pet
}

enum PetStatus {
  AVAILABLE
  SOLD
}
`

const namedTypeSDL = namedTypeOperations + `
type Pet {
  id: ID!
  name: String!
}
`

const namedTypeSubgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

` + namedTypeOperations + `
type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const namedTypeGraphYAML = "provides:\n  graphql:\n    graph: " + namedTypeGraphName + "\n"

const namedTypeWebYAML = `consumes:
  %s:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        pets: {arguments: PetsArgs, returns: Pets}
        near: {arguments: NearArgs, returns: MaybePet}
        tag: {arguments: TagArgs, returns: MaybePet}
        one: {arguments: OneArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  PetsArgs: {type: object, properties: {ids: {type: array, items: {type: string}}}}
  NearArgs: {type: object, properties: {id: {type: string, nullable: true}}}
  TagArgs: {type: object, properties: {status: {ref: Status}}}
  OneArgs: {type: object, properties: {ids: {type: string}}}
  Status: {type: enum, members: [SOLD]}
  MaybePet: {ref: Pet, nullable: true}
  Pets: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {name: {type: string}}}
`

type namedTypeBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

type namedTypeWithoutVerdict struct{}

func (this namedTypeWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type namedTypeGraphs struct {
	named     map[string]bool
	members   map[string][]graphqlmodel.DeployedContract
	consumers map[string][]graphqlmodel.DeployedContract
	graphOf   map[int64]string
}

func (this namedTypeGraphs) GraphNamed(_ context.Context, name string) bool {
	return this.named[name]
}

func (this namedTypeGraphs) DeployedMembers(_ context.Context, _ int64, graph string) []graphqlmodel.DeployedContract {
	return this.members[graph]
}

func (this namedTypeGraphs) DeployedConsumers(_ context.Context, _ int64, provider string) []graphqlmodel.DeployedContract {
	return this.consumers[provider]
}

func (this namedTypeGraphs) DeployedGraph(_ context.Context, _, participantID int64) string {
	return this.graphOf[participantID]
}

type namedTypeDeployments map[int64]*model.PersistedContract

func (this namedTypeDeployments) CurrentDeploymentInEnv(_ context.Context, participantID, _ int64) (string, int64, bool) {
	contract, deployed := this[participantID]
	if !deployed {
		return "", 0, false
	}

	return contract.Version, contract.ID, true
}

type namedTypeContracts map[int64]*model.PersistedContract

func (this namedTypeContracts) GetContractByIDAndVersion(_ context.Context, contractID int64, _ string) *model.PersistedContract {
	return this[contractID]
}

func namedTypeContract(t *testing.T, id, participantID int64, name, version string, files ...model.ContractFile) *model.PersistedContract {
	t.Helper()

	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	uploaded, violations, readErr, err := interpreter.ToUploadedContract(files, &model.Participant{ID: participantID, Name: name}, version)
	require.NoError(t, readErr)
	require.Empty(t, violations)
	require.NoError(t, err)

	resources := map[string]model.PersistedResource{}
	for hash, resource := range uploaded.Resources {
		persisted := model.PersistedResource{
			Resource:           resource.Resource,
			Schemas:            model.CatalogFor(uploaded.Schemas, resource.Bodies),
			ProviderHash:       resource.ProviderHash(),
			ContractID:         id,
			ParticipantVersion: null.StringFrom(version),
		}
		persisted.ParticipantName = name
		resources[hash] = persisted
	}

	return &model.PersistedContract{
		ID:              id,
		ParticipantID:   participantID,
		ParticipantName: name,
		Version:         version,
		Resources:       resources,
		Schemas:         uploaded.Schemas,
		Extensions:      uploaded.Extensions,
	}
}

func namedTypeConsumers(contract *model.PersistedContract) map[string][]model.PersistedResource {
	consumers := map[string][]model.PersistedResource{}
	for _, resource := range contract.Resources {
		if resource.IsConsumer() {
			consumers[resource.ProviderHash] = append(consumers[resource.ProviderHash], resource)
		}
	}

	return consumers
}

func namedTypeBreaks(t *testing.T, viaGraph bool, oldText, newText string) []namedTypeBreak {
	t.Helper()

	sdl, files, consumed := namedTypeSDL, func(sdl string) []model.ContractFile {
		return []model.ContractFile{{Source: "schema.graphqls", Content: sdl}}
	}, "petstore_api"
	if viaGraph {
		sdl, files, consumed = namedTypeSubgraphSDL, func(sdl string) []model.ContractFile {
			return []model.ContractFile{{Source: "graph.yaml", Content: namedTypeGraphYAML}, {Source: "api.graphqls", Content: sdl}}
		}, namedTypeGraphName
	}

	changedSDL := strings.ReplaceAll(sdl, oldText, newText)
	require.NotEqual(t, sdl, changedSDL)

	deployed := namedTypeContract(t, namedTypeDeployedID, namedTypeProviderID, "petstore_api", "1.0.0", files(sdl)...)
	changed := namedTypeContract(t, namedTypeChangedID, namedTypeProviderID, "petstore_api", "1.1.0", files(changedSDL)...)
	web := namedTypeContract(t, namedTypeWebID, namedTypeConsumerID, "petstore_web", "1.0.0",
		model.ContractFile{Source: "web.yaml", Content: fmt.Sprintf(namedTypeWebYAML, consumed)})

	webDeployment := []graphqlmodel.DeployedContract{{ParticipantID: namedTypeConsumerID, ContractID: namedTypeWebID, Version: "1.0.0"}}
	graphs := namedTypeGraphs{consumers: map[string][]graphqlmodel.DeployedContract{"petstore_api": webDeployment}}
	counterparts := model.ResourceCounterparts{Consumers: namedTypeConsumers(web)}

	if viaGraph {
		graphs = namedTypeGraphs{
			named:     map[string]bool{namedTypeGraphName: true},
			members:   map[string][]graphqlmodel.DeployedContract{namedTypeGraphName: {{ParticipantID: namedTypeProviderID, ContractID: namedTypeDeployedID, Version: "1.0.0"}}},
			consumers: map[string][]graphqlmodel.DeployedContract{namedTypeGraphName: webDeployment},
			graphOf:   map[int64]string{namedTypeProviderID: namedTypeGraphName},
		}
		counterparts = model.ResourceCounterparts{}
	}

	hooks := graphqlcontracts.NewCheckerHooks(
		graphs,
		namedTypeDeployments{namedTypeProviderID: deployed},
		namedTypeContracts{namedTypeDeployedID: deployed, namedTypeChangedID: changed, namedTypeWebID: web},
	)

	report := compatibility_checker.NewCompatibilityChecker(namedTypeWithoutVerdict{}, []compatibility_checker.ProtocolHooks{restcontracts.New(), hooks}).
		Check(context.Background(), changed, &model.Environment{ID: 1, Name: "production"}, counterparts)

	result := report.Results["petstore_web"]

	var breaks []namedTypeBreak
	for _, change := range append(result.Breaks, result.LiveBreaks...) {
		breaks = append(breaks, namedTypeBreak{change.Reason, change.Details})
	}

	return breaks
}

func namedTypeBreakAt(viaGraph bool, reason model.BreakingReason, details map[string]string) []namedTypeBreak {
	details["consumerName"] = "petstore_web"
	details["providerName"] = "petstore_api"
	details["property"] = "$"

	if viaGraph {
		details["providerName"] = namedTypeGraphName
		details["graph"] = namedTypeGraphName
	}

	return []namedTypeBreak{{reason, details}}
}

func assertNamedTypeBreaks(t *testing.T, oldText, newText string, expected func(viaGraph bool) []namedTypeBreak) {
	t.Helper()

	for name, viaGraph := range map[string]bool{"direct": false, "via graph": true} {
		t.Run(name, func(t *testing.T) {
			assert.ElementsMatch(t, expected(viaGraph), namedTypeBreaks(t, viaGraph, oldText, newText))
		})
	}
}

func TestNamedTypeChanges_AScalarRenamedWithTheSameShape_BreaksOnlyByTheName(t *testing.T) {
	assertNamedTypeBreaks(t, "pet(id: ID!)", "pet(id: String!)", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyTypeMismatch, map[string]string{
			"argument": "id", "consumerPropertyType": "ID", "providerPropertyType": "String",
			"consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)",
		})
	})
}

func TestNamedTypeChanges_AnEnumRenamedWithTheSameValues_BreaksOnlyByTheName(t *testing.T) {
	assertNamedTypeBreaks(t, "PetStatus", "Status", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyTypeMismatch, map[string]string{
			"argument": "status", "consumerPropertyType": "PetStatus", "providerPropertyType": "Status",
			"consumerField": "Query.tag(status:)", "providerField": "Query.tag(status:)",
		})
	})
}

func TestNamedTypeChanges_AScalarFromIDToInt_BreaksOnlyByTheShape(t *testing.T) {
	assertNamedTypeBreaks(t, "pet(id: ID!)", "pet(id: Int!)", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyTypeMismatch, map[string]string{
			"argument": "id", "consumerPropertyType": "String!", "providerPropertyType": "Int!",
			"consumerField": "Query.pet(id:)", "providerField": "Query.pet(id:)",
		})
	})
}

func TestNamedTypeChanges_ASingleValueSentToARenamedList_BreaksOnlyByTheName(t *testing.T) {
	assertNamedTypeBreaks(t, "one(ids: [ID!]!)", "one(ids: [String!]!)", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyTypeMismatch, map[string]string{
			"argument": "ids", "consumerPropertyType": "ID", "providerPropertyType": "String",
			"consumerField": "Query.one(ids:)", "providerField": "Query.one(ids:)",
		})
	})
}

func TestNamedTypeChanges_AListWhoseItemsNoLongerFit_BreaksOnlyByTheShape(t *testing.T) {
	assertNamedTypeBreaks(t, "pets(ids: [ID!]!)", "pets(ids: [Int!]!)", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyTypeMismatch, map[string]string{
			"argument": "ids[]", "consumerPropertyType": "String!", "providerPropertyType": "Int!",
			"consumerField": "Query.pets(ids:)[]", "providerField": "Query.pets(ids:)[]",
		})
	})
}

func TestNamedTypeChanges_ANullableValueSentToARenamedNonNullType_BreaksOnlyByTheNullability(t *testing.T) {
	assertNamedTypeBreaks(t, "near(id: ID)", "near(id: String!)", func(viaGraph bool) []namedTypeBreak {
		return namedTypeBreakAt(viaGraph, model.ReasonPropertyNullableInConsumerNotNullableInProvider, map[string]string{
			"argument": "id", "propertyType": "String",
			"consumerField": "Query.near(id:)", "providerField": "Query.near(id:)",
		})
	})
}

func TestNamedTypeChanges_AnEnumRenamedWithoutAValueTheConsumerSends_BreaksOnlyByTheValue(t *testing.T) {
	assertNamedTypeBreaks(t,
		"PetStatus!): Pet\n  one(ids: [ID!]!): Pet\n}\n\nenum PetStatus {\n  AVAILABLE\n  SOLD\n}",
		"Status!): Pet\n  one(ids: [ID!]!): Pet\n}\n\nenum Status {\n  AVAILABLE\n}",
		func(viaGraph bool) []namedTypeBreak {
			return namedTypeBreakAt(viaGraph, model.ReasonPropertyMemberMissingInProvider, map[string]string{
				"argument": "status", "member": "SOLD",
				"consumerField": "Query.tag(status:)", "providerField": "Query.tag(status:)",
			})
		})
}
