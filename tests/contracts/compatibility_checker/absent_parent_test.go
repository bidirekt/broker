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

func groupsProperties(properties ...model.Property) map[string]model.Property {
	byPath := make(map[string]model.Property, len(properties))
	for _, property := range properties {
		byPath[property.Path] = property
	}

	return byPath
}

func groupRequestReadByTheProvider() map[string]model.Property {
	return groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
		model.Property{Path: "$.address", Type: "object", Optional: true},
		model.Property{Path: "$.address.address", Type: "string"},
		model.Property{Path: "$.address.city", Type: "string"},
		model.Property{Path: "$.address.state", Type: "string"},
		model.Property{Path: "$.address.zipCode", Type: "string"},
		model.Property{Path: "$.categories", Type: "array", Optional: true},
		model.Property{Path: "$.categories[]", Type: "string"},
		model.Property{Path: "$.leaders", Type: "array", Optional: true},
		model.Property{Path: "$.leaders[]", Type: "string"},
		model.Property{Path: "$.members", Type: "array", Optional: true},
		model.Property{Path: "$.members[]", Type: "string"},
	)
}

func groupRequestMergedFrom(t *testing.T, modules ...string) map[string]model.Property {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(modules))
	for index, module := range modules {
		fragment, err := contract.DecodeFragment(fmt.Sprintf("module_%d.yaml", index), module)
		require.NoError(t, err)

		fragments = append(fragments, fragment)
	}

	resources := fragmentmapper.ToResourceModels(fragmentmapper.ToDeclarations(fragments))
	require.Len(t, resources, 1)

	return resources[0].Properties
}

func groupsResources(
	interaction string,
	consumerProperties, providerProperties map[string]model.Property,
) (consumer, provider model.PersistedResource) {
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
		Properties:         consumerProperties,
	}

	provider = model.PersistedResource{
		ParticipantName:    groupsProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       groupsProviderHash,
		Endpoint:           "/api/groups",
		Method:             "post",
		ResponseStatusCode: status,
		Properties:         providerProperties,
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
	consumerProperties, providerProperties map[string]model.Property,
	expected []groupsBreak,
) {
	t.Helper()

	consumer, provider := groupsResources(interaction, consumerProperties, providerProperties)

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
	sentByTheConsumer := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
	)

	assertGroupsBreaks(t, "request", sentByTheConsumer, groupRequestReadByTheProvider(), nil)
}

func TestRequestWithAPartialAddressBreaksOnItsMissingChildren(t *testing.T) {
	sentByTheConsumer := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
		model.Property{Path: "$.address", Type: "object"},
		model.Property{Path: "$.address.address", Type: "string"},
	)

	assertGroupsBreaks(t, "request", sentByTheConsumer, groupRequestReadByTheProvider(), []groupsBreak{
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.city"},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.state"},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address.zipCode"},
	})
}

func TestRequestWithTheWholeAddressHasNoBreak(t *testing.T) {
	sentByTheConsumer := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
		model.Property{Path: "$.address", Type: "object"},
		model.Property{Path: "$.address.address", Type: "string"},
		model.Property{Path: "$.address.city", Type: "string"},
		model.Property{Path: "$.address.state", Type: "string"},
		model.Property{Path: "$.address.zipCode", Type: "string"},
	)

	assertGroupsBreaks(t, "request", sentByTheConsumer, groupRequestReadByTheProvider(), nil)
}

func TestAbsentRequiredParentBreaksOnlyOnItself(t *testing.T) {
	t.Run("in a request", func(t *testing.T) {
		readByTheProvider := groupRequestReadByTheProvider()
		readByTheProvider["$.address"] = model.Property{Path: "$.address", Type: "object"}

		sentByTheConsumer := groupsProperties(
			model.Property{Path: "$", Type: "object"},
			model.Property{Path: "$.name", Type: "string"},
		)

		assertGroupsBreaks(t, "request", sentByTheConsumer, readByTheProvider, []groupsBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, "$.address"},
		})
	})

	t.Run("in a response", func(t *testing.T) {
		readByTheConsumer := groupsProperties(
			model.Property{Path: "$", Type: "object"},
			model.Property{Path: "$.address", Type: "object"},
			model.Property{Path: "$.address.city", Type: "string"},
		)

		providedByTheProvider := groupsProperties(model.Property{Path: "$", Type: "object"})

		assertGroupsBreaks(t, "201", readByTheConsumer, providedByTheProvider, []groupsBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, "$.address"},
		})
	})
}

func TestResponseWithoutTheOptionalAddressHasNoBreak(t *testing.T) {
	readByTheConsumer := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.address", Type: "object", Optional: true},
		model.Property{Path: "$.address.city", Type: "string"},
	)

	providedByTheProvider := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
	)

	assertGroupsBreaks(t, "201", readByTheConsumer, providedByTheProvider, nil)
}

func TestArrayItemsAndMapValuesAreChildrenOfTheirCollection(t *testing.T) {
	root := model.Property{Path: "$", Type: "object"}

	collections := []struct {
		segment    string
		collection model.Property
		child      model.Property
	}{
		{"[]", model.Property{Path: "$.categories", Type: "array"}, model.Property{Path: "$.categories[]", Type: "string"}},
		{"{}", model.Property{Path: "$.pets", Type: "object"}, model.Property{Path: "$.pets{}", Type: "object"}},
	}

	for _, tc := range collections {
		t.Run(tc.segment+" of an absent required collection in a request", func(t *testing.T) {
			assertGroupsBreaks(t, "request",
				groupsProperties(root),
				groupsProperties(root, tc.collection, tc.child),
				[]groupsBreak{{compatibility_checker.ReasonPropertyMissingInConsumer, tc.collection.Path}},
			)
		})

		t.Run(tc.segment+" of an absent required collection in a response", func(t *testing.T) {
			assertGroupsBreaks(t, "201",
				groupsProperties(root, tc.collection, tc.child),
				groupsProperties(root),
				[]groupsBreak{{compatibility_checker.ReasonPropertyMissingInProvider, tc.collection.Path}},
			)
		})
	}
}

func TestRequestMergedFromModulesSendingTheAddressOnlySometimesHasNoBreak(t *testing.T) {
	readByTheProvider := groupsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.name", Type: "string"},
		model.Property{Path: "$.address", Type: "object", Optional: true},
		model.Property{Path: "$.address.city", Type: "string"},
	)

	t.Run("with the address module first", func(t *testing.T) {
		sentByTheConsumer := groupRequestMergedFrom(t, groupWithAddressModuleYAML, groupNameModuleYAML)

		assertGroupsBreaks(t, "request", sentByTheConsumer, readByTheProvider, nil)
	})

	t.Run("with the address module last", func(t *testing.T) {
		sentByTheConsumer := groupRequestMergedFrom(t, groupNameModuleYAML, groupWithAddressModuleYAML)

		assertGroupsBreaks(t, "request", sentByTheConsumer, readByTheProvider, nil)
	})
}
