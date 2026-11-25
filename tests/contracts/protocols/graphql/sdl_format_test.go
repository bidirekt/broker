package graphql_test

import (
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const petsSDL = "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"

func sdlDeclarations(t *testing.T, sdl string) protocol.Declarations {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "schema.graphqls", Content: sdl}})
	require.NoError(t, readErr)

	return declarations
}

func sdlSchema(t *testing.T, declarations protocol.Declarations, name string) contract.Schema {
	t.Helper()

	for _, declaration := range declarations.Schemas {
		if declaration.Name == name {
			assert.Equal(t, "graphql", declaration.Namespace)

			return declaration.Schema
		}
	}

	require.Failf(t, "schema not declared", "%s", name)

	return contract.Schema{}
}

func sdlSchemaNames(declarations protocol.Declarations) []string {
	names := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		names = append(names, declaration.Name)
	}

	return names
}

func TestSDLFormat_ReadsGraphQLAndGraphQLSFiles(t *testing.T) {
	assert.Equal(t, []string{".graphql", ".graphqls"}, graphqlcontracts.NewSDLFormat().Extensions())
}

func TestSDLFormat_ToDeclarations_DeclaresTheGraphQLFormat(t *testing.T) {
	assert.Equal(t, []string{"graphql"}, sdlDeclarations(t, petsSDL).Formats)
}

func TestSDLFormat_BuiltInScalars_BecomeTheModelPrimitives(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet!\n}\n\ntype Pet {\n  id: ID!\n  name: String!\n  age: Int!\n  weight: Float!\n  vaccinated: Boolean!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"id":         {Type: "string"},
		"name":       {Type: "string"},
		"age":        {Type: "integer"},
		"weight":     {Type: "float"},
		"vaccinated": {Type: "boolean"},
	}}, sdlSchema(t, declarations, "Pet"))
}

func TestSDLFormat_ACustomScalar_IsAScalarOfItsName(t *testing.T) {
	declarations := sdlDeclarations(t, "scalar DateTime\n\ntype Query {\n  pet: Pet!\n}\n\ntype Pet {\n  bornAt: DateTime!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"bornAt": {Type: "scalar", Scalar: "DateTime"},
	}}, sdlSchema(t, declarations, "Pet"))
	assert.Equal(t, contract.Schema{Type: "scalar", Scalar: "DateTime"}, sdlSchema(t, declarations, "DateTime"))
}

func TestSDLFormat_TypeAndInput_BecomeObjects(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pets(filter: PetFilter!): [Pet!]!\n}\n\ntype Pet {\n  name: String!\n}\n\ninput PetFilter {\n  name: String!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{"name": {Type: "string"}}}, sdlSchema(t, declarations, "Pet"))
	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{"name": {Type: "string"}}}, sdlSchema(t, declarations, "PetFilter"))
}

func TestSDLFormat_AnEnum_IsAnEnumOfItsValues(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  status: Status!\n}\n\nenum Status {\n  AVAILABLE\n  ADOPTED\n}\n")

	assert.Equal(t, contract.Schema{Type: "enum", Members: []string{"AVAILABLE", "ADOPTED"}}, sdlSchema(t, declarations, "Status"))
}

func TestSDLFormat_AUnion_IsAUnionKeyedByTypeName(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet!\n}\n\nunion Pet = Dog | Cat\n\ntype Dog {\n  barks: Boolean!\n}\n\ntype Cat {\n  meows: Boolean!\n}\n")

	assert.Equal(t, contract.Schema{Type: "union", UnionMembers: map[string]contract.Schema{
		"Dog": {Ref: "Dog"},
		"Cat": {Ref: "Cat"},
	}}, sdlSchema(t, declarations, "Pet"))
}

func TestSDLFormat_AnInterface_IsAUnionOfItsImplementations(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet!\n}\n\ninterface Pet {\n  name: String!\n}\n\ntype Dog implements Pet {\n  name: String!\n}\n\ntype Cat implements Pet {\n  name: String!\n}\n")

	assert.Equal(t, contract.Schema{Type: "union", UnionMembers: map[string]contract.Schema{
		"Dog": {Ref: "Dog"},
		"Cat": {Ref: "Cat"},
	}}, sdlSchema(t, declarations, "Pet"))
}

func TestSDLFormat_AnOutputFieldWithoutBang_IsNullableNeverOptional(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  name: String\n  owner: Owner\n}\n\ntype Owner {\n  name: String!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"name":  {Type: "string", Nullable: true},
		"owner": {Ref: "Owner", Nullable: true},
	}}, sdlSchema(t, declarations, "Pet"))
}

func TestSDLFormat_AnArgumentOrInputFieldWithoutBang_IsOptionalAndNullable(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pets(name: String, filter: PetFilter): [String!]!\n}\n\ninput PetFilter {\n  minAge: Int\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"name":   {Type: "string", Optional: true, Nullable: true},
		"filter": {Ref: "PetFilter", Optional: true, Nullable: true},
	}}, sdlSchema(t, declarations, "Query:pets:arguments"))
	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"minAge": {Type: "integer", Optional: true, Nullable: true},
	}}, sdlSchema(t, declarations, "PetFilter"))
}

func TestSDLFormat_AnArgumentOrInputFieldWithBangAndDefault_IsOptionalNeverNullable(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pets(first: Int! = 10, filter: PetFilter!): [String!]!\n}\n\ninput PetFilter {\n  minAge: Int! = 0\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"first":  {Type: "integer", Optional: true},
		"filter": {Ref: "PetFilter"},
	}}, sdlSchema(t, declarations, "Query:pets:arguments"))
	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"minAge": {Type: "integer", Optional: true},
	}}, sdlSchema(t, declarations, "PetFilter"))
}

func TestSDLFormat_ANonNullListOfNonNullItems_IsANonNullArrayOfNonNullItems(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet!\n}\n\ntype Pet {\n  reviews: [Review!]!\n  tags: [String]\n}\n\ntype Review {\n  stars: Int!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"reviews": {Type: "array", Items: &contract.Schema{Ref: "Review"}},
		"tags":    {Type: "array", Nullable: true, Items: &contract.Schema{Type: "string", Nullable: true}},
	}}, sdlSchema(t, declarations, "Pet"))
}

func TestSDLFormat_ARootField_HasItsArgumentsAndReturnsSchemas(t *testing.T) {
	declarations := sdlDeclarations(t, petsSDL)

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"id": {Type: "string"},
	}}, sdlSchema(t, declarations, "Query:pet:arguments"))
	assert.Equal(t, contract.Schema{Ref: "Pet", Nullable: true}, sdlSchema(t, declarations, "Query:pet:returns"))
}

func TestSDLFormat_ARootFieldWithoutArguments_HasAnEmptyArgumentsObject(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pets: [Pet!]!\n}\n\ntype Mutation {\n  adoptAll: Boolean!\n}\n\ntype Pet {\n  id: ID!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object"}, sdlSchema(t, declarations, "Query:pets:arguments"))
	assert.Equal(t, contract.Schema{Type: "array", Items: &contract.Schema{Ref: "Pet"}}, sdlSchema(t, declarations, "Query:pets:returns"))
	assert.Equal(t, contract.Schema{Type: "object"}, sdlSchema(t, declarations, "Mutation:adoptAll:arguments"))
	assert.Equal(t, contract.Schema{Type: "boolean"}, sdlSchema(t, declarations, "Mutation:adoptAll:returns"))
}

func TestSDLFormat_ANestedFieldWithArguments_PointsAtItsArgumentsSchema(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet!\n}\n\ntype Pet {\n  reviews(first: Int): [Review!]!\n}\n\ntype Review {\n  stars: Int!\n}\n")

	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"reviews": {Type: "array", Items: &contract.Schema{Ref: "Review"}, Arguments: "Pet:reviews:arguments"},
	}}, sdlSchema(t, declarations, "Pet"))
	assert.Equal(t, contract.Schema{Type: "object", Properties: map[string]contract.Schema{
		"first": {Type: "integer", Optional: true, Nullable: true},
	}}, sdlSchema(t, declarations, "Pet:reviews:arguments"))
}

func TestSDLFormat_TheSubscriptionRoot_IsASchemaWithoutResources(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet: Pet\n}\n\ntype Subscription {\n  petAdded(kind: String): Pet!\n}\n\ntype Pet {\n  id: ID!\n}\n")

	assert.Equal(t, []string{"Pet", "Query", "Subscription", "Subscription:petAdded:arguments", "Query:pet:arguments", "Query:pet:returns"}, sdlSchemaNames(declarations))

	for _, resource := range declarations.Resources {
		assert.Equal(t, "query", resource.Resource.Identity.(graphqlmodel.Identity).Operation)
	}
}

func TestSDLFormat_ATypePointingAtTheSubscriptionRoot_IsPublishedWithTheRootAsASchema(t *testing.T) {
	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	uploaded, violations, readErr, err := interpreter.ToUploadedContract(
		[]model.ContractFile{{Source: "schema.graphqls", Content: "type Query {\n  pet: Pet\n}\n\ntype Subscription {\n  petAdded: Pet!\n}\n\ntype Pet {\n  id: ID!\n  updates: Subscription\n}\n"}},
		&model.Participant{ID: 1, Name: "pets_api"},
		"1.0.0",
	)
	require.NoError(t, err)
	require.NoError(t, readErr)
	require.Empty(t, violations)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "petAdded": {Ref: "Pet"}}, uploaded.Schemas["graphql"]["Subscription"])
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string"}, "updates": {Ref: "Subscription", Nullable: true}}, uploaded.Schemas["graphql"]["Pet"])
	assert.Len(t, uploaded.Resources, 2)
}

const sharedRootSDL = "schema {\n  query: Root\n  mutation: Root\n}\n\ntype Root {\n  pet(id: ID!): Pet\n  pets: [Pet!]!\n}\n\ntype Pet {\n  id: ID!\n}\n"

func TestSDLFormat_OneTypeAsTheQueryAndTheMutationRoot_GivesItsFieldsOnce(t *testing.T) {
	declarations := sdlDeclarations(t, sharedRootSDL)

	assert.Equal(t, []string{"Pet", "Root", "Root:pet:arguments", "Root:pet:returns", "Root:pets:arguments", "Root:pets:returns"}, sdlSchemaNames(declarations))

	var paths []string
	for _, declaration := range declarations.Resources {
		paths = append(paths, declaration.Path.String())
	}

	assert.Equal(t, []string{
		"provides;graphql;query;pet;arguments",
		"provides;graphql;query;pet;returns",
		"provides;graphql;query;pets;arguments",
		"provides;graphql;query;pets;returns",
	}, paths)
}

func TestSDLFormat_OneTypeAsTheQueryAndTheMutationRoot_HasNoDuplicateSchema(t *testing.T) {
	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	violations, readErr := interpreter.ToViolations([]model.ContractFile{{Source: "schema.graphqls", Content: sharedRootSDL}})

	require.NoError(t, readErr)
	assert.Empty(t, violations)
}

func TestSDLFormat_EachRootField_IsTwoProvidedResources(t *testing.T) {
	declarations := sdlDeclarations(t, "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Mutation {\n  adoptPet(id: ID!): Pet!\n}\n\ntype Pet {\n  id: ID!\n}\n")

	resource := func(operation, field, interaction string, body model.BodyRole, schemaName string) protocol.ResourceDeclaration {
		return protocol.ResourceDeclaration{
			Source:     "schema.graphqls",
			Path:       contract.ResourcePath("provides;graphql;" + operation + ";" + field + ";" + interaction),
			Namespace:  "graphql",
			SchemaName: schemaName,
			Resource: model.UploadedResource{Resource: model.Resource{
				Direction: model.Provides,
				Protocol:  "graphql",
				Body:      body,
				Identity:  graphqlmodel.Identity{Operation: operation, Field: field, Interaction: interaction},
				Bodies:    []model.ResourceBody{{Source: "schema.graphqls", Namespace: "graphql", SchemaName: null.StringFrom(schemaName)}},
			}},
		}
	}

	assert.Equal(t, []protocol.ResourceDeclaration{
		resource("query", "pet", "arguments", model.Input, "Query:pet:arguments"),
		resource("query", "pet", "returns", model.Output, "Query:pet:returns"),
		resource("mutation", "adoptPet", "arguments", model.Input, "Mutation:adoptPet:arguments"),
		resource("mutation", "adoptPet", "returns", model.Output, "Mutation:adoptPet:returns"),
	}, declarations.Resources)
}

func TestSDLFormat_KindsAndNamedTypes_AreTheGraphQLPieceOfEachFile(t *testing.T) {
	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "schema.graphqls", Content: "type Query {\n  pet(id: ID!, name: String): Pet\n}\n"},
		{Source: "pet.graphqls", Content: "type Pet {\n  id: ID!\n  tags: [String!]!\n}\n\ninput PetInput {\n  id: ID!\n}\n"},
	})
	require.NoError(t, readErr)

	assert.Equal(t, map[string][]protocol.ExtensionPiece{"graphql": {
		{Source: "pet.graphqls", Value: graphqlmodel.Extension{Types: []graphqlmodel.TypeInfo{
			{Coordinate: "Pet", Kind: "object"},
			{Coordinate: "Pet.id", Kind: "field", NamedType: "ID"},
			{Coordinate: "Pet.tags", Kind: "field", NamedType: "String"},
			{Coordinate: "PetInput", Kind: "input"},
			{Coordinate: "PetInput.id", Kind: "field", NamedType: "ID"},
		}}},
		{Source: "schema.graphqls", Value: graphqlmodel.Extension{Types: []graphqlmodel.TypeInfo{
			{Coordinate: "Query", Kind: "object"},
			{Coordinate: "Query.pet", Kind: "field", NamedType: "Pet"},
			{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"},
			{Coordinate: "Query.pet(name:)", Kind: "argument", NamedType: "String"},
		}}},
	}}, declarations.ExtensionPieces)
}

func TestSDLFormat_EveryKind_IsDelivered(t *testing.T) {
	declarations := sdlDeclarations(t, "scalar DateTime\n\ntype Query {\n  node: Node\n}\n\ninterface Node {\n  id: ID!\n}\n\ntype Pet implements Node {\n  id: ID!\n}\n\nunion Search = Pet\n\nenum Status {\n  AVAILABLE\n}\n\ninput PetInput {\n  id: ID!\n}\n")

	kinds := map[string]string{}
	for _, info := range declarations.ExtensionPieces["graphql"][0].Value.(graphqlmodel.Extension).Types {
		if info.NamedType == "" {
			kinds[info.Coordinate] = info.Kind
		}
	}

	assert.Equal(t, map[string]string{
		"DateTime": "scalar",
		"Node":     "interface",
		"Pet":      "object",
		"PetInput": "input",
		"Query":    "object",
		"Search":   "union",
		"Status":   "enum",
	}, kinds)
}

func TestSDLFormat_SeveralFiles_AreOneSchema(t *testing.T) {
	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "query.graphqls", Content: "type Query {\n  pet: Pet\n}\n"},
		{Source: "pet.graphqls", Content: "type Pet {\n  id: ID!\n}\n\nextend type Query {\n  pets: [Pet!]!\n}\n"},
	})
	require.NoError(t, readErr)

	assert.Equal(t, contract.Schema{Ref: "Pet", Nullable: true}, sdlSchema(t, declarations, "Query:pet:returns"))
	assert.Equal(t, contract.Schema{Type: "array", Items: &contract.Schema{Ref: "Pet"}}, sdlSchema(t, declarations, "Query:pets:returns"))

	sources := map[string]string{}
	for _, declaration := range declarations.Resources {
		sources[declaration.Path.String()] = declaration.Source
	}

	assert.Equal(t, map[string]string{
		"provides;graphql;query;pet;arguments":  "query.graphqls",
		"provides;graphql;query;pet;returns":    "query.graphqls",
		"provides;graphql;query;pets;arguments": "pet.graphqls",
		"provides;graphql;query;pets;returns":   "pet.graphqls",
	}, sources)
}

func TestSDLFormat_AnInvalidSDL_IsAReadErrorWithTheParserText(t *testing.T) {
	_, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "schema.graphqls", Content: "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  owner: Owner\n}\n"},
	})

	assert.EqualError(t, readErr, "schema.graphqls:6:10: Undefined type Owner.")
}

func TestSDLFormat_ADocumentOfOperations_IsAReadError(t *testing.T) {
	_, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "schema.graphqls", Content: petsSDL},
		{Source: "queries.graphql", Content: "query {\n  pet(id: 1) {\n    id\n  }\n}\n"},
	})

	assert.EqualError(t, readErr, "queries.graphql: holds operations; publish expects the GraphQL schema")
}

func TestSDLFormat_ATypeNoRootFieldReaches_IsKeptInTheContract(t *testing.T) {
	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	uploaded, violations, readErr, err := interpreter.ToUploadedContract(
		[]model.ContractFile{{Source: "schema.graphqls", Content: petsSDL + "\ntype Product {\n  sku: String!\n}\n"}},
		&model.Participant{ID: 1, Name: "pets_api"},
		"1.0.0",
	)
	require.NoError(t, err)
	require.NoError(t, readErr)
	require.Empty(t, violations)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "sku": {Type: "string"}}, uploaded.Schemas["graphql"]["Product"])
	assert.Len(t, uploaded.Resources, 2)
}

func TestSDLFormat_GraphQLFilesReachTheFormat_NextToYAMLFiles(t *testing.T) {
	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	violations, readErr := interpreter.ToViolations([]model.ContractFile{
		{Source: "schema.graphqls", Content: "type Query {\n  pet(id: ID!): Pet\n}\n"},
		{Source: "pet.graphql", Content: "type Pet {\n  id: ID!\n}\n"},
		{Source: "api.yaml", Content: "provides:\n  rest:\n    /pets:\n      get:\n        responses:\n          200: ~\n"},
	})

	require.NoError(t, readErr)
	assert.Empty(t, violations)
}
