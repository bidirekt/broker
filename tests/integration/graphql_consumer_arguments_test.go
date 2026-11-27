package integration_test

import (
	"encoding/json"
	"net/http"
)

const argumentsProviderSDL = `type Query {
  pet(id: ID!): Pet
}

type Pet {
  id: ID!
  reviews(first: Int): [String!]!
}
`

type argumentsFile struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

func (this *IntegrationSuite) mustPostForArguments(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) deployArgumentsProvider() {
	this.mustPostForArguments("/api/environments", `{"environment":"production"}`)
	this.mustPostForArguments("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForArguments("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForArguments("/api/contracts", this.publishBody("petstore_api", "1.0.0", contractFragment{"schema.graphqls", argumentsProviderSDL}))
	this.mustPostForArguments("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) validateArguments(consumer string) (status int, body string) {
	request, err := json.Marshal(struct {
		Participant string          `json:"participant"`
		Environment string          `json:"environment"`
		Contracts   []argumentsFile `json:"contracts"`
	}{Participant: "petstore_web", Environment: "production", Contracts: []argumentsFile{{"petstore_web.yaml", consumer}}})
	this.Require().NoError(err)

	return this.post("/api/contracts/validate", string(request))
}

func (this *IntegrationSuite) TestPublishAndValidate_ArgumentsNamingAnythingButANonNullableObject_AreA400AndNeverChecked() {
	this.deployArgumentsProvider()
	contractsBefore, schemasBefore := this.countRows("contracts"), this.countRows("schemas")

	cases := []struct {
		name      string
		consumer  string
		violation string
	}{
		{
			name: "the field's arguments name a string",
			consumer: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetId, returns: MaybePet}
schemas:
  PetId: {type: string}
  MaybePet: {type: object, nullable: true, properties: {id: {type: string}}}
`,
			violation: `{"code":"schema.arguments_not_object","path":"consumes;petstore_api;graphql;query;pet;arguments","source":"petstore_web.yaml","details":{"schema":"PetId","reason":"not_object","type":"string"}}`,
		},
		{
			name: "the field's arguments are nullable",
			consumer: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: MaybeArgs, returns: MaybePet}
schemas:
  MaybeArgs: {type: object, nullable: true, properties: {id: {type: string}}}
  MaybePet: {type: object, nullable: true, properties: {id: {type: string}}}
`,
			violation: `{"code":"schema.arguments_not_object","path":"consumes;petstore_api;graphql;query;pet;arguments","source":"petstore_web.yaml","details":{"schema":"MaybeArgs","reason":"nullable"}}`,
		},
		{
			name: "a property's arguments name an integer",
			consumer: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet:
    type: object
    nullable: true
    properties:
      reviews: {type: array, items: {type: string}, arguments: First}
  First: {type: integer}
`,
			violation: `{"code":"schema.arguments_not_object","path":"schemas;MaybePet;properties;reviews;arguments","source":"petstore_web.yaml","details":{"schema":"First","reason":"not_object","type":"integer"}}`,
		},
		{
			name: "a property's arguments ref a nullable object",
			consumer: `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet:
    type: object
    nullable: true
    properties:
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs: {ref: MaybeArgs}
  MaybeArgs: {type: object, nullable: true, properties: {first: {type: integer}}}
`,
			violation: `{"code":"schema.arguments_not_object","path":"schemas;MaybePet;properties;reviews;arguments","source":"petstore_web.yaml","details":{"schema":"ReviewsArgs","reason":"nullable"}}`,
		},
	}

	for _, tc := range cases {
		expected := `{"message":"contract validation failed","violations":[` + tc.violation + `]}`

		status, body := this.post("/api/contracts", this.publishBody("petstore_web", "1.0.0", contractFragment{"petstore_web.yaml", tc.consumer}))
		this.Equal(http.StatusBadRequest, status, tc.name)
		this.JSONEq(expected, body, tc.name)

		status, body = this.validateArguments(tc.consumer)
		this.Equal(http.StatusBadRequest, status, tc.name)
		this.JSONEq(expected, body, tc.name)
	}

	this.Equal(contractsBefore, this.countRows("contracts"))
	this.Equal(schemasBefore, this.countRows("schemas"))
}
