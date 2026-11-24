package graphql

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func (this Plugin) Rules() []protocol.Rule {
	return []protocol.Rule{scalarsOutsideGraphQL, argumentsOutsideGraphQL}
}

func scalarsOutsideGraphQL(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, reached := range schemasReachedOutsideGraphQL(declarations) {
		if reached.schema.Type == model.ScalarType {
			violations = append(violations, reached.violation("schema.scalar_not_allowed", "type"))
		}
	}

	return violations
}

func argumentsOutsideGraphQL(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, reached := range schemasReachedOutsideGraphQL(declarations) {
		if reached.schema.Arguments != "" {
			violations = append(violations, reached.violation("schema.arguments_not_allowed", argumentsKey))
		}
	}

	return violations
}

// reachedSchema is a schema, or one nested in it, where it is written, and the first protocol whose body reaches it.
type reachedSchema struct {
	source       string
	documentPath string
	schema       contract.Schema
	protocol     model.Protocol
}

func (this reachedSchema) violation(errorCode, key string) violation.Violation {
	return violation.Violation{
		ErrorCode: errorCode,
		Path:      this.documentPath + ";" + key,
		Source:    this.source,
		Details:   map[string]string{"protocol": this.protocol.String()},
	}
}

// Each schema is walked once, so a schema many bodies reach is reported once, where it is written.
func schemasReachedOutsideGraphQL(declarations protocol.Declarations) []reachedSchema {
	sources := map[[2]string]string{}
	for _, declaration := range declarations.Schemas {
		key := [2]string{declaration.Namespace, declaration.Name}
		if _, declared := sources[key]; !declared {
			sources[key] = declaration.Source
		}
	}

	visited := map[[2]string]bool{}

	var reached []reachedSchema

	var visit func(namespace, name string, reachedBy model.Protocol)
	var walk func(schema contract.Schema, namespace, source, documentPath string, reachedBy model.Protocol)

	visit = func(namespace, name string, reachedBy model.Protocol) {
		key := [2]string{namespace, name}
		schema, declared := declarations.Catalog[namespace][name]
		if visited[key] || !declared {
			return
		}

		visited[key] = true
		walk(schema, namespace, sources[key], "schemas;"+name, reachedBy)
	}

	walk = func(schema contract.Schema, namespace, source, documentPath string, reachedBy model.Protocol) {
		reached = append(reached, reachedSchema{source: source, documentPath: documentPath, schema: schema, protocol: reachedBy})

		if schema.IsRef() {
			visit(namespace, schema.Ref, reachedBy)
		}

		for _, child := range schema.Children() {
			walk(child.Schema, namespace, source, documentPath+";"+child.DocumentPath, reachedBy)
		}
	}

	for _, declaration := range declarations.Resources {
		if declaration.Resource.Protocol != graphqlmodel.Protocol && !declaration.WithoutSchema {
			visit(declaration.Namespace, declaration.SchemaName, declaration.Resource.Protocol)
		}
	}

	return reached
}
