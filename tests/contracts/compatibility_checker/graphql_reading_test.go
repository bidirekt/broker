package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readingConsumerName = "petstore_web"
	readingProviderName = "petstore_api"
	readingEnvironment  = "production"
)

const readingInterfaceSDL = `interface Node {
  id: ID!
  name: String
}

type Person implements Node {
  id: ID!
  name: String
  age: Int
}

type Org implements Node {
  id: ID!
  name: String
}

type Query {
  node(id: ID!): Node
}
`

const readingInterfaceWithoutIDSDL = `interface Node {
  name: String
}

type Person implements Node {
  id: ID!
  name: String
  age: Int
}

type Org implements Node {
  id: ID!
  name: String
}

type Query {
  node(id: ID!): Node
}
`

const readingPetSDL = `type Pet {
  id: ID!
}

type Query {
  pet(id: ID!, weight: Float, name: String): Pet
}
`

const readingSearchSDL = `type Pet {
  id: ID!
}

type Owner {
  id: ID!
}

union SearchResult = Pet | Owner

type Query {
  search: SearchResult
}
`

type readingWithoutVerdict struct{}

func (this readingWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type readingBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

func readingProviderSchemas(t *testing.T, sdl string) model.Schemas {
	t.Helper()

	declarations, err := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "schema.graphqls", Content: sdl}})
	require.NoError(t, err)

	catalog := contract.SchemasMap{}
	roots := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		catalog[declaration.Name] = declaration.Schema
		roots = append(roots, declaration.Name)
	}

	return schemamapper.ToSchemaModels(catalog, roots)
}

func readingResources(t *testing.T, sdl, field, interaction string, consumerSchemas model.Schemas, consumerRoot string) (consumer, provider model.PersistedResource) {
	t.Helper()

	body := model.Output
	if interaction == graphqlmodel.ArgumentsInteraction {
		body = model.Input
	}

	identity := graphqlmodel.Identity{Operation: graphqlmodel.QueryOperation, Field: field, Interaction: interaction}
	providerHash := readingProviderName + "-query-" + field + "-" + interaction

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  readingConsumerName,
			Direction:        model.Consumes,
			Protocol:         graphqlmodel.Protocol,
			Body:             body,
			Identity:         identity,
			ConsumedProvider: null.StringFrom(readingProviderName),
			Bodies:           []model.ResourceBody{{Source: "petstore_web.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom(consumerRoot)}},
		},
		ConsumerHash: null.StringFrom(readingConsumerName + "-" + providerHash),
		ProviderHash: providerHash,
		Schemas:      map[string]model.Schemas{model.YAMLNamespace: consumerSchemas},
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: readingProviderName,
			Direction:       model.Provides,
			Protocol:        graphqlmodel.Protocol,
			Body:            body,
			Identity:        identity,
			Bodies:          []model.ResourceBody{{Source: "schema.graphqls", Namespace: graphqlmodel.Namespace, SchemaName: null.StringFrom("Query:" + field + ":" + interaction)}},
		},
		ProviderHash:       providerHash,
		Schemas:            map[string]model.Schemas{graphqlmodel.Namespace: readingProviderSchemas(t, sdl)},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func readingBreaksCheckedFrom(checked model.PersistedResource, counterparts model.ResourceCounterparts, counterpartName string) []readingBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(readingWithoutVerdict{}, []compatibility_checker.ProtocolHooks{graphqlcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(readingEnvironment),
		counterparts,
	)

	var breaks []readingBreak
	for _, found := range report.Results[counterpartName].Breaks {
		breaks = append(breaks, readingBreak{found.Reason, found.Details})
	}

	return breaks
}

func assertReadingBreaks(t *testing.T, sdl, field, interaction string, consumerSchemas model.Schemas, consumerRoot string, expected []readingBreak) {
	t.Helper()

	consumer, provider := readingResources(t, sdl, field, interaction, consumerSchemas, consumerRoot)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, readingBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{provider.ProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{readingProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, readingProviderName))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, readingBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{provider.ProviderHash: {consumer}},
		}, readingConsumerName))
	})
}

var readingNodeAsObject = model.Schemas{
	"NodeRead": {"": {Type: "object", Nullable: true}, "id": {Type: "string"}},
}

var readingNodeAsUnion = model.Schemas{
	"NodeRead":   {"": {Type: "union", Nullable: true}, "(Person)": {Ref: "PersonRead"}},
	"PersonRead": {"": {Type: "object"}, "id": {Type: "string"}, "age": {Type: "integer", Nullable: true}},
}

func TestGraphQLReading_AnInterfaceReadAsAnObject_IsCompatible(t *testing.T) {
	assertReadingBreaks(t, readingInterfaceSDL, "node", graphqlmodel.ReturnsInteraction, readingNodeAsObject, "NodeRead", nil)
}

func TestGraphQLReading_AnInterfaceReadAsTheUnionOfItsImplementations_IsCompatible(t *testing.T) {
	assertReadingBreaks(t, readingInterfaceSDL, "node", graphqlmodel.ReturnsInteraction, readingNodeAsUnion, "NodeRead", nil)
}

func TestGraphQLReading_AFieldRemovedOnlyFromTheInterface_BreaksTheObjectRead(t *testing.T) {
	assertReadingBreaks(t, readingInterfaceWithoutIDSDL, "node", graphqlmodel.ReturnsInteraction, readingNodeAsObject, "NodeRead", []readingBreak{
		{model.ReasonPropertyMissingInProvider, map[string]string{
			"property":      "$.id",
			"consumerName":  readingConsumerName,
			"providerName":  readingProviderName,
			"propertyType":  "string",
			"consumerField": "NodeRead.id",
		}},
	})
}

func TestGraphQLReading_AFieldRemovedOnlyFromTheInterface_KeepsTheUnionRead(t *testing.T) {
	assertReadingBreaks(t, readingInterfaceWithoutIDSDL, "node", graphqlmodel.ReturnsInteraction, readingNodeAsUnion, "NodeRead", nil)
}

func TestGraphQLReading_AUnionWithoutFieldsReadAsAnObject_IsATypeMismatch(t *testing.T) {
	assertReadingBreaks(t, readingSearchSDL, "search", graphqlmodel.ReturnsInteraction, model.Schemas{
		"SearchRead": {"": {Type: "object", Nullable: true}, "id": {Type: "string"}},
	}, "SearchRead", []readingBreak{
		{model.ReasonPropertyTypeMismatch, map[string]string{
			"property":             "$",
			"consumerName":         readingConsumerName,
			"providerName":         readingProviderName,
			"consumerPropertyType": "object",
			"providerPropertyType": "union",
			"consumerField":        "SearchRead",
			"providerField":        "Query.search",
		}},
	})
}

func TestGraphQLReading_AnOptionalFieldTheProviderLacks_Breaks(t *testing.T) {
	assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ReturnsInteraction, model.Schemas{
		"PetRead": {"": {Type: "object", Nullable: true}, "id": {Type: "string"}, "bornAt": {Type: "string", Optional: true, Nullable: true}},
	}, "PetRead", []readingBreak{
		{model.ReasonPropertyMissingInProvider, map[string]string{
			"property":      "$.bornAt",
			"consumerName":  readingConsumerName,
			"providerName":  readingProviderName,
			"propertyType":  "string",
			"consumerField": "PetRead.bornAt",
		}},
	})
}

func TestGraphQLReading_AStringReadFromAnID_IsCompatible(t *testing.T) {
	assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ReturnsInteraction, model.Schemas{
		"PetRead": {"": {Type: "object", Nullable: true}, "id": {Type: "string"}},
	}, "PetRead", nil)
}

func TestGraphQLReading_ArgumentsTheSpecCoerces_AreCompatible(t *testing.T) {
	for label, arguments := range map[string]model.Schema{
		"a string to an ID":    {"": {Type: "object"}, "id": {Type: "string"}},
		"an integer to an ID":  {"": {Type: "object"}, "id": {Type: "integer"}},
		"an integer to Float":  {"": {Type: "object"}, "id": {Type: "string"}, "weight": {Type: "integer"}},
		"a number to Float":    {"": {Type: "object"}, "id": {Type: "string"}, "weight": {Type: "number"}},
		"a float to Float":     {"": {Type: "object"}, "id": {Type: "string"}, "weight": {Type: "float"}},
		"a string to a String": {"": {Type: "object"}, "id": {Type: "string"}, "name": {Type: "string"}},
	} {
		t.Run(label, func(t *testing.T) {
			assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ArgumentsInteraction, model.Schemas{"PetArgs": arguments}, "PetArgs", nil)
		})
	}
}

func TestGraphQLReading_AnIntegerSentToAString_IsATypeMismatch(t *testing.T) {
	assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ArgumentsInteraction, model.Schemas{
		"PetArgs": {"": {Type: "object"}, "id": {Type: "string"}, "name": {Type: "integer"}},
	}, "PetArgs", []readingBreak{
		{model.ReasonPropertyTypeMismatch, map[string]string{
			"property":             "$",
			"argument":             "name",
			"consumerName":         readingConsumerName,
			"providerName":         readingProviderName,
			"consumerPropertyType": "integer",
			"providerPropertyType": "string",
			"consumerField":        "PetArgs.name",
			"providerField":        "Query.pet(name:)",
		}},
	})
}

func TestGraphQLReading_TheTypenameOfAnObject_IsAlwaysProvided(t *testing.T) {
	assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ReturnsInteraction, model.Schemas{
		"PetRead": {"": {Type: "object", Nullable: true}, "__typename": {Type: "string"}, "id": {Type: "string"}},
	}, "PetRead", nil)
}

func TestGraphQLReading_TheTypenameOfAUnion_IsAlwaysProvided(t *testing.T) {
	assertReadingBreaks(t, readingSearchSDL, "search", graphqlmodel.ReturnsInteraction, model.Schemas{
		"SearchRead": {"": {Type: "object", Nullable: true}, "__typename": {Type: "string"}},
	}, "SearchRead", nil)
}

func TestGraphQLReading_TheTypenameOfAUnionMember_IsAlwaysProvided(t *testing.T) {
	assertReadingBreaks(t, readingSearchSDL, "search", graphqlmodel.ReturnsInteraction, model.Schemas{
		"SearchRead": {"": {Type: "union", Nullable: true}, "(Pet)": {Ref: "PetRead"}},
		"PetRead":    {"": {Type: "object"}, "__typename": {Type: "string"}, "id": {Type: "string"}},
	}, "SearchRead", nil)
}

func TestGraphQLReading_ATypenameSentAsAnArgument_Breaks(t *testing.T) {
	assertReadingBreaks(t, readingPetSDL, "pet", graphqlmodel.ArgumentsInteraction, model.Schemas{
		"PetArgs": {"": {Type: "object"}, "id": {Type: "string"}, "__typename": {Type: "string"}},
	}, "PetArgs", []readingBreak{
		{model.ReasonPropertyMissingInProvider, map[string]string{
			"property":      "$",
			"argument":      "__typename",
			"consumerName":  readingConsumerName,
			"providerName":  readingProviderName,
			"propertyType":  "string",
			"consumerField": "PetArgs.__typename",
		}},
	})
}

func TestGraphQLReading_AnIntrospectionRootField_IsAlwaysProvided(t *testing.T) {
	for _, field := range []string{"__schema", "__type"} {
		t.Run(field, func(t *testing.T) {
			consumer, _ := readingResources(t, readingPetSDL, field, graphqlmodel.ReturnsInteraction, model.Schemas{
				"SchemaRead": {"": {Type: "object"}, "description": {Type: "string", Nullable: true}},
			}, "SchemaRead")

			checkedContract := &model.PersistedContract{
				ParticipantName: consumer.ParticipantName,
				Version:         "2.0.0",
				Resources:       map[string]model.PersistedResource{consumer.PrimaryHash(): consumer},
			}

			report := compatibility_checker.NewCompatibilityChecker(readingWithoutVerdict{}, []compatibility_checker.ProtocolHooks{graphqlcontracts.New()}).Check(
				context.Background(),
				checkedContract,
				model.NewEnvironment(readingEnvironment),
				model.ResourceCounterparts{
					RegisteredProviders: map[string]model.RegisteredProvider{readingProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
				},
			)

			assert.Empty(t, report.Results[readingProviderName].Breaks)
			assert.True(t, report.Results[readingProviderName].Deployable)
			assert.Equal(t, null.StringFrom("1.0.0"), report.Results[readingProviderName].Counterpart.ParticipantVersion)
		})
	}
}
