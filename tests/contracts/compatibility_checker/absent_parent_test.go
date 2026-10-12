package compatibility_checker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	groupsConsumerName = "spa"
	groupsProviderName = "api"
	groupsEnvironment  = "production"
	groupsProviderHash = "post-api-groups"
)

const groupWithAddressModuleYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupWithAddress
schemas:
  GroupWithAddress:
    type: object
    properties:
      name: { type: string }
      address:
        type: object
        properties:
          city: { type: string }
`

const groupNameModuleYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupName
schemas:
  GroupName:
    type: object
    properties:
      name: { type: string }
`

type groupsWithoutVerdict struct{}

func (this groupsWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type groupsBreak struct {
	reason   compatibility_checker.BreakingReason
	property string
}

type groupsBody struct {
	bodies  []model.ResourceBody
	schemas model.Schemas
}

func groupsSchema(schema model.Schema) groupsBody {
	return groupsBody{
		bodies:  []model.ResourceBody{{Source: "groups.yaml", SchemaName: null.StringFrom("Group")}},
		schemas: model.Schemas{"Group": schema},
	}
}

func groupRequestReadByTheProvider() model.Schema {
	return model.Schema{
		"":                {Type: "object"},
		"name":            {Type: "string"},
		"address":         {Type: "object", Optional: true},
		"address.address": {Type: "string"},
		"address.city":    {Type: "string"},
		"address.state":   {Type: "string"},
		"address.zipCode": {Type: "string"},
		"categories":      {Type: "array", Optional: true},
		"categories[]":    {Type: "string"},
		"leaders":         {Type: "array", Optional: true},
		"leaders[]":       {Type: "string"},
		"members":         {Type: "array", Optional: true},
		"members[]":       {Type: "string"},
	}
}

func groupRequestMergedFrom(t *testing.T, modules ...string) groupsBody {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(modules))
	for index, module := range modules {
		fragment, err := contract.DecodeFragment(fmt.Sprintf("module_%d.yaml", index), module)
		require.NoError(t, err)

		fragments = append(fragments, fragment)
	}

	declarations := fragmentmapper.ToDeclarations(fragments)
	resources := fragmentmapper.ToResourceModels(declarations)
	require.Len(t, resources, 1)

	return groupsBody{bodies: resources[0].Bodies, schemas: fragmentmapper.ToSchemaModels(declarations)}
}

func groupsResources(interaction string, consumerBody, providerBody groupsBody) (consumer, provider model.PersistedResource) {
	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer = model.PersistedResource{
		ParticipantName:    groupsConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(groupsProviderName),
		ConsumerHash:       null.StringFrom("spa-" + groupsProviderHash),
		ProviderHash:       groupsProviderHash,
		Endpoint:           "/api/groups",
		Method:             "post",
		ResponseStatusCode: status,
		Bodies:             consumerBody.bodies,
		Schemas:            consumerBody.schemas,
	}

	provider = model.PersistedResource{
		ParticipantName:    groupsProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       groupsProviderHash,
		Endpoint:           "/api/groups",
		Method:             "post",
		ResponseStatusCode: status,
		Bodies:             providerBody.bodies,
		Schemas:            providerBody.schemas,
		DeployedVersions:   map[string]string{groupsEnvironment: "1.0.0"},
	}

	return consumer, provider
}

func groupsBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []groupsBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(groupsWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(groupsEnvironment),
		counterparts,
	)

	var breaks []groupsBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints["/api/groups"]["post"][interaction] {
		breaks = append(breaks, groupsBreak{leaf.Reason, leaf.Details["property"]})
	}

	return breaks
}

func assertGroupsBreaks(
	t *testing.T,
	interaction string,
	consumerBody, providerBody groupsBody,
	expected []groupsBreak,
) {
	t.Helper()

	consumer, provider := groupsResources(interaction, consumerBody, providerBody)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, groupsBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers: map[string]model.PersistedResource{groupsProviderHash: provider},
		}, groupsProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, groupsBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{groupsProviderHash: {consumer}},
		}, groupsConsumerName, interaction))
	})
}

func TestRequestWithoutTheOptionalAddressHasNoBreak(t *testing.T) {
	sentByTheConsumer := model.Schema{
		"":     {Type: "object"},
		"name": {Type: "string"},
	}

	assertGroupsBreaks(t, "request", groupsSchema(sentByTheConsumer), groupsSchema(groupRequestReadByTheProvider()), nil)
}

func TestRequestWithAPartialAddressBreaksOnItsMissingChildren(t *testing.T) {
	sentByTheConsumer := model.Schema{
		"":                {Type: "object"},
		"name":            {Type: "string"},
		"address":         {Type: "object"},
		"address.address": {Type: "string"},
	}

	assertGroupsBreaks(t, "request", groupsSchema(sentByTheConsumer), groupsSchema(groupRequestReadByTheProvider()), []groupsBreak{
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.city"},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.state"},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.zipCode"},
	})
}

func TestRequestWithTheWholeAddressHasNoBreak(t *testing.T) {
	sentByTheConsumer := model.Schema{
		"":                {Type: "object"},
		"name":            {Type: "string"},
		"address":         {Type: "object"},
		"address.address": {Type: "string"},
		"address.city":    {Type: "string"},
		"address.state":   {Type: "string"},
		"address.zipCode": {Type: "string"},
	}

	assertGroupsBreaks(t, "request", groupsSchema(sentByTheConsumer), groupsSchema(groupRequestReadByTheProvider()), nil)
}

func TestAbsentRequiredParentBreaksOnlyOnItself(t *testing.T) {
	t.Run("in a request", func(t *testing.T) {
		readByTheProvider := groupRequestReadByTheProvider()
		readByTheProvider["address"] = model.Field{Type: "object"}

		sentByTheConsumer := model.Schema{
			"":     {Type: "object"},
			"name": {Type: "string"},
		}

		assertGroupsBreaks(t, "request", groupsSchema(sentByTheConsumer), groupsSchema(readByTheProvider), []groupsBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address"},
		})
	})

	t.Run("in a response", func(t *testing.T) {
		readByTheConsumer := model.Schema{
			"":             {Type: "object"},
			"address":      {Type: "object"},
			"address.city": {Type: "string"},
		}

		providedByTheProvider := model.Schema{"": {Type: "object"}}

		assertGroupsBreaks(t, "201", groupsSchema(readByTheConsumer), groupsSchema(providedByTheProvider), []groupsBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, "$.address"},
		})
	})
}

func TestResponseWithoutTheOptionalAddressHasNoBreak(t *testing.T) {
	readByTheConsumer := model.Schema{
		"":             {Type: "object"},
		"address":      {Type: "object", Optional: true},
		"address.city": {Type: "string"},
	}

	providedByTheProvider := model.Schema{
		"":     {Type: "object"},
		"name": {Type: "string"},
	}

	assertGroupsBreaks(t, "201", groupsSchema(readByTheConsumer), groupsSchema(providedByTheProvider), nil)
}

func TestArrayItemsAndMapValuesAreChildrenOfTheirCollection(t *testing.T) {
	collections := []struct {
		segment    string
		collection model.Schema
		path       string
	}{
		{"[]", model.Schema{"": {Type: "object"}, "categories": {Type: "array"}, "categories[]": {Type: "string"}}, "$.categories"},
		{"{}", model.Schema{"": {Type: "object"}, "pets": {Type: "object"}, "pets{}": {Type: "object"}}, "$.pets"},
	}

	root := model.Schema{"": {Type: "object"}}

	for _, tc := range collections {
		t.Run(tc.segment+" of an absent required collection in a request", func(t *testing.T) {
			assertGroupsBreaks(t, "request",
				groupsSchema(root),
				groupsSchema(tc.collection),
				[]groupsBreak{{compatibility_checker.ReasonPropertyMissingInConsumer, tc.path}},
			)
		})

		t.Run(tc.segment+" of an absent required collection in a response", func(t *testing.T) {
			assertGroupsBreaks(t, "201",
				groupsSchema(tc.collection),
				groupsSchema(root),
				[]groupsBreak{{compatibility_checker.ReasonPropertyMissingInProvider, tc.path}},
			)
		})
	}
}

func TestRequestMergedFromModulesSendingTheAddressOnlySometimesHasNoBreak(t *testing.T) {
	readByTheProvider := groupsSchema(model.Schema{
		"":             {Type: "object"},
		"name":         {Type: "string"},
		"address":      {Type: "object", Optional: true},
		"address.city": {Type: "string"},
	})

	t.Run("with the address module first", func(t *testing.T) {
		sentByTheConsumer := groupRequestMergedFrom(t, groupWithAddressModuleYAML, groupNameModuleYAML)

		assertGroupsBreaks(t, "request", sentByTheConsumer, readByTheProvider, nil)
	})

	t.Run("with the address module last", func(t *testing.T) {
		sentByTheConsumer := groupRequestMergedFrom(t, groupNameModuleYAML, groupWithAddressModuleYAML)

		assertGroupsBreaks(t, "request", sentByTheConsumer, readByTheProvider, nil)
	})
}
