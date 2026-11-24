package graphql

import (
	"maps"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/guregu/null"
)

// An ID is serialized as a string, but as an input it also takes an integer, so it keeps its name.
var builtInScalarSchemas = map[string]contract.Schema{
	"ID":      {Type: model.StringType, Scalar: "ID"},
	"String":  {Type: model.StringType},
	"Int":     {Type: model.IntegerType},
	"Float":   {Type: model.FloatType},
	"Boolean": {Type: model.BooleanType},
}

var definitionKinds = map[ast.DefinitionKind]string{
	ast.Object:      graphqlmodel.ObjectKind,
	ast.InputObject: graphqlmodel.InputKind,
	ast.Interface:   graphqlmodel.InterfaceKind,
	ast.Union:       graphqlmodel.UnionKind,
	ast.Enum:        graphqlmodel.EnumKind,
	ast.Scalar:      graphqlmodel.ScalarKind,
}

type sdlDeclarations struct {
	schema       *ast.Schema
	directives   map[string]string
	declarations protocol.Declarations
	pieces       map[string]*graphqlmodel.Extension
}

// Every type becomes a schema kept in the contract, whether a root field reaches it or not.
func toDeclarations(schema *ast.Schema, federation federation) protocol.Declarations {
	converted := &sdlDeclarations{
		schema:       schema,
		directives:   federation.countedDirectives(),
		declarations: protocol.Declarations{Formats: []string{formatName}},
		pieces:       map[string]*graphqlmodel.Extension{},
	}

	if federation.linkSource != "" {
		converted.pieceOf(federation.linkSource).LinkSource = federation.linkSource
	}

	for _, name := range slices.Sorted(maps.Keys(schema.Types)) {
		definition := schema.Types[name]
		if definition.BuiltIn {
			continue
		}

		converted.addDefinition(definition)
	}

	converted.addRootFields(schema.Query, graphqlmodel.QueryOperation)

	// One type may be both roots; its fields are then resources once, as queries.
	if schema.Mutation != schema.Query {
		converted.addRootFields(schema.Mutation, graphqlmodel.MutationOperation)
	}

	converted.addExtensionPieces()

	return converted.declarations
}

func (this *sdlDeclarations) addDefinition(definition *ast.Definition) {
	this.addSchema(definition.Position, definition.Name, this.toDefinitionSchema(definition))
	this.addTypeInfo(definition.Position, definition.Name, definitionKinds[definition.Kind], "")
	this.addDirectives(definition.Name, definition.Directives)

	for _, value := range definition.EnumValues {
		this.addDirectives(definition.Name+"."+value.Name, value.Directives)
	}

	for _, field := range declaredFields(definition) {
		coordinate := definition.Name + "." + field.Name
		this.addTypeInfo(field.Position, coordinate, graphqlmodel.FieldKind, field.Type.Name())
		this.addDirectives(coordinate, field.Directives)

		for _, argument := range field.Arguments {
			argumentCoordinate := coordinate + "(" + argument.Name + ":)"
			this.addTypeInfo(argument.Position, argumentCoordinate, graphqlmodel.ArgumentKind, argument.Type.Name())
			this.addDirectives(argumentCoordinate, argument.Directives)
		}

		if len(field.Arguments) > 0 {
			this.addSchema(field.Position, argumentsSchemaName(definition.Name, field.Name), this.toArgumentsSchema(field.Arguments))
		}
	}
}

// A resource has one body, so a root field is two: the arguments it takes and what it returns.
func (this *sdlDeclarations) addRootFields(root *ast.Definition, operation string) {
	if root == nil {
		return
	}

	for _, field := range declaredFields(root) {
		if len(field.Arguments) == 0 {
			this.addSchema(field.Position, argumentsSchemaName(root.Name, field.Name), contract.Schema{Type: model.ObjectType})
		}

		this.addSchema(field.Position, returnsSchemaName(root.Name, field.Name), this.toOutputSchema(field.Type))

		this.addResource(field.Position, operation, field.Name, graphqlmodel.ArgumentsInteraction, model.Input, argumentsSchemaName(root.Name, field.Name))
		this.addResource(field.Position, operation, field.Name, graphqlmodel.ReturnsInteraction, model.Output, returnsSchemaName(root.Name, field.Name))
	}
}

func (this *sdlDeclarations) addResource(position *ast.Position, operation, field, interaction string, body model.BodyRole, schemaName string) {
	source := position.Src.Name
	root := contract.NewResourcePath("")

	this.declarations.Resources = append(this.declarations.Resources, protocol.ResourceDeclaration{
		Source:     source,
		Path:       root.Append(model.Provides.String(), graphqlmodel.Protocol.String(), operation, field, interaction),
		Namespace:  graphqlmodel.Namespace,
		SchemaName: schemaName,
		Resource: model.UploadedResource{Resource: model.Resource{
			Direction: model.Provides,
			Protocol:  graphqlmodel.Protocol,
			Body:      body,
			Identity:  graphqlmodel.Identity{Operation: operation, Field: field, Interaction: interaction},
			Bodies:    []model.ResourceBody{{Source: source, Namespace: graphqlmodel.Namespace, SchemaName: null.StringFrom(schemaName)}},
		}},
	})
}

func (this *sdlDeclarations) addSchema(position *ast.Position, name string, schema contract.Schema) {
	this.declarations.Schemas = append(this.declarations.Schemas, protocol.SchemaDeclaration{
		Source:        position.Src.Name,
		Namespace:     graphqlmodel.Namespace,
		Name:          name,
		Schema:        schema,
		KeepUnreached: true,
	})
}

func (this *sdlDeclarations) addTypeInfo(position *ast.Position, coordinate, kind, namedType string) {
	piece := this.pieceOf(position.Src.Name)
	piece.Types = append(piece.Types, graphqlmodel.TypeInfo{Coordinate: coordinate, Kind: kind, NamedType: namedType})
}

// Only the directives the composition counts are kept; the others are read and ignored.
func (this *sdlDeclarations) addDirectives(coordinate string, directives ast.DirectiveList) {
	for _, directive := range directives {
		counted, counts := this.directives[directive.Name]
		if !counts {
			continue
		}

		kept := graphqlmodel.Directive{Coordinate: coordinate, Directive: counted, Resolvable: true}
		if counted == graphqlmodel.KeyDirective {
			kept.Fields = toNormalizedFieldSet(directive.Arguments.ForName("fields").Value.Raw)
			if resolvable := directive.Arguments.ForName("resolvable"); resolvable != nil {
				kept.Resolvable = resolvable.Value.Raw != "false"
			}
		}

		piece := this.pieceOf(directive.Position.Src.Name)
		piece.Directives = append(piece.Directives, kept)
	}
}

func (this *sdlDeclarations) pieceOf(source string) *graphqlmodel.Extension {
	if this.pieces[source] == nil {
		this.pieces[source] = &graphqlmodel.Extension{}
	}

	return this.pieces[source]
}

func (this *sdlDeclarations) addExtensionPieces() {
	for _, source := range slices.Sorted(maps.Keys(this.pieces)) {
		this.declarations.AddExtensionPiece(graphqlmodel.Protocol.String(), protocol.ExtensionPiece{
			Source: source,
			Value:  *this.pieces[source],
		})
	}
}

func (this *sdlDeclarations) toDefinitionSchema(definition *ast.Definition) contract.Schema {
	switch definition.Kind {
	case ast.Object:
		return contract.Schema{Type: model.ObjectType, Properties: this.toOutputProperties(definition)}

	case ast.InputObject:
		properties := make(map[string]contract.Schema, len(definition.Fields))
		for _, field := range definition.Fields {
			properties[field.Name] = this.toInputSchema(field.Type, field.DefaultValue)
		}

		return contract.Schema{Type: model.ObjectType, Properties: properties}

	case ast.Interface:
		members := map[string]contract.Schema{}
		for _, implementation := range this.schema.GetPossibleTypes(definition) {
			if implementation.Kind == ast.Object {
				members[implementation.Name] = contract.Schema{Ref: implementation.Name}
			}
		}

		return contract.Schema{Type: model.UnionType, Properties: this.toOutputProperties(definition), UnionMembers: members}

	case ast.Union:
		members := make(map[string]contract.Schema, len(definition.Types))
		for _, member := range definition.Types {
			members[member] = contract.Schema{Ref: member}
		}

		return contract.Schema{Type: model.UnionType, UnionMembers: members}

	case ast.Enum:
		values := make([]string, 0, len(definition.EnumValues))
		for _, value := range definition.EnumValues {
			values = append(values, value.Name)
		}

		return contract.Schema{Type: model.EnumType, Members: values}

	default:
		return contract.Schema{Type: model.ScalarType, Scalar: definition.Name}
	}
}

func (this *sdlDeclarations) toOutputProperties(definition *ast.Definition) map[string]contract.Schema {
	properties := make(map[string]contract.Schema, len(definition.Fields))
	for _, field := range declaredFields(definition) {
		property := this.toOutputSchema(field.Type)
		if len(field.Arguments) > 0 {
			property.Arguments = argumentsSchemaName(definition.Name, field.Name)
		}

		properties[field.Name] = property
	}

	return properties
}

func (this *sdlDeclarations) toArgumentsSchema(arguments ast.ArgumentDefinitionList) contract.Schema {
	properties := make(map[string]contract.Schema, len(arguments))
	for _, argument := range arguments {
		properties[argument.Name] = this.toInputSchema(argument.Type, argument.DefaultValue)
	}

	return contract.Schema{Type: model.ObjectType, Properties: properties}
}

// What a server returns may be null, never absent.
func (this *sdlDeclarations) toOutputSchema(fieldType *ast.Type) contract.Schema {
	schema := this.toTypeSchema(fieldType)
	schema.Nullable = !fieldType.NonNull

	return schema
}

// A sender may leave out what is nullable or has a default, but may send null only where the type is nullable.
func (this *sdlDeclarations) toInputSchema(fieldType *ast.Type, defaultValue *ast.Value) contract.Schema {
	schema := this.toTypeSchema(fieldType)
	schema.Nullable = !fieldType.NonNull
	schema.Optional = !fieldType.NonNull || defaultValue != nil

	return schema
}

func (this *sdlDeclarations) toTypeSchema(fieldType *ast.Type) contract.Schema {
	if fieldType.Elem != nil {
		items := this.toTypeSchema(fieldType.Elem)
		items.Nullable = !fieldType.Elem.NonNull

		return contract.Schema{Type: model.ArrayType, Items: &items}
	}

	if builtIn, isBuiltIn := builtInScalarSchemas[fieldType.NamedType]; isBuiltIn {
		return builtIn
	}

	if this.schema.Types[fieldType.NamedType].Kind == ast.Scalar {
		return contract.Schema{Type: model.ScalarType, Scalar: fieldType.NamedType}
	}

	return contract.Schema{Ref: fieldType.NamedType}
}

// The query root also holds the introspection fields, which no SDL declares: a declared name never starts with "__".
func declaredFields(definition *ast.Definition) ast.FieldList {
	var fields ast.FieldList
	for _, field := range definition.Fields {
		if !strings.HasPrefix(field.Name, "__") {
			fields = append(fields, field)
		}
	}

	return fields
}

// ":" is invalid in a GraphQL name, so a synthesized schema never takes the name of a type.
func argumentsSchemaName(typeName, field string) string {
	return typeName + ":" + field + ":" + graphqlmodel.ArgumentsInteraction
}

func synthesizedSchemaNameToField(name string) (typeName, field, interaction string, synthesized bool) {
	parts := strings.Split(name, ":")
	if len(parts) != 3 || (parts[2] != graphqlmodel.ArgumentsInteraction && parts[2] != graphqlmodel.ReturnsInteraction) {
		return "", "", "", false
	}

	return parts[0], parts[1], parts[2], true
}

func returnsSchemaName(typeName, field string) string {
	return typeName + ":" + field + ":" + graphqlmodel.ReturnsInteraction
}
