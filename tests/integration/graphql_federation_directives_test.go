package integration_test

import (
	"context"
	"net/http"

	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
)

const directivesGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const directivesLinkSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@external", "@requires"])
`

const directivesTypesSDL = `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") @key(fields: "name owner { id }") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
}

type Owner @key(fields: "id", resolvable: false) {
  id: ID!
}
`

const directivesReorderedSDL = directivesLinkSDL + `
type Owner @key(resolvable: false, fields: "id") {
  id: ID!
}

type Pet @key(fields: "name,owner{id}") @key(fields: "  id ") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
}

type Query {
  pet(id: ID!): Pet
}
`

var directivesStored = []graphqlmodel.Directive{
	{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: false},
	{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
	{Coordinate: "Pet", Directive: "key", Fields: "name owner { id }", Resolvable: true},
	{Coordinate: "Pet.name", Directive: "shareable", Resolvable: true},
	{Coordinate: "Pet.weight", Directive: "external", Resolvable: true},
}

var directivesTypeInfo = []graphqlmodel.TypeInfo{
	{Coordinate: "Owner", Kind: "object"},
	{Coordinate: "Owner.id", Kind: "field", NamedType: "ID"},
	{Coordinate: "Pet", Kind: "object"},
	{Coordinate: "Pet.id", Kind: "field", NamedType: "ID"},
	{Coordinate: "Pet.name", Kind: "field", NamedType: "String"},
	{Coordinate: "Pet.owner", Kind: "field", NamedType: "Owner"},
	{Coordinate: "Pet.shippingCost", Kind: "field", NamedType: "Int"},
	{Coordinate: "Pet.weight", Kind: "field", NamedType: "Int"},
	{Coordinate: "Query", Kind: "object"},
	{Coordinate: "Query.pet", Kind: "field", NamedType: "Pet"},
	{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"},
}

func (this *IntegrationSuite) publishDirectivesSubgraph(version string, fragments ...contractFragment) {
	status, body := this.post("/api/contracts", this.publishBody("petstore_api", version, fragments...))
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) createDirectivesParticipant() {
	status, body := this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) storedDirectives() []graphqlmodel.Directive {
	rows, err := this.Pool.Query(context.Background(), `
		SELECT coordinate, directive, fields, override_from, resolvable
		FROM graphql_federation_directives
		ORDER BY contract_id, coordinate COLLATE "C", directive, fields COLLATE "C"`)
	this.Require().NoError(err)
	defer rows.Close()

	var directives []graphqlmodel.Directive
	for rows.Next() {
		var directive graphqlmodel.Directive
		this.Require().NoError(rows.Scan(&directive.Coordinate, &directive.Directive, &directive.Fields, &directive.From, &directive.Resolvable))
		directives = append(directives, directive)
	}
	this.Require().NoError(rows.Err())

	return directives
}

func (this *IntegrationSuite) storedTypeInfo() []graphqlmodel.TypeInfo {
	rows, err := this.Pool.Query(context.Background(), `
		SELECT coordinate, kind, named_type
		FROM graphql_type_info
		ORDER BY contract_id, coordinate COLLATE "C"`)
	this.Require().NoError(err)
	defer rows.Close()

	var types []graphqlmodel.TypeInfo
	for rows.Next() {
		var info graphqlmodel.TypeInfo
		this.Require().NoError(rows.Scan(&info.Coordinate, &info.Kind, &info.NamedType))
		types = append(types, info)
	}
	this.Require().NoError(rows.Err())

	return types
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_Publish_StoresARowPerCountedDirectiveAndPerTypeInfo() {
	this.createDirectivesParticipant()

	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + directivesTypesSDL},
	)

	this.Equal(directivesStored, this.storedDirectives())
	this.Equal(directivesTypeInfo, this.storedTypeInfo())
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_ALoadedContract_CarriesTheStoredExtension() {
	this.createDirectivesParticipant()
	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + directivesTypesSDL},
	)

	repo := repository.NewContractRepository(this.Pool, repository.Protocols{
		Tables:     []repository.ProtocolTable{restrepository.New(), graphqlrepository.New()},
		Extensions: []repository.ContractExtensionStore{graphqlrepository.New()},
	})
	contract, found := repo.GetContractByNameAndVersion(context.Background(), "petstore_api", "1")
	this.Require().True(found)

	this.Equal(map[string]any{"graphql": graphqlmodel.Extension{
		Graph:      "petstore_graph",
		Directives: directivesStored,
		Types:      directivesTypeInfo,
		Roots:      graphqlmodel.Roots{Query: "Query"},
	}}, contract.Extensions)
	this.Equal(contract.Extensions, repo.GetContractByIDAndVersion(context.Background(), contract.ID, "1").Extensions)
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_AnotherOrderSpacingOrFileNames_KeepsTheContract() {
	this.createDirectivesParticipant()

	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + directivesTypesSDL},
	)
	this.publishDirectivesSubgraph("2",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesReorderedSDL},
	)
	this.publishDirectivesSubgraph("3",
		contractFragment{"subgraph.yaml", directivesGraphYAML},
		contractFragment{"link.graphqls", directivesLinkSDL},
		contractFragment{"types.graphqls", directivesTypesSDL},
	)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(3, this.countRows("contract_versions"))
	this.Equal(directivesStored, this.storedDirectives())
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_ChangingOnlyADirective_IsANewContract() {
	this.createDirectivesParticipant()

	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + directivesTypesSDL},
	)
	this.publishDirectivesSubgraph("2",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") @key(fields: "name owner { id }") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
}

type Owner @key(fields: "id") {
  id: ID!
}
`},
	)

	this.Equal(2, this.countRows("contracts"))

	var resolvable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `
		SELECT d.resolvable
		FROM graphql_federation_directives d
		JOIN contract_versions cv ON cv.contract_id = d.contract_id
		WHERE cv.version = '2' AND d.coordinate = 'Owner'`).Scan(&resolvable))
	this.True(resolvable)
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_RepublishingTheSame_ReusesTheContractAndTheRows() {
	this.createDirectivesParticipant()
	files := []contractFragment{
		{"graph.yaml", directivesGraphYAML},
		{"schema.graphqls", directivesLinkSDL + directivesTypesSDL},
	}

	this.publishDirectivesSubgraph("1", files...)
	this.publishDirectivesSubgraph("1", files...)
	this.publishDirectivesSubgraph("2", files...)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(len(directivesStored), this.countRows("graphql_federation_directives"))
	this.Equal(len(directivesTypeInfo), this.countRows("graphql_type_info"))
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_AnSDLThatDeclaresTheFederationDefinitions_StoresTheSameDirectives() {
	this.createDirectivesParticipant()

	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", directivesLinkSDL + `
directive @link(url: String!, as: String, for: link__Purpose, import: [link__Import]) repeatable on SCHEMA

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

scalar link__Import

enum link__Purpose {
  SECURITY
  EXECUTION
}

scalar FieldSet
` + directivesTypesSDL},
	)

	this.Equal(directivesStored, this.storedDirectives())
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_AKeyRepeatedWithAndWithoutResolvable_KeepsOneContractInEitherOrder() {
	this.createDirectivesParticipant()
	ownerKeyedBy := func(keys string) string {
		return directivesLinkSDL + "\ntype Query {\n  owner: Owner\n}\n\ntype Owner " + keys + " {\n  id: ID!\n}\n"
	}

	this.publishDirectivesSubgraph("1",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", ownerKeyedBy(`@key(fields: "id", resolvable: false) @key(fields: "id")`)},
	)
	this.publishDirectivesSubgraph("2",
		contractFragment{"graph.yaml", directivesGraphYAML},
		contractFragment{"schema.graphqls", ownerKeyedBy(`@key(fields: "id") @key(fields: "id", resolvable: false)`)},
	)

	this.Equal(1, this.countRows("contracts"))
	this.Equal([]graphqlmodel.Directive{{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: true}}, this.storedDirectives())
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_AnOverride_IsStoredWithItsFromAndCountsInTheChecksum() {
	this.createDirectivesParticipant()
	overridingFrom := func(from string) string {
		return `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@override"])

type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
  rating: Int @override(from: "` + from + `")
}
`
	}

	this.publishDirectivesSubgraph("1", contractFragment{"graph.yaml", directivesGraphYAML}, contractFragment{"schema.graphqls", overridingFrom("petstore_api")})
	this.publishDirectivesSubgraph("2", contractFragment{"graph.yaml", directivesGraphYAML}, contractFragment{"schema.graphqls", overridingFrom("petstore_api")})
	this.publishDirectivesSubgraph("3", contractFragment{"graph.yaml", directivesGraphYAML}, contractFragment{"schema.graphqls", overridingFrom("petstore_orders")})

	this.Equal(2, this.countRows("contracts"))
	this.Equal([]graphqlmodel.Directive{
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.rating", Directive: "override", From: "petstore_api", Resolvable: true},
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.rating", Directive: "override", From: "petstore_orders", Resolvable: true},
	}, this.storedDirectives())
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_ALoadedContract_CarriesTheRootTypesTheSchemaDefinitionNames() {
	this.createDirectivesParticipant()
	this.publishDirectivesSubgraph("1", contractFragment{"schema.graphqls", "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Int\n}\n"})

	repo := repository.NewContractRepository(this.Pool, repository.Protocols{
		Tables:     []repository.ProtocolTable{graphqlrepository.New()},
		Extensions: []repository.ContractExtensionStore{graphqlrepository.New()},
	})
	contract, found := repo.GetContractByNameAndVersion(context.Background(), "petstore_api", "1")
	this.Require().True(found)

	this.Equal(graphqlmodel.Roots{Query: "RootQuery"}, contract.Extensions["graphql"].(graphqlmodel.Extension).Roots)
}

func (this *IntegrationSuite) TestGraphQLFederationDirectives_AnSDLWithTheDefaultRoots_KeepsItsChecksumAcrossPublishes() {
	this.createDirectivesParticipant()
	files := []contractFragment{{"schema.graphqls", "type Query {\n  pet: Int\n}\n\ntype Mutation {\n  adoptPet: Int\n}\n"}}

	this.publishDirectivesSubgraph("1", files...)
	this.publishDirectivesSubgraph("2", files...)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("graphql_root_types"))
}
