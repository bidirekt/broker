package integration_test

import (
	"context"
	"net/http"
)

const consumerPublishParticipant = "petstore_web"

func (this *IntegrationSuite) createConsumerPublishParticipant() {
	status, body := this.post("/api/participants", `{"participant":"`+consumerPublishParticipant+`"}`)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) publishConsumer(files ...contractFragment) (status int, body string) {
	return this.post("/api/contracts", this.publishBody(consumerPublishParticipant, "1.0.0", files...))
}

func (this *IntegrationSuite) consumerPublishStrings(query string) []string {
	rows, err := this.Pool.Query(context.Background(), query)
	this.Require().NoError(err)
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		this.Require().NoError(rows.Scan(&value))
		values = append(values, value)
	}
	this.Require().NoError(rows.Err())

	return values
}

const consumerPublishArgumentsBodies = `
	SELECT body FROM (
		SELECT gr.operation || ' ' || r.consumed_provider || ' ' || gr.field || ' ' || s.namespace || ' ' || s.name AS body
		FROM resources r
		JOIN graphql_resources gr ON gr.resource_id = r.id
		JOIN resource_bodies rb ON rb.resource_id = r.id
		JOIN schemas s ON s.id = rb.schema_id
		WHERE gr.interaction = 'arguments'
	) bodies
	ORDER BY body COLLATE "C"`

func (this *IntegrationSuite) TestPublishGraphQLConsumer_EachRefusal_IsA400WithItsViolationAndNothingStored() {
	this.createConsumerPublishParticipant()

	cases := []struct {
		name      string
		contract  string
		violation string
	}{
		{
			name: "subscription",
			contract: `consumes:
  petstore_api:
    graphql:
      subscription:
        petAdopted: {returns: Pet}
schemas:
  Pet: {type: object, properties: {name: {type: string}}}
`,
			violation: `{"code":"key.unknown","path":"consumes;petstore_api;graphql;subscription","source":"web.yaml","details":{"key":"subscription"}}`,
		},
		{
			name: "scalar in REST",
			contract: `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      bornAt: {type: scalar, name: DateTime}
`,
			violation: `{"code":"schema.scalar_not_allowed","path":"schemas;Pet;properties;bornAt;type","source":"web.yaml","details":{"protocol":"rest"}}`,
		},
		{
			name: "arguments in REST",
			contract: `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      reviews:
        type: array
        items: {type: string}
        arguments: ReviewsArgs
  ReviewsArgs: {type: object, properties: {first: {type: integer}}}
`,
			violation: `{"code":"schema.arguments_not_allowed","path":"schemas;Pet;properties;reviews;arguments","source":"web.yaml","details":{"protocol":"rest"}}`,
		},
		{
			name: "union with properties in REST",
			contract: `consumes:
  petstore_api:
    rest:
      /nodes:
        get:
          responses:
            200: Node
schemas:
  Node:
    type: union
    properties:
      id: {type: string}
    members:
      User: {ref: UserName}
  UserName: {type: object, properties: {name: {type: string}}}
`,
			violation: `{"code":"schema.key_not_allowed","path":"schemas;Node","source":"web.yaml","details":{"key":"properties","type":"union"}}`,
		},
		{
			name: "query in provides.graphql",
			contract: `provides:
  graphql:
    query:
      pet: {returns: Pet}
schemas:
  Pet: {type: object, properties: {name: {type: string}}}
`,
			violation: `{"code":"key.unknown","path":"provides;graphql;query","source":"web.yaml","details":{"key":"query"}}`,
		},
		{
			name: "graph in consumes",
			contract: `consumes:
  petstore_api:
    graphql:
      graph: petstore_graph
`,
			violation: `{"code":"key.unknown","path":"consumes;petstore_api;graphql;graph","source":"web.yaml","details":{"key":"graph"}}`,
		},
		{
			name: "field without returns",
			contract: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
`,
			violation: `{"code":"returns.required","path":"consumes;petstore_api;graphql;query;pet","source":"web.yaml","details":{}}`,
		},
		{
			name: "scalar without name",
			contract: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {returns: Pet}
schemas:
  Pet:
    type: object
    properties:
      bornAt: {type: scalar}
`,
			violation: `{"code":"schema.scalar_without_name","path":"schemas;Pet;properties;bornAt","source":"web.yaml","details":null}`,
		},
		{
			name: "arguments in the field name",
			contract: `consumes:
  petstore_api:
    graphql:
      query:
        "pets(first: 10)": {returns: PetList}
schemas:
  PetList: {type: array, items: {type: string}}
`,
			violation: `{"code":"operation.name_syntax","path":"consumes;petstore_api;graphql;query;pets(first: 10)","source":"web.yaml","details":{"name":"pets(first: 10)"}}`,
		},
	}

	for _, tc := range cases {
		status, body := this.publishConsumer(contractFragment{"web.yaml", tc.contract})

		this.Equal(http.StatusBadRequest, status, tc.name)
		this.JSONEq(`{"message":"contract validation failed","violations":[`+tc.violation+`]}`, body, tc.name)
	}

	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("schemas"))
}

func (this *IntegrationSuite) TestPublishGraphQLConsumer_AFieldWithoutArguments_SendsTheGeneratedEmptyArguments() {
	this.createConsumerPublishParticipant()

	status, body := this.publishConsumer(contractFragment{"web.yaml", `consumes:
  petstore_api:
    graphql:
      query:
        pets: {returns: PetList}
schemas:
  PetList: {type: array, items: {type: string}}
`})

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal([]string{"query petstore_api pets graphql_generated EmptyArguments"}, this.consumerPublishStrings(consumerPublishArgumentsBodies))
	this.Equal([]string{"object"}, this.consumerPublishStrings(`
		SELECT fv.type
		FROM schemas s
		JOIN fields f ON f.schema_id = s.id
		JOIN field_versions fv ON fv.field_id = f.id
		WHERE s.namespace = 'graphql_generated' AND s.name = 'EmptyArguments'`))
}

func (this *IntegrationSuite) TestPublishGraphQLConsumer_AYAMLSchemaNamedEmptyArguments_NeverCollidesWithTheGenerated() {
	this.createConsumerPublishParticipant()

	status, body := this.publishConsumer(
		contractFragment{"profile.yaml", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: EmptyArguments, returns: MaybePet}
        pets: {returns: PetList}
schemas:
  EmptyArguments: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  PetList: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {name: {type: string}}}
`},
		contractFragment{"search.yaml", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {returns: SearchedPet}
schemas:
  SearchedPet: {type: object, nullable: true, properties: {name: {type: string}}}
`},
	)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal([]string{
		"query petstore_api pet graphql_generated EmptyArguments",
		"query petstore_api pet yaml EmptyArguments",
		"query petstore_api pets graphql_generated EmptyArguments",
	}, this.consumerPublishStrings(consumerPublishArgumentsBodies))
	this.Equal([]string{"graphql_generated", "yaml"}, this.consumerPublishStrings(
		`SELECT namespace FROM schemas WHERE name = 'EmptyArguments' ORDER BY namespace COLLATE "C"`))
}

func (this *IntegrationSuite) TestPublishGraphQLConsumer_FieldsWithoutArgumentsInTwoBlocks_ShareOneEmptyArguments() {
	this.createConsumerPublishParticipant()

	status, body := this.publishConsumer(
		contractFragment{"api.yaml", `consumes:
  petstore_api:
    graphql:
      query:
        pets: {returns: PetList}
  petstore_graph:
    graphql:
      query:
        owners: {returns: OwnerList}
schemas:
  PetList: {type: array, items: {type: string}}
  OwnerList: {type: array, items: {type: string}}
`},
		contractFragment{"graph.yaml", `consumes:
  petstore_graph:
    graphql:
      mutation:
        adoptAny: {returns: PetList}
`},
	)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal([]string{
		"mutation petstore_graph adoptAny graphql_generated EmptyArguments",
		"query petstore_api pets graphql_generated EmptyArguments",
		"query petstore_graph owners graphql_generated EmptyArguments",
	}, this.consumerPublishStrings(consumerPublishArgumentsBodies))
	this.Equal(1, this.countRows("schemas WHERE name = 'EmptyArguments'"))
}
