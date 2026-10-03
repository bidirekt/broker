package compatibility_checker_test

import (
	"encoding/json"
	"testing"

	"github.com/bidirekt/broker/internal/features/can_i_deploy/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func consumerResource() *model.PersistedResource {
	return &model.PersistedResource{
		Direction:          model.Consumes,
		Interaction:        model.RestResponse,
		ConsumedProvider:   null.StringFrom("api"),
		Endpoint:           "/things",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		ParticipantName:    "front",
		Properties: map[string]model.Property{
			"$.id":   {Path: "$.id", Type: "integer"},
			"$.name": {Path: "$.name", Type: "string"},
		},
	}
}

func providerResource() *model.PersistedResource {
	return &model.PersistedResource{
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		Endpoint:           "/things",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		ParticipantName:    "api",
		Properties: map[string]model.Property{
			"$.id": {Path: "$.id", Type: "string"},
		},
	}
}

func TestProviderResourceNotFound(t *testing.T) {
	consumer := &model.PersistedResource{
		Direction:        model.Consumes,
		ConsumedProvider: null.StringFrom("accounts"),
		ParticipantName:  "front",
	}

	change := compatibility_checker.NewContractBreakingChange(
		consumer,
		nil,
		compatibility_checker.ReasonProviderResourceNotFound,
	)

	assert.Equal(t, compatibility_checker.ReasonProviderResourceNotFound, change.Reason)
	assert.Same(t, consumer, change.CheckedResource)
	assert.Nil(t, change.CounterpartResource)
	assert.Nil(t, change.Details)
	assert.Equal(t, "front", change.ConsumerName())
	assert.Equal(t, "accounts", change.ProviderName())
}

func TestPropertyBreakDetails(t *testing.T) {
	consumerResponse := consumerResource()
	providerResponse := providerResource()
	consumerRequest := &model.PersistedResource{
		Direction:        model.Consumes,
		Interaction:      model.RestRequest,
		ConsumedProvider: null.StringFrom("api"),
		Endpoint:         "/things",
		Method:           "post",
		ParticipantName:  "front",
		Properties: map[string]model.Property{
			"$.flag": {Path: "$.flag", Type: "boolean", Optional: true},
		},
	}
	providerRequest := &model.PersistedResource{
		Direction:       model.Provides,
		Interaction:     model.RestRequest,
		Endpoint:        "/things",
		Method:          "post",
		ParticipantName: "api",
		Properties: map[string]model.Property{
			"$.user": {Path: "$.user", Type: "string"},
			"$.flag": {Path: "$.flag", Type: "boolean"},
		},
	}

	typeMismatch := compatibility_checker.NewPropertyBreakChange(
		consumerResponse, providerResponse, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)
	missingInProvider := compatibility_checker.NewPropertyBreakChange(
		consumerResponse, providerResponse, compatibility_checker.ReasonPropertyMissingInProvider, "$.name",
	)
	optionalInProvider := compatibility_checker.NewPropertyBreakChange(
		consumerResponse, providerResponse, compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, "$.id",
	)
	missingInConsumer := compatibility_checker.NewPropertyBreakChange(
		consumerRequest, providerRequest, compatibility_checker.ReasonPropertyMissingInConsumer, "$.user",
	)
	optionalInConsumer := compatibility_checker.NewPropertyBreakChange(
		consumerRequest, providerRequest, compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, "$.flag",
	)

	assert.Equal(t, compatibility_checker.ReasonPropertyTypeMismatch, typeMismatch.Reason)
	assert.Equal(t, compatibility_checker.ReasonPropertyMissingInProvider, missingInProvider.Reason)
	assert.Equal(t, compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, optionalInProvider.Reason)
	assert.Equal(t, compatibility_checker.ReasonPropertyMissingInConsumer, missingInConsumer.Reason)
	assert.Equal(t, compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, optionalInConsumer.Reason)

	assert.Equal(t, map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, typeMismatch.Details)

	assert.Equal(t, map[string]string{"property": "$.name", "consumerName": "front", "providerName": "api", "propertyType": "string"}, missingInProvider.Details)
	assert.Equal(t, map[string]string{"property": "$.id", "consumerName": "front", "providerName": "api", "propertyType": "integer"}, optionalInProvider.Details)
	assert.Equal(t, map[string]string{"property": "$.user", "consumerName": "front", "providerName": "api", "propertyType": "string"}, missingInConsumer.Details)
	assert.Equal(t, map[string]string{"property": "$.flag", "consumerName": "front", "providerName": "api", "propertyType": "boolean"}, optionalInConsumer.Details)

	assert.Equal(t, "front", typeMismatch.ConsumerName())
	assert.Equal(t, "api", typeMismatch.ProviderName())
	assert.Equal(t, "front", missingInConsumer.ConsumerName())
	assert.Equal(t, "api", missingInConsumer.ProviderName())
}

func TestPropertyTypeRendersArrayItemType(t *testing.T) {
	consumer := consumerResource()
	consumer.Properties = map[string]model.Property{
		"$.list":      {Path: "$.list", Type: "array"},
		"$.list[]":    {Path: "$.list[]", Type: "object"},
		"$.list[].id": {Path: "$.list[].id", Type: "string"},
		"$.tags":      {Path: "$.tags", Type: "array"},
		"$.tags[]":    {Path: "$.tags[]", Type: "array"},
		"$.tags[][]":  {Path: "$.tags[][]", Type: "integer"},
		"$.other":     {Path: "$.other", Type: "array"},
		"$.other[]":   {Path: "$.other[]", Type: "string"},
	}
	provider := providerResource()
	provider.Properties = map[string]model.Property{
		"$.other": {Path: "$.other", Type: "array"},
	}

	missingList := compatibility_checker.NewPropertyBreakChange(
		consumer, provider, compatibility_checker.ReasonPropertyMissingInProvider, "$.list",
	)
	missingTags := compatibility_checker.NewPropertyBreakChange(
		consumer, provider, compatibility_checker.ReasonPropertyMissingInProvider, "$.tags",
	)
	mismatch := compatibility_checker.NewPropertyBreakChange(
		consumer, provider, compatibility_checker.ReasonPropertyTypeMismatch, "$.other",
	)

	assert.Equal(t, "array<object>", missingList.Details["propertyType"])
	assert.Equal(t, "array<array<integer>>", missingTags.Details["propertyType"])
	assert.Equal(t, "array<string>", mismatch.Details["consumerPropertyType"])
}

func TestTypeMismatchTypesResolvedByRole(t *testing.T) {
	consumerRes := consumerResource()
	providerRes := providerResource()

	consumerChecked := compatibility_checker.NewPropertyBreakChange(
		consumerRes, providerRes, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)
	providerChecked := compatibility_checker.NewPropertyBreakChange(
		providerRes, consumerRes, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)

	expected := map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}
	assert.Equal(t, expected, consumerChecked.Details)
	assert.Equal(t, expected, providerChecked.Details)

	assert.Same(t, providerRes, providerChecked.CheckedResource)
	assert.Same(t, consumerRes, providerChecked.CounterpartResource)

	assert.Equal(t, "front", consumerChecked.ConsumerName())
	assert.Equal(t, "api", consumerChecked.ProviderName())
	assert.Equal(t, "front", providerChecked.ConsumerName())
	assert.Equal(t, "api", providerChecked.ProviderName())
}

func TestBreakMarshalsOnlyReasonRoleAndDetails(t *testing.T) {
	change := compatibility_checker.NewPropertyBreakChange(
		consumerResource(), providerResource(), compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)

	data, err := json.Marshal(change)
	assert.NoError(t, err)

	decoded := map[string]json.RawMessage{}
	assert.NoError(t, json.Unmarshal(data, &decoded))

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}

	assert.ElementsMatch(t, []string{"reason", "role", "details"}, keys)
}

func TestProviderNotDeployedDetails(t *testing.T) {
	consumer := &model.PersistedResource{
		Direction:        model.Consumes,
		ConsumedProvider: null.StringFrom("accounts"),
		ParticipantName:  "front",
	}

	deployed := compatibility_checker.NewProviderResourceNotDeployed(
		consumer, []string{"staging", "prod"},
	)
	nowhere := compatibility_checker.NewProviderResourceNotDeployed(
		consumer, nil,
	)

	assert.Equal(t, compatibility_checker.ReasonProviderResourceNotDeployedInEnvironment, deployed.Reason)
	assert.Equal(t, map[string]string{"deployedEnvironments": "staging, prod"}, deployed.Details)
	assert.Equal(t, "front", deployed.ConsumerName())
	assert.Equal(t, "accounts", deployed.ProviderName())

	assert.Equal(t, compatibility_checker.ReasonProviderResourceNotDeployedInEnvironment, nowhere.Reason)
	assert.Nil(t, nowhere.Details)
}

func TestReportAppendMergesResultsPerCounterpart(t *testing.T) {
	consumerRes := consumerResource()
	providerRes := providerResource()

	consumerChecked := compatibility_checker.NewPropertyBreakChange(
		consumerRes, providerRes, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)
	providerChecked := compatibility_checker.NewPropertyBreakChange(
		providerRes, consumerRes, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)

	report := compatibility_checker.NewContractCompatibilityReport("unknown", "unknown", "unknwonw")

	withoutIdentity := compatibility_checker.NewIncompatibleItem()
	withoutIdentity.AppendContractBreakChange(consumerChecked)
	report.AppendResult("api", withoutIdentity)

	withIdentity := compatibility_checker.NewIncompatibleItem()
	withIdentity.IncompatibleCounterpart = compatibility_checker.IncompatibleCounterpart{
		ParticipantID:      7,
		ParticipantVersion: null.StringFrom("v1"),
	}
	withIdentity.AppendContractBreakChange(providerChecked)
	report.AppendResult("api", withIdentity)

	auth := compatibility_checker.NewIncompatibleItem()
	auth.IncompatibleCounterpart = compatibility_checker.IncompatibleCounterpart{
		ParticipantID:      9,
		ParticipantVersion: null.StringFrom("v2"),
	}
	report.AppendResult("auth", auth)

	assert.Len(t, report.Results, 2)

	apiResult := report.Results["api"]
	assert.Len(t, apiResult.Breaks, 2)
	assert.False(t, apiResult.Deployable)
	assert.Equal(t, int64(7), apiResult.IncompatibleCounterpart.ParticipantID)
	assert.Equal(t, "api", apiResult.IncompatibleCounterpart.ParticipantName)
	assert.Equal(t, null.StringFrom("v1"), apiResult.IncompatibleCounterpart.ParticipantVersion)

	authResult := report.Results["auth"]
	assert.Empty(t, authResult.Breaks)
	assert.True(t, authResult.Deployable)
	assert.Equal(t, int64(9), authResult.IncompatibleCounterpart.ParticipantID)
	assert.Equal(t, "auth", authResult.IncompatibleCounterpart.ParticipantName)
	assert.Equal(t, null.StringFrom("v2"), authResult.IncompatibleCounterpart.ParticipantVersion)
}

func TestHierarchicalNodeWireKeys(t *testing.T) {
	node := compatibility_checker.Hierarchical{
		Deployable: true,
		Version:    null.StringFrom("v1"),
		Endpoints:  make(compatibility_checker.HierarchicalEndpoint),
	}

	data, err := json.Marshal(node)
	assert.NoError(t, err)

	decoded := map[string]json.RawMessage{}
	assert.NoError(t, json.Unmarshal(data, &decoded))

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}

	assert.ElementsMatch(t, []string{"deployable", "participantVersion", "endpoints"}, keys)
}

func TestHierarchicalGroupsBreaksByInteraction(t *testing.T) {
	consumerResponse := consumerResource()
	providerResponse := providerResource()
	consumerRequest := &model.PersistedResource{
		Direction:          model.Consumes,
		Interaction:        model.RestRequest,
		ConsumedProvider:   null.StringFrom("api"),
		Endpoint:           "/things",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		ParticipantName:    "front",
	}

	requestBreak := compatibility_checker.NewPropertyBreakChange(
		consumerRequest, providerResponse, compatibility_checker.ReasonPropertyMissingInProvider, "$.user",
	)
	responseBreak := compatibility_checker.NewPropertyBreakChange(
		consumerResponse, providerResponse, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	)

	item := compatibility_checker.NewIncompatibleItem()
	item.AppendContractBreakChange(requestBreak)
	item.AppendContractBreakChange(responseBreak)

	report := compatibility_checker.NewContractCompatibilityReport("front", "v1", "production")
	report.AppendResult("api", item)

	methods := report.Hierarchical["api"].Endpoints["/things"]["get"]
	assert.Len(t, methods["request"], 1)
	assert.Equal(t, compatibility_checker.ReasonPropertyMissingInProvider, methods["request"][0].Reason)
	assert.Len(t, methods["200"], 1)
	assert.Equal(t, compatibility_checker.ReasonPropertyTypeMismatch, methods["200"][0].Reason)
}

func TestHierarchicalAccumulatesAcrossAppends(t *testing.T) {
	consumerResponse := consumerResource()
	providerResponse := providerResource()

	first := compatibility_checker.NewIncompatibleItem()
	report := compatibility_checker.NewContractCompatibilityReport("front", "v1", "production")
	report.AppendResult("api", first)

	assert.True(t, report.Hierarchical["api"].Deployable)
	assert.Equal(t, null.String{}, report.Hierarchical["api"].Version)

	second := compatibility_checker.NewIncompatibleItem()
	second.IncompatibleCounterpart = compatibility_checker.IncompatibleCounterpart{
		ParticipantID:      7,
		ParticipantVersion: null.StringFrom("3.1.0"),
	}
	second.AppendContractBreakChange(compatibility_checker.NewPropertyBreakChange(
		consumerResponse, providerResponse, compatibility_checker.ReasonPropertyTypeMismatch, "$.id",
	))
	report.AppendResult("api", second)

	node := report.Hierarchical["api"]
	assert.False(t, node.Deployable)
	assert.Equal(t, null.StringFrom("3.1.0"), node.Version)

	notFound := compatibility_checker.NewIncompatibleItem()
	notFound.AppendContractBreakChange(compatibility_checker.NewProviderResourceNotFound(consumerResponse))
	report.AppendResult("ghost", notFound)

	assert.False(t, report.Hierarchical["ghost"].Deployable)
	assert.Equal(t, null.String{}, report.Hierarchical["ghost"].Version)
}
