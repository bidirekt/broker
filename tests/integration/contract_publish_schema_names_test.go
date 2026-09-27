package integration_test

import (
	"net/http"
)

const namesParticipantBody = `{"participant":"names_service"}`

const namesEndpointsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
`

const namesRequestYAML = `provides:
  rest:
    /pets:
      post:
        request: Pet
`

const namesConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Pets
`

const namesConsumerRequestYAML = `consumes:
  payments:
    rest:
      /invoices:
        post:
          request: Pet
`

const namesDanglingSchemaYAML = `schemas:
  Invoice:
    type: object
    properties:
      payment:
        ref: Payment
`

const namesResolvedSchemasYAML = `schemas:
  Invoice:
    type: object
    properties:
      payment:
        ref: Payment
  Payment:
    type: object
    properties:
      total:
        type: integer
`

const namesCyclicSchemasYAML = `schemas:
  Pet:
    type: object
    properties:
      owner:
        ref: Owner
  Owner:
    type: object
    properties:
      pet:
        ref: Pet
`

const namesCyclicEndpointsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
`

const namesSingleFileJSON = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": { "200": "Inexistente" }
        }
      }
    }
  },
  "schemas": {
    "Pet": { "type": "object", "properties": { "id": { "type": "string" } } }
  }
}`

func (s *IntegrationSuite) TestPublishContract_UnresolvedResponseSchema_MultipleFragments() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesEndpointsYAML},
		contractFragment{"schemas.yaml", namesResolvedSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"pets.yaml","details":{"schema":"Pets","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnresolvedResponseSchema_SingleFile() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"api.yaml", namesSingleFileJSON},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"api.yaml","details":{"schema":"Inexistente","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnresolvedRequestSchema() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesRequestYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;post;request","source":"pets.yaml","details":{"schema":"Pet","resource":"provides POST /pets request"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnresolvedConsumedResponseSchema() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"a.yaml", namesConsumerYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"consumes;payments;rest;/invoices;get;responses;200","source":"a.yaml","details":{"schema":"Pets","resource":"consumes payments GET /invoices 200"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnresolvedConsumedRequestSchema() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"a.yaml", namesConsumerRequestYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"consumes;payments;rest;/invoices;post;request","source":"a.yaml","details":{"schema":"Pet","resource":"consumes payments POST /invoices request"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnresolvedRefInUnreachedSchema() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesEndpointsYAML},
		contractFragment{"billing.yaml", namesDanglingSchemaYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_ref","path":"schemas;Invoice;properties;payment","source":"billing.yaml","details":{"schema":"Payment","property":"Invoice.payment"}},`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"pets.yaml","details":{"schema":"Pets","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_CyclicSchema_RejectedBrokerStaysUp() {
	status, _ := s.post("/api/participants", namesParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesCyclicEndpointsYAML},
		contractFragment{"schemas.yaml", namesCyclicSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.too_deep","path":"schemas;Owner","source":"schemas.yaml","details":{"schema":"Owner","maxDepth":"10"}},`+
		`{"code":"schema.too_deep","path":"schemas;Pet","source":"schemas.yaml","details":{"schema":"Pet","maxDepth":"10"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))

	status, _ = s.post("/api/participants", `{"participant":"still_alive"}`)
	s.Equal(http.StatusOK, status)
}
