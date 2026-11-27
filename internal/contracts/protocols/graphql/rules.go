package graphql

import (
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func (this Plugin) Rules() []protocol.Rule {
	return []protocol.Rule{graphQLKeysOutsideGraphQL, argumentsNotObject}
}

func graphQLKeysOutsideGraphQL(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, reached := range schemasWithGraphQLKeys(declarations, func(reachedBy model.Protocol) bool { return reachedBy != graphqlmodel.Protocol }) {
		if reached.scalar {
			violations = append(violations, reached.violation("schema.scalar_not_allowed", "type"))
		}

		if reached.arguments != "" {
			violations = append(violations, reached.violation("schema.arguments_not_allowed", argumentsKey))
		}

		if reached.unionProperties {
			violations = append(violations, unionPropertiesNotAllowed(reached))
		}
	}

	return violations
}

// Outside GraphQL a union reads no fields of its own, so it is refused as the core grammar refuses a key of another type.
func unionPropertiesNotAllowed(reached reachedSchema) violation.Violation {
	return violation.Violation{
		ErrorCode: "schema.key_not_allowed",
		Path:      reached.documentPath,
		Source:    reached.source,
		Details:   map[string]any{"key": "properties", "type": model.UnionType},
	}
}

// A body of arguments is compared argument by argument, never as a whole, so it must name a non-nullable object.
func argumentsNotObject(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Resources {
		if declaration.Resource.Protocol != graphqlmodel.Protocol || declaration.Resource.Body != model.Input || declaration.WithoutSchema {
			continue
		}

		if details, refused := toArgumentsRefusal(declarations.Catalog[declaration.Namespace], declaration.SchemaName); refused {
			violations = append(violations, argumentsNotObjectViolation(declaration.Path.String(), declaration.Source, details))
		}
	}

	for _, reached := range schemasWithGraphQLKeys(declarations, func(reachedBy model.Protocol) bool { return reachedBy == graphqlmodel.Protocol }) {
		if reached.arguments == "" {
			continue
		}

		if details, refused := toArgumentsRefusal(declarations.Catalog[reached.namespace], reached.arguments); refused {
			violations = append(violations, argumentsNotObjectViolation(reached.documentPath+";"+argumentsKey, reached.source, details))
		}
	}

	return violations
}

// The refs from name end on the schema of the arguments; a nullable link anywhere on the way makes them nullable.
// An undeclared name is left to the unresolved ref rules.
func toArgumentsRefusal(catalog contract.SchemasMap, name string) (map[string]any, bool) {
	nullable := false
	followed := map[string]bool{}

	for current := name; !followed[current]; {
		schema, declared := catalog[current]
		if !declared {
			return nil, false
		}

		followed[current] = true
		nullable = nullable || schema.Nullable

		if schema.IsRef() {
			current = schema.Ref

			continue
		}

		if schema.Type != model.ObjectType {
			return map[string]any{"schema": name, "reason": "not_object", "type": schema.Type}, true
		}

		if nullable {
			return map[string]any{"schema": name, "reason": "nullable"}, true
		}

		return nil, false
	}

	return nil, false
}

func argumentsNotObjectViolation(path, source string, details map[string]any) violation.Violation {
	return violation.Violation{ErrorCode: "schema.arguments_not_object", Path: path, Source: source, Details: details}
}

// reachedSchema is a schema, or one nested in it, holding a GraphQL key, where it is written, and the first protocol whose body reaches it.
type reachedSchema struct {
	source          string
	namespace       string
	documentPath    string
	scalar          bool
	arguments       string
	unionProperties bool
	protocol        model.Protocol
}

func (this reachedSchema) violation(errorCode, key string) violation.Violation {
	return violation.Violation{
		ErrorCode: errorCode,
		Path:      this.documentPath + ";" + key,
		Source:    this.source,
		Details:   map[string]any{"protocol": this.protocol.String()},
	}
}

// Each schema a body of an accepted protocol reaches is walked once, so a schema many bodies reach is reported once,
// where it is written. Only what holds a GraphQL key is kept: the schemas of another protocol mostly hold none.
func schemasWithGraphQLKeys(declarations protocol.Declarations, reaches func(model.Protocol) bool) []reachedSchema {
	walk := graphQLKeysWalk{declarations: declarations, sources: map[[2]string]string{}, visited: map[[2]string]bool{}}
	for _, declaration := range declarations.Schemas {
		key := [2]string{declaration.Namespace, declaration.Name}
		if _, declared := walk.sources[key]; !declared {
			walk.sources[key] = declaration.Source
		}
	}

	for _, declaration := range declarations.Resources {
		if reaches(declaration.Resource.Protocol) && !declaration.WithoutSchema {
			walk.visit(declaration.Namespace, declaration.SchemaName, declaration.Resource.Protocol)
		}
	}

	return walk.reached
}

type graphQLKeysWalk struct {
	declarations protocol.Declarations
	sources      map[[2]string]string
	visited      map[[2]string]bool
	reached      []reachedSchema
}

func (this *graphQLKeysWalk) visit(namespace, name string, reachedBy model.Protocol) {
	key := [2]string{namespace, name}
	schema, declared := this.declarations.Catalog[namespace][name]
	if this.visited[key] || !declared {
		return
	}

	this.visited[key] = true
	this.walk(schema, namespace, this.sources[key], append(make([]string, 0, 16), "schemas", name), reachedBy)
}

// Children is not used: it sorts and copies every child, and this walk runs on every contract, of every protocol.
func (this *graphQLKeysWalk) walk(schema contract.Schema, namespace, source string, documentPath []string, reachedBy model.Protocol) {
	unionProperties := schema.IsUnion() && schema.Properties != nil
	if schema.Type == model.ScalarType || schema.Arguments != "" || unionProperties {
		this.reached = append(this.reached, reachedSchema{
			source:          source,
			namespace:       namespace,
			documentPath:    strings.Join(documentPath, ";"),
			scalar:          schema.Type == model.ScalarType,
			arguments:       schema.Arguments,
			unionProperties: unionProperties,
			protocol:        reachedBy,
		})
	}

	if schema.IsRef() {
		this.visit(namespace, schema.Ref, reachedBy)
	}

	for name, property := range schema.Properties {
		this.walk(property, namespace, source, append(documentPath, "properties", name), reachedBy)
	}

	for key, member := range schema.UnionMembers {
		this.walk(member, namespace, source, append(documentPath, "members", key), reachedBy)
	}

	if schema.Items != nil {
		this.walk(*schema.Items, namespace, source, append(documentPath, "items"), reachedBy)
	}

	if schema.Values != nil {
		this.walk(*schema.Values, namespace, source, append(documentPath, "values"), reachedBy)
	}
}
