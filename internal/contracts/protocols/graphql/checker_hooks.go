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
		},
		Join:           schemagraph.Senders,
		FieldArguments: true,
	},
	model.Output: {
		Rules: compatibility_checker.ReadingRules{
			ReaderIsConsumer: true,
			Missing:          model.ReasonPropertyMissingInProvider,
			Optional:         model.ReasonPropertyOptionalInProviderRequiredInConsumer,
			Nullable:         model.ReasonPropertyNullableInProviderNotNullableInConsumer,
		},
		Join: schemagraph.Readers,
	},
}

func (this Plugin) BodyReading(body model.BodyRole) compatibility_checker.BodyReading {
	return bodyReadings[body]
}

// An argument of a schema of arguments is named by its GraphQL coordinate: "Query.pet(id:)", "Pet.reviews(first:)[]".
func (this Plugin) FieldName(site schemagraph.Site) (string, bool) {
	typeName, field, ofArguments := argumentsSchemaNameToField(site.Schema)
	if site.Namespace != graphqlmodel.Namespace || !ofArguments || site.Path == "" {
		return "", false
	}

	argumentEnd := strings.IndexAny(site.Path, ".[{")
	if argumentEnd == -1 {
		argumentEnd = len(site.Path)
	}

	return typeName + "." + field + "(" + site.Path[:argumentEnd] + ":)" + site.Path[argumentEnd:], true
}
