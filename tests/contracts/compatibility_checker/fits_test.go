package compatibility_checker_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

var fitsRules = compatibility_checker.ReadingRules{
	Missing:      model.ReasonPropertyMissingInConsumer,
	Optional:     model.ReasonPropertyOptionalInConsumerRequiredInProvider,
	Nullable:     model.ReasonPropertyNullableInConsumerNotNullableInProvider,
	Unknown:      model.ReasonPropertyMissingInProvider,
	SingleToList: true,
}

var fitsConsumed = &model.PersistedResource{
	Resource: model.Resource{
		ParticipantName:  "petstore_web",
		Direction:        model.Consumes,
		Body:             model.Input,
		ConsumedProvider: null.StringFrom("petstore_api"),
	},
}

func fitsNode(schema model.Schema, segments ...string) schemagraph.Node {
	bodies := []model.ResourceBody{{Source: "contract.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Args")}}

	node := schemagraph.New(map[string]model.Schemas{model.YAMLNamespace: {"Args": schema}}).Root(bodies, schemagraph.Senders)
	for _, segment := range segments {
		node = node.Child(segment)
	}

	return node
}

func fits(consumer, provider model.Schema, segments ...string) bool {
	return compatibility_checker.Fits(fitsConsumed, fitsNode(consumer, segments...), fitsNode(provider, segments...), fitsRules)
}

func TestFits_ASiteThatMeetsTheProvider_Fits(t *testing.T) {
	assert.True(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.StringType}},
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.StringType}},
		"id",
	))
}

func TestFits_AValueOfAnotherType_DoesNotFit(t *testing.T) {
	assert.False(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.StringType}},
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.IntegerType}},
		"id",
	))
}

func TestFits_AListWhoseItemsAreOfAnotherType_DoesNotFit(t *testing.T) {
	assert.False(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "ids": {Type: model.ArrayType}, "ids[]": {Type: model.StringType}},
		model.Schema{"": {Type: model.ObjectType}, "ids": {Type: model.ArrayType}, "ids[]": {Type: model.IntegerType}},
		"ids",
	))
}

func TestFits_ANullableValueSentToANonNullableSite_DoesNotFit(t *testing.T) {
	assert.False(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.StringType, Nullable: true}},
		model.Schema{"": {Type: model.ObjectType}, "id": {Type: model.StringType}},
		"id",
	))
}

func TestFits_AnEnumValueTheProviderLacks_DoesNotFit(t *testing.T) {
	assert.False(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "status": {Type: model.EnumType, Members: []string{"AVAILABLE"}}},
		model.Schema{"": {Type: model.ObjectType}, "status": {Type: model.EnumType, Members: []string{"SOLD"}}},
		"status",
	))
}

func TestFits_ASingleValueSentToAList_Fits(t *testing.T) {
	assert.True(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "ids": {Type: model.StringType}},
		model.Schema{"": {Type: model.ObjectType}, "ids": {Type: model.ArrayType}, "ids[]": {Type: model.StringType}},
		"ids",
	))
}

func TestFits_ABodyWithABreakDeepInside_DoesNotFit(t *testing.T) {
	assert.False(t, fits(
		model.Schema{"": {Type: model.ObjectType}, "owner": {Type: model.ObjectType}, "owner.id": {Type: model.StringType}},
		model.Schema{"": {Type: model.ObjectType}, "owner": {Type: model.ObjectType}, "owner.id": {Type: model.IntegerType}},
	))
}
