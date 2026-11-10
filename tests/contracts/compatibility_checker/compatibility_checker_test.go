package compatibility_checker_test

import (
	"encoding/json"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

var thingsHooks = []compatibility_checker.ProtocolHooks{restcontracts.New()}

func consumerResource() *model.PersistedResource {
	return &model.PersistedResource{
		Resource: model.Resource{
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Output,
			Identity:         restmodel.Identity{Endpoint: "/things", Method: "get", Status: "200"},
			ConsumedProvider: null.StringFrom("api"),
			ParticipantName:  "front",
			Bodies:           thingsBodies("FrontThing"),
		},
		Schemas: model.Schemas{"FrontThing": {
			"":     {Type: "object"},
			"id":   {Type: "integer"},
			"name": {Type: "string"},
		}},
	}
}

func providerResource() *model.PersistedResource {
	return &model.PersistedResource{
		Resource: model.Resource{
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            model.Output,
			Identity:        restmodel.Identity{Endpoint: "/things", Method: "get", Status: "200"},
			ParticipantName: "api",
			Bodies:          thingsBodies("ApiThing"),
		},
		Schemas: model.Schemas{"ApiThing": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		}},
	}
}

func thingsBodies(schemaName string) []model.ResourceBody {
	return []model.ResourceBody{{Source: "things.yaml", SchemaName: null.StringFrom(schemaName)}}
}

func thingsNode(resource *model.PersistedResource, segments ...string) schemagraph.Node {
	join := schemagraph.Readers
	if resource.Body == model.Input {
		join = schemagraph.Senders
	}

	node := schemagraph.New(resource.Schemas).Root(resource.Bodies, join)
	for _, segment := range segments {
		node = node.Child(segment)
	}

	return node
}

func thingsBreak(
	checked, counterpart *model.PersistedResource,
	reason model.BreakingReason,
	segment string,
) compatibility_checker.ContractBreakingChange {
	consumer, provider := checked, counterpart
	if !checked.IsConsumer() {
		consumer, provider = counterpart, checked
	}

	return compatibility_checker.NewPropertyBreakChange(
		checked, counterpart, reason, "$."+segment, thingsNode(consumer, segment), thingsNode(provider, segment),
	)
}

func TestProviderResourceNotFound(t *testing.T) {
	consumer := &model.PersistedResource{
		Resource: model.Resource{
			Direction:        model.Consumes,
			ConsumedProvider: null.StringFrom("accounts"),
			ParticipantName:  "front",
		},
	}

	change := compatibility_checker.NewContractBreakingChange(
		consumer,
		nil,
		model.ReasonProviderResourceNotFound,
	)

	assert.Equal(t, model.ReasonProviderResourceNotFound, change.Reason)
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
		Resource: model.Resource{
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Input,
			Identity:         restmodel.Identity{Endpoint: "/things", Method: "post"},
			ConsumedProvider: null.StringFrom("api"),
			ParticipantName:  "front",
			Bodies:           thingsBodies("FrontFlag"),
		},
		Schemas: model.Schemas{"FrontFlag": {
			"":     {Type: "object"},
			"flag": {Type: "boolean", Optional: true},
		}},
	}
	providerRequest := &model.PersistedResource{
		Resource: model.Resource{
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            model.Input,
			Identity:        restmodel.Identity{Endpoint: "/things", Method: "post"},
			ParticipantName: "api",
			Bodies:          thingsBodies("ApiFlag"),
		},
		Schemas: model.Schemas{"ApiFlag": {
			"":     {Type: "object"},
			"user": {Type: "string"},
			"flag": {Type: "boolean"},
		}},
	}

	typeMismatch := thingsBreak(consumerResponse, providerResponse, model.ReasonPropertyTypeMismatch, "id")
	missingInProvider := thingsBreak(consumerResponse, providerResponse, model.ReasonPropertyMissingInProvider, "name")
	optionalInProvider := thingsBreak(consumerResponse, providerResponse, model.ReasonPropertyOptionalInProviderRequiredInConsumer, "id")
	missingInConsumer := thingsBreak(consumerRequest, providerRequest, model.ReasonPropertyMissingInConsumer, "user")
	optionalInConsumer := thingsBreak(consumerRequest, providerRequest, model.ReasonPropertyOptionalInConsumerRequiredInProvider, "flag")

	assert.Equal(t, model.ReasonPropertyTypeMismatch, typeMismatch.Reason)
	assert.Equal(t, model.ReasonPropertyMissingInProvider, missingInProvider.Reason)
	assert.Equal(t, model.ReasonPropertyOptionalInProviderRequiredInConsumer, optionalInProvider.Reason)
	assert.Equal(t, model.ReasonPropertyMissingInConsumer, missingInConsumer.Reason)
	assert.Equal(t, model.ReasonPropertyOptionalInConsumerRequiredInProvider, optionalInConsumer.Reason)

	assert.Equal(t, map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
		"consumerField":        "FrontThing.id",
		"providerField":        "ApiThing.id",
	}, typeMismatch.Details)

	assert.Equal(t, map[string]string{"property": "$.name", "consumerName": "front", "providerName": "api", "propertyType": "string", "consumerField": "FrontThing.name"}, missingInProvider.Details)
	assert.Equal(t, map[string]string{"property": "$.id", "consumerName": "front", "providerName": "api", "propertyType": "integer", "consumerField": "FrontThing.id", "providerField": "ApiThing.id"}, optionalInProvider.Details)
	assert.Equal(t, map[string]string{"property": "$.user", "consumerName": "front", "providerName": "api", "propertyType": "string", "providerField": "ApiFlag.user"}, missingInConsumer.Details)
	assert.Equal(t, map[string]string{"property": "$.flag", "consumerName": "front", "providerName": "api", "propertyType": "boolean", "consumerField": "FrontFlag.flag", "providerField": "ApiFlag.flag"}, optionalInConsumer.Details)

	assert.Equal(t, "front", typeMismatch.ConsumerName())
	assert.Equal(t, "api", typeMismatch.ProviderName())
	assert.Equal(t, "front", missingInConsumer.ConsumerName())
	assert.Equal(t, "api", missingInConsumer.ProviderName())
}

func TestPropertyTypeRendersArrayItemType(t *testing.T) {
	consumer := consumerResource()
	consumer.Schemas = model.Schemas{"FrontThing": {
		"":          {Type: "object"},
		"list":      {Type: "array"},
		"list[]":    {Type: "object"},
		"list[].id": {Type: "string"},
		"tags":      {Type: "array"},
		"tags[]":    {Type: "array"},
		"tags[][]":  {Type: "integer"},
		"other":     {Type: "array"},
		"other[]":   {Type: "string"},
	}}
	provider := providerResource()
	provider.Schemas = model.Schemas{"ApiThing": {
		"":      {Type: "object"},
		"other": {Type: "array"},
	}}

	missingList := thingsBreak(consumer, provider, model.ReasonPropertyMissingInProvider, "list")
	missingTags := thingsBreak(consumer, provider, model.ReasonPropertyMissingInProvider, "tags")
	mismatch := thingsBreak(consumer, provider, model.ReasonPropertyTypeMismatch, "other")

	assert.Equal(t, "array<object>", missingList.Details["propertyType"])
	assert.Equal(t, "array<array<integer>>", missingTags.Details["propertyType"])
	assert.Equal(t, "array<string>", mismatch.Details["consumerPropertyType"])
}

func TestPropertyTypeRendersMapValueType(t *testing.T) {
	consumer := consumerResource()
	consumer.Schemas = model.Schemas{"FrontThing": {
		"":            {Type: "object"},
		"pets":        {Type: "map"},
		"pets{}":      {Type: "object"},
		"pets{}.name": {Type: "string"},
		"tags":        {Type: "map"},
		"tags{}":      {Type: "array"},
		"tags{}[]":    {Type: "string"},
		"owner":       {Type: "object"},
		"owner.name":  {Type: "string"},
		"scores":      {Type: "map"},
		"scores{}":    {Type: "integer"},
	}}
	provider := providerResource()
	provider.Schemas = model.Schemas{"ApiThing": {
		"":         {Type: "object"},
		"scores":   {Type: "array"},
		"scores[]": {Type: "integer"},
	}}

	missingPets := thingsBreak(consumer, provider, model.ReasonPropertyMissingInProvider, "pets")
	missingTags := thingsBreak(consumer, provider, model.ReasonPropertyMissingInProvider, "tags")
	missingOwner := thingsBreak(consumer, provider, model.ReasonPropertyMissingInProvider, "owner")
	mismatch := thingsBreak(consumer, provider, model.ReasonPropertyTypeMismatch, "scores")

	assert.Equal(t, "map<object>", missingPets.Details["propertyType"])
	assert.Equal(t, "map<array<string>>", missingTags.Details["propertyType"])
	assert.Equal(t, "object", missingOwner.Details["propertyType"])
	assert.Equal(t, "map<integer>", mismatch.Details["consumerPropertyType"])
	assert.Equal(t, "array<integer>", mismatch.Details["providerPropertyType"])
}

func TestTypeMismatchTypesResolvedByRole(t *testing.T) {
	consumerRes := consumerResource()
	providerRes := providerResource()

	consumerChecked := thingsBreak(consumerRes, providerRes, model.ReasonPropertyTypeMismatch, "id")
	providerChecked := thingsBreak(providerRes, consumerRes, model.ReasonPropertyTypeMismatch, "id")

	expected := map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
		"consumerField":        "FrontThing.id",
		"providerField":        "ApiThing.id",
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
	item := compatibility_checker.NewCounterpartResult()
	item.AppendContractBreakChange(thingsBreak(consumerResource(), providerResource(), model.ReasonPropertyTypeMismatch, "id"))

	report := compatibility_checker.NewContractCompatibilityReport("front", "v1", "production", thingsHooks)
	report.AppendResult("api", item)

	data, err := json.Marshal(report.Hierarchical["api"].Trees[restmodel.Protocol].(restmodel.ReportTree)["/things"]["get"]["200"][0])
	assert.NoError(t, err)

	decoded := map[string]json.RawMessage{}
	assert.NoError(t, json.Unmarshal(data, &decoded))

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}

	assert.ElementsMatch(t, []string{"reason", "role", "details"}, keys)
}

func TestProviderNotDeployedHasNoDetails(t *testing.T) {
	consumer := &model.PersistedResource{
		Resource: model.Resource{
			Direction:        model.Consumes,
			ConsumedProvider: null.StringFrom("accounts"),
			ParticipantName:  "front",
		},
	}

	notDeployed := compatibility_checker.NewProviderResourceNotDeployed(consumer)

	assert.Equal(t, model.ReasonProviderResourceNotDeployedInEnvironment, notDeployed.Reason)
	assert.Nil(t, notDeployed.Details)
	assert.Equal(t, "front", notDeployed.ConsumerName())
	assert.Equal(t, "accounts", notDeployed.ProviderName())
}

func TestReportAppendMergesResultsPerCounterpart(t *testing.T) {
	consumerRes := consumerResource()
	providerRes := providerResource()

	consumerChecked := thingsBreak(consumerRes, providerRes, model.ReasonPropertyTypeMismatch, "id")
	providerChecked := thingsBreak(providerRes, consumerRes, model.ReasonPropertyTypeMismatch, "id")

	report := compatibility_checker.NewContractCompatibilityReport("unknown", "unknown", "unknwonw", thingsHooks)

	withoutIdentity := compatibility_checker.NewCounterpartResult()
	withoutIdentity.AppendContractBreakChange(consumerChecked)
	report.AppendResult("api", withoutIdentity)

	withIdentity := compatibility_checker.NewCounterpartResult()
	withIdentity.Counterpart = compatibility_checker.Counterpart{
		ParticipantID:      7,
		ParticipantVersion: null.StringFrom("v1"),
	}
	withIdentity.AppendContractBreakChange(providerChecked)
	report.AppendResult("api", withIdentity)

	auth := compatibility_checker.NewCounterpartResult()
	auth.Counterpart = compatibility_checker.Counterpart{
		ParticipantID:      9,
		ParticipantVersion: null.StringFrom("v2"),
	}
	report.AppendResult("auth", auth)

	assert.Len(t, report.Results, 2)

	apiResult := report.Results["api"]
	assert.Len(t, apiResult.Breaks, 2)
	assert.False(t, apiResult.Deployable)
	assert.Equal(t, int64(7), apiResult.Counterpart.ParticipantID)
	assert.Equal(t, "api", apiResult.Counterpart.ParticipantName)
	assert.Equal(t, null.StringFrom("v1"), apiResult.Counterpart.ParticipantVersion)

	authResult := report.Results["auth"]
	assert.Empty(t, authResult.Breaks)
	assert.True(t, authResult.Deployable)
	assert.Equal(t, int64(9), authResult.Counterpart.ParticipantID)
	assert.Equal(t, "auth", authResult.Counterpart.ParticipantName)
	assert.Equal(t, null.StringFrom("v2"), authResult.Counterpart.ParticipantVersion)
}

func TestHierarchicalNodeWireKeys(t *testing.T) {
	node := compatibility_checker.Hierarchical{
		Deployable: true,
		Version:    null.StringFrom("v1"),
		Trees:      map[model.Protocol]compatibility_checker.ReportTree{restmodel.Protocol: restmodel.ReportTree{}},
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

func TestHierarchicalMarshalsFieldsInWireOrder(t *testing.T) {
	compatible := compatibility_checker.Hierarchical{
		Deployable: true,
		Trees:      map[model.Protocol]compatibility_checker.ReportTree{restmodel.Protocol: restmodel.ReportTree{}},
	}
	broken := compatibility_checker.Hierarchical{
		Version: null.StringFrom("v1"),
		Trees: map[model.Protocol]compatibility_checker.ReportTree{restmodel.Protocol: restmodel.ReportTree{
			"/things": {"get": {"200": {{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer}}}},
		}},
	}

	compatibleData, err := json.Marshal(compatible)
	assert.NoError(t, err)
	brokenData, err := json.Marshal(broken)
	assert.NoError(t, err)

	assert.Equal(t, `{"deployable":true,"participantVersion":null,"endpoints":{}}`, string(compatibleData))
	assert.Equal(t, `{"deployable":false,"participantVersion":"v1","endpoints":{"/things":{"get":{"200":[{"reason":"property_missing_in_provider","role":"consumer"}]}}}}`, string(brokenData))
}

func TestHierarchicalGroupsBreaksByInteraction(t *testing.T) {
	consumerResponse := consumerResource()
	providerResponse := providerResource()
	consumerRequest := &model.PersistedResource{
		Resource: model.Resource{
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Input,
			Identity:         restmodel.Identity{Endpoint: "/things", Method: "get"},
			ConsumedProvider: null.StringFrom("api"),
			ParticipantName:  "front",
		},
	}

	requestBreak := thingsBreak(consumerRequest, providerResponse, model.ReasonPropertyMissingInProvider, "user")
	responseBreak := thingsBreak(consumerResponse, providerResponse, model.ReasonPropertyTypeMismatch, "id")

	item := compatibility_checker.NewCounterpartResult()
	item.AppendContractBreakChange(requestBreak)
	item.AppendContractBreakChange(responseBreak)

	report := compatibility_checker.NewContractCompatibilityReport("front", "v1", "production", thingsHooks)
	report.AppendResult("api", item)

	methods := report.Hierarchical["api"].Trees[restmodel.Protocol].(restmodel.ReportTree)["/things"]["get"]
	assert.Len(t, methods["request"], 1)
	assert.Equal(t, model.ReasonPropertyMissingInProvider, methods["request"][0].Reason)
	assert.Len(t, methods["200"], 1)
	assert.Equal(t, model.ReasonPropertyTypeMismatch, methods["200"][0].Reason)
}

func TestHierarchicalAccumulatesAcrossAppends(t *testing.T) {
	consumerResponse := consumerResource()
	providerResponse := providerResource()

	first := compatibility_checker.NewCounterpartResult()
	report := compatibility_checker.NewContractCompatibilityReport("front", "v1", "production", thingsHooks)
	report.AppendResult("api", first)

	assert.True(t, report.Hierarchical["api"].Deployable)
	assert.Equal(t, null.String{}, report.Hierarchical["api"].Version)

	second := compatibility_checker.NewCounterpartResult()
	second.Counterpart = compatibility_checker.Counterpart{
		ParticipantID:      7,
		ParticipantVersion: null.StringFrom("3.1.0"),
	}
	second.AppendContractBreakChange(thingsBreak(consumerResponse, providerResponse, model.ReasonPropertyTypeMismatch, "id"))
	report.AppendResult("api", second)

	node := report.Hierarchical["api"]
	assert.False(t, node.Deployable)
	assert.Equal(t, null.StringFrom("3.1.0"), node.Version)

	notFound := compatibility_checker.NewCounterpartResult()
	notFound.AppendContractBreakChange(compatibility_checker.NewProviderResourceNotFound(consumerResponse))
	report.AppendResult("ghost", notFound)

	assert.False(t, report.Hierarchical["ghost"].Deployable)
	assert.Equal(t, null.String{}, report.Hierarchical["ghost"].Version)
}
