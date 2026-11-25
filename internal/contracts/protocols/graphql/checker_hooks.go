package graphql

import (
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

// A root field starting with "__" is an introspection field, which every GraphQL schema serves without declaring it.
func (this Plugin) ProvidesImplicitly(identity model.ResourceIdentity) bool {
	return strings.HasPrefix(identity.(graphqlmodel.Identity).Field, "__")
}

func (this Plugin) BodyReading(body model.BodyRole) compatibility_checker.BodyReading {
	return bodyReadings[body]
}

// A site of a synthesized schema is named by GraphQL coordinates: an argument "Query.pet(id:)", "Pet.reviews(first:)[]",
// what a field returns "Query.pet", "Query.pets[]", and the arguments as a whole by their field, "Query.pet".
func (this Plugin) FieldName(site schemagraph.Site) (string, bool) {
	typeName, field, interaction, synthesized := synthesizedSchemaNameToField(site.Schema)
	if site.Namespace != graphqlmodel.Namespace || !synthesized {
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
