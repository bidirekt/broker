package schemagraph_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func tagsOfTwoModules(join schemagraph.Join) schemagraph.Node {
	catalog := model.Schemas{
		"PetTagsMap": {
			"":       {Type: "object"},
			"tags":   {Type: "object"},
			"tags{}": {Type: "string"},
		},
		"PetVip": {
			"":     {Type: "object"},
			"tags": {Ref: "VipTags"},
		},
		"VipTags": {
			"":    {Type: "object"},
			"vip": {Type: "boolean"},
		},
	}

	return schemagraph.New(catalog).Root([]model.ResourceBody{
		{Source: "a.yaml", SchemaName: null.StringFrom("PetTagsMap")},
		{Source: "b.yaml", SchemaName: null.StringFrom("PetVip")},
	}, join).Child("tags")
}

func TestWithoutChild_KeepsOnlyTheMembersThatDoNotDeclareIt(t *testing.T) {
	tags := tagsOfTwoModules(schemagraph.Readers)

	withoutVip := tags.WithoutChild("vip")
	assert.Equal(t, []schemagraph.Member{{Source: "a.yaml", Type: "object"}}, withoutVip.Members())
	assert.Equal(t, "PetTagsMap.tags{}", withoutVip.Child(model.ValuesSegment).Field())

	withoutValues := tags.WithoutChild(model.ValuesSegment)
	assert.Equal(t, []schemagraph.Member{{Source: "b.yaml", Type: "object"}}, withoutValues.Members())
	assert.Equal(t, []string{"vip"}, withoutValues.Segments())
}

func TestWithoutChild_DeclaredByEveryMember_DoesNotExist(t *testing.T) {
	tags := tagsOfTwoModules(schemagraph.Senders)

	assert.False(t, tags.WithoutChild("vip").WithoutChild(model.ValuesSegment).Exists())
}

func TestWithoutChild_DeclaredByNoMember_IsTheSameNode(t *testing.T) {
	tags := tagsOfTwoModules(schemagraph.Senders)

	assert.Equal(t, tags.SiteKey(), tags.WithoutChild("name").SiteKey())
	assert.Equal(t, tags.ShapeKey(), tags.WithoutChild("name").ShapeKey())
}
