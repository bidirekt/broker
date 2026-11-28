package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

const (
	closedInputConsumerName = "petstore_web"
	closedInputProviderName = "petstore_api"
	closedInputEnvironment  = "production"
)

type closedInputWithoutVerdict struct{}

func (this closedInputWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type closedInputBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

type closedInputBody struct {
	namespace string
	name      string
	schemas   model.Schemas
}

func closedInputYAMLBody(name string, schemas model.Schemas) closedInputBody {
	return closedInputBody{namespace: model.YAMLNamespace, name: name, schemas: schemas}
}

func closedInputSDLBody(interaction string, schemas model.Schemas) closedInputBody {
	return closedInputBody{namespace: graphqlmodel.Namespace, name: "Query:pet:" + interaction, schemas: schemas}
}

func closedInputResource(direction model.Direction, interaction string, body closedInputBody) model.PersistedResource {
	role := model.Output
	if interaction == graphqlmodel.ArgumentsInteraction {
		role = model.Input
	}

	resource := model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: closedInputProviderName,
			Direction:       direction,
			Protocol:        graphqlmodel.Protocol,
			Body:            role,
			Identity:        graphqlmodel.Identity{Operation: graphqlmodel.QueryOperation, Field: "pet", Interaction: interaction},
			Bodies:          []model.ResourceBody{{Source: "petstore_api.graphql", Namespace: body.namespace, SchemaName: null.StringFrom(body.name)}},
		},
		ProviderHash:       "petstore_api-query-pet-" + interaction,
		Schemas:            map[string]model.Schemas{body.namespace: body.schemas},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	if direction == model.Consumes {
		resource.ParticipantName = closedInputConsumerName
		resource.ConsumedProvider = null.StringFrom(closedInputProviderName)
		resource.ConsumerHash = null.StringFrom("petstore_web-" + resource.ProviderHash)
		resource.ParticipantVersion = null.String{}
		resource.Bodies[0].Source = "petstore_web.yaml"
	}

	return resource
}

func closedInputBreaksCheckedFrom(checked model.PersistedResource, counterparts model.ResourceCounterparts, counterpartName string) []closedInputBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(closedInputWithoutVerdict{}, []compatibility_checker.ProtocolHooks{graphqlcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(closedInputEnvironment),
		counterparts,
	)

	var breaks []closedInputBreak
	for _, found := range report.Results[counterpartName].Breaks {
		breaks = append(breaks, closedInputBreak{found.Reason, found.Details})
	}

	return breaks
}

func assertClosedInputBreaks(t *testing.T, interaction string, consumerBody, providerBody closedInputBody, expected []closedInputBreak) {
	t.Helper()

	consumer := closedInputResource(model.Consumes, interaction, consumerBody)
	provider := closedInputResource(model.Provides, interaction, providerBody)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, closedInputBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{provider.ProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{closedInputProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, closedInputProviderName))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, closedInputBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{provider.ProviderHash: {consumer}},
		}, closedInputConsumerName))
	})
}

func closedInputDetails(property, propertyType, consumerField, providerField string) map[string]string {
	details := map[string]string{
		"property":     property,
		"consumerName": closedInputConsumerName,
		"providerName": closedInputProviderName,
		"propertyType": propertyType,
	}

	if consumerField != "" {
		details["consumerField"] = consumerField
	}

	if providerField != "" {
		details["providerField"] = providerField
	}

	return details
}

func closedInputArgumentDetails(argument, propertyType, consumerField, providerField string) map[string]string {
	details := closedInputDetails("$", propertyType, consumerField, providerField)
	details["argument"] = argument

	return details
}

func TestClosedInput_AnOptionalArgumentTheProviderRemoved_Breaks(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{"PetArgs": {
		"":    {Type: "object"},
		"id":  {Type: "string"},
		"tag": {Type: "string", Optional: true, Nullable: true},
	}})
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyMissingInProvider,
		closedInputArgumentDetails("tag", "String", "Query.pet(tag:)", ""),
	}})
}

func TestClosedInput_ARequiredArgumentTheProviderRemoved_Breaks(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{"PetArgs": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {"": {Type: "object"}}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyMissingInProvider,
		closedInputArgumentDetails("id", "String!", "Query.pet(id:)", ""),
	}})
}

func TestClosedInput_AnInputFieldTheProviderRemoved_Breaks(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{
		"PetArgs":   {"": {Type: "object"}, "filter": {Ref: "PetFilter"}},
		"PetFilter": {"": {Type: "object"}, "species": {Type: "string"}, "minAge": {Type: "integer", Optional: true, Nullable: true}},
	})
	provider := closedInputSDLBody("arguments", model.Schemas{
		"Query:pet:arguments": {"": {Type: "object"}, "filter": {Ref: "PetFilter"}},
		"PetFilter":           {"": {Type: "object"}, "species": {Type: "string"}},
	})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyMissingInProvider,
		closedInputArgumentDetails("filter.minAge", "Int", "PetFilter.minAge", ""),
	}})
}

func TestClosedInput_AnOptionalArgumentTheConsumerDoesNotSend_IsCompatible(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{"PetArgs": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {
		"":    {Type: "object"},
		"id":  {Type: "string"},
		"tag": {Type: "string", Optional: true, Nullable: true},
	}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, nil)
}

func TestClosedInput_ANewRequiredArgument_BreaksTheConsumerThatDeclaresArguments(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{"PetArgs": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {
		"":      {Type: "object"},
		"id":    {Type: "string"},
		"owner": {Type: "string"},
	}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyMissingInConsumer,
		closedInputArgumentDetails("owner", "String!", "", "Query.pet(owner:)"),
	}})
}

func TestClosedInput_ANewRequiredArgument_BreaksTheConsumerWithoutArguments(t *testing.T) {
	consumer := closedInputBody{
		namespace: graphqlmodel.GeneratedNamespace,
		name:      graphqlmodel.EmptyArgumentsSchema,
		schemas:   model.Schemas{graphqlmodel.EmptyArgumentsSchema: {"": {Type: "object"}}},
	}
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyMissingInConsumer,
		closedInputArgumentDetails("id", "String!", "", "Query.pet(id:)"),
	}})
}

func TestClosedInput_ANullableArgumentThatBecomesNonNull_Breaks(t *testing.T) {
	consumer := closedInputYAMLBody("PetArgs", model.Schemas{"PetArgs": {
		"":   {Type: "object"},
		"id": {Type: "string", Optional: true, Nullable: true},
	}})
	provider := closedInputSDLBody("arguments", model.Schemas{"Query:pet:arguments": {
		"":   {Type: "object"},
		"id": {Type: "string"},
	}})

	assertClosedInputBreaks(t, "arguments", consumer, provider, []closedInputBreak{
		{model.ReasonPropertyOptionalInConsumerRequiredInProvider, closedInputArgumentDetails("id", "String", "Query.pet(id:)", "Query.pet(id:)")},
		{model.ReasonPropertyNullableInConsumerNotNullableInProvider, closedInputArgumentDetails("id", "String", "Query.pet(id:)", "Query.pet(id:)")},
	})
}

func TestClosedInput_ANonNullOutputFieldThatBecomesNullable_Breaks(t *testing.T) {
	consumer := closedInputYAMLBody("Pet", model.Schemas{"Pet": {
		"":     {Type: "object"},
		"name": {Type: "string"},
	}})
	provider := closedInputSDLBody("returns", model.Schemas{"Query:pet:returns": {
		"":     {Type: "object"},
		"name": {Type: "string", Nullable: true},
	}})

	assertClosedInputBreaks(t, "returns", consumer, provider, []closedInputBreak{{
		model.ReasonPropertyNullableInProviderNotNullableInConsumer,
		closedInputDetails("$.name", "String!", "Pet.name", "Query.pet.name"),
	}})
}

func TestClosedInput_AnOutputFieldTheConsumerDoesNotRead_IsCompatible(t *testing.T) {
	consumer := closedInputYAMLBody("Pet", model.Schemas{"Pet": {
		"":     {Type: "object"},
		"name": {Type: "string"},
	}})
	provider := closedInputSDLBody("returns", model.Schemas{"Query:pet:returns": {
		"":     {Type: "object"},
		"name": {Type: "string"},
		"age":  {Type: "integer"},
	}})

	assertClosedInputBreaks(t, "returns", consumer, provider, nil)
}
