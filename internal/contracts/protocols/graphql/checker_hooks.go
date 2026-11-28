package graphql

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func (this Plugin) NewReportTree() compatibility_checker.ReportTree {
	return graphqlmodel.ReportTree{}
}

var bodyReadings = map[model.BodyRole]compatibility_checker.BodyReading{
	model.Input: {
		Rules: compatibility_checker.ReadingRules{
			ReaderIsConsumer: false,
			Missing:          model.ReasonPropertyMissingInConsumer,
			Optional:         model.ReasonPropertyOptionalInConsumerRequiredInProvider,
			Nullable:         model.ReasonPropertyNullableInConsumerNotNullableInProvider,
			Unknown:          model.ReasonPropertyMissingInProvider,
			SingleToList:     true,
			Widenings: []compatibility_checker.Widening{
				{Reader: compatibility_checker.SchemaType{Type: model.FloatType}, Producer: compatibility_checker.SchemaType{Type: model.IntegerType}},
				{Reader: compatibility_checker.SchemaType{Type: model.FloatType}, Producer: compatibility_checker.SchemaType{Type: model.NumberType}},
				{Reader: compatibility_checker.SchemaType{Type: model.StringType, Scalar: "ID"}, Producer: compatibility_checker.SchemaType{Type: model.IntegerType}},
			},
		},
		Join:           schemagraph.Senders,
		FieldArguments: true,
	},
	model.Output: {
		Rules: compatibility_checker.ReadingRules{
			ReaderIsConsumer:  true,
			Missing:           model.ReasonPropertyMissingInProvider,
			Optional:          model.ReasonPropertyOptionalInProviderRequiredInConsumer,
			Nullable:          model.ReasonPropertyNullableInProviderNotNullableInConsumer,
			OptionalMustExist: true,
			ImplicitFields:    []string{"__typename"},
		},
		Join: schemagraph.Readers,
	},
}

func (this Plugin) BodyReading(body model.BodyRole) compatibility_checker.BodyReading {
	return bodyReadings[body]
}

// A site of a synthesized schema, or of a root field's own body schema, is named by GraphQL coordinates: an argument
// "Query.pet(id:)", "Pet.reviews(first:)[]", what a field returns "Query.pet", "Query.pets[]", and the arguments as a
// whole by their field, "Query.pet". A field the consumer's root schema declares keeps the schema's name, "PetRead.name".
func (this Plugin) FieldName(resource *model.PersistedResource, site schemagraph.Site) (string, bool) {
	typeName, field, interaction, named := siteCoordinate(resource, site)
	if !named {
		return "", false
	}

	if interaction == graphqlmodel.ReturnsInteraction || site.Path == "" {
		return model.FieldName(typeName+"."+field, site.Path), true
	}

	argumentEnd := strings.IndexAny(site.Path, ".[{")
	if argumentEnd == -1 {
		argumentEnd = len(site.Path)
	}

	return typeName + "." + field + "(" + site.Path[:argumentEnd] + ":)" + site.Path[argumentEnd:], true
}

func siteCoordinate(resource *model.PersistedResource, site schemagraph.Site) (typeName, field, interaction string, named bool) {
	if typeName, field, interaction, synthesized := synthesizedSchemaNameToField(site.Schema); synthesized && site.Namespace == graphqlmodel.Namespace {
		return typeName, field, interaction, true
	}

	identity, rootField := resource.Identity.(graphqlmodel.Identity)
	if !rootField || !isBodySchema(resource, site) || (identity.Interaction == graphqlmodel.ReturnsInteraction && namesField(site.Path)) {
		return "", "", "", false
	}

	root := rootTypes[slices.IndexFunc(rootTypes, func(root rootType) bool { return root.operation == identity.Operation })]

	return root.typeName, identity.Field, identity.Interaction, true
}

func isBodySchema(resource *model.PersistedResource, site schemagraph.Site) bool {
	return slices.ContainsFunc(resource.Bodies, func(body model.ResourceBody) bool {
		return body.SchemaName.Valid && body.Namespace == site.Namespace && body.SchemaName.String == site.Schema
	})
}

func namesField(path string) bool {
	for parent, segment, nested := model.ParentOf(path); nested; parent, segment, nested = model.ParentOf(parent) {
		if segment != model.ItemsSegment && segment != model.ValuesSegment && !model.IsMemberSegment(segment) {
			return true
		}
	}

	return false
}

var builtInScalarNames = toBuiltInScalarNames()

func toBuiltInScalarNames() map[string]string {
	names := map[string]string{}
	for name, schema := range builtInScalarSchemas {
		if schema.Scalar == "" {
			names[schema.Type] = name
		}
	}

	// GraphQL has no number: Float is the type a number reads.
	names[model.NumberType] = names[model.FloatType]

	return names
}

var namedShapeTypes = []string{model.ObjectType, model.UnionType, model.MapType, model.EnumType}

// TypeName writes the type as GraphQL does, "[Review!]!", and an enum, union or map without a schema name by its
// values, "enum (AVAILABLE | SOLD)!": an item or value of a shape met again is written by its kind, so a
// self-referencing list ends.
func (this Plugin) TypeName(node schemagraph.Node) (string, bool) {
	return graphqlTypeName(node, map[string]bool{}), true
}

func graphqlTypeName(node schemagraph.Node, writing map[string]bool) string {
	name := node.Type()

	switch shape := node.ShapeKey(); {
	case writing[shape]:
	case node.Type() == model.ArrayType:
		writing[shape] = true
		name = "[" + graphqlTypeName(node.Child(model.ItemsSegment), writing) + "]"
	case node.Scalar() != "":
		name = node.Scalar()
	case builtInScalarNames[node.Type()] != "":
		name = builtInScalarNames[node.Type()]
	case slices.Contains(namedShapeTypes, node.Type()) && node.ShapeName() != "":
		name = node.ShapeName()
	case node.Type() == model.EnumType:
		name = model.EnumType + " (" + strings.Join(node.Members(), " | ") + ")"
	case node.Type() == model.UnionType:
		name = model.UnionType + " (" + strings.Join(memberKeys(node), " | ") + ")"
	case node.Type() == model.MapType:
		writing[shape] = true
		name = model.MapType + "<" + graphqlTypeName(node.Child(model.ValuesSegment), writing) + ">"
	}

	if !node.Nullable() {
		name += "!"
	}

	return name
}

func memberKeys(union schemagraph.Node) []string {
	var keys []string
	for _, segment := range union.Segments() {
		if model.IsMemberSegment(segment) {
			keys = append(keys, model.MemberKey(segment))
		}
	}

	return keys
}
