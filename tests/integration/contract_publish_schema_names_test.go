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

func (this *IntegrationSuite) TestPublishContract_UnresolvedResponseSchema_MultipleFragments() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesEndpointsYAML},
		contractFragment{"schemas.yaml", namesResolvedSchemasYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"pets.yaml","details":{"schema":"Pets","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnresolvedResponseSchema_SingleFile() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"api.yaml", namesSingleFileJSON},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"api.yaml","details":{"schema":"Inexistente","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnresolvedRequestSchema() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesRequestYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;post;request","source":"pets.yaml","details":{"schema":"Pet","resource":"provides POST /pets request"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnresolvedConsumedResponseSchema() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"a.yaml", namesConsumerYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"consumes;payments;rest;/invoices;get;responses;200","source":"a.yaml","details":{"schema":"Pets","resource":"consumes payments GET /invoices 200"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnresolvedConsumedRequestSchema() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"a.yaml", namesConsumerRequestYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_name","path":"consumes;payments;rest;/invoices;post;request","source":"a.yaml","details":{"schema":"Pet","resource":"consumes payments POST /invoices request"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnresolvedRefInUnreachedSchema() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesEndpointsYAML},
		contractFragment{"billing.yaml", namesDanglingSchemaYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_ref","path":"schemas;Invoice;properties;payment","source":"billing.yaml","details":{"schema":"Payment","property":"Invoice.payment"}},`+
		`{"code":"schema.unresolved_name","path":"provides;rest;/pets;get;responses;200","source":"pets.yaml","details":{"schema":"Pets","resource":"provides GET /pets 200"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_CyclicSchema_RejectedBrokerStaysUp() {
	status, _ := this.post("/api/participants", namesParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("names_service", "1",
		contractFragment{"pets.yaml", namesCyclicEndpointsYAML},
		contractFragment{"schemas.yaml", namesCyclicSchemasYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.too_deep","path":"schemas;Owner","source":"schemas.yaml","details":{"schema":"Owner","maxDepth":"10"}},`+
		`{"code":"schema.too_deep","path":"schemas;Pet","source":"schemas.yaml","details":{"schema":"Pet","maxDepth":"10"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))

	status, _ = this.post("/api/participants", `{"participant":"still_alive"}`)
	this.Equal(http.StatusOK, status)
}
