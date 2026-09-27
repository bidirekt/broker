package integration_test

import (
	"context"
	"net/http"
)

const contractBody = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": {
            "200": "Pet"
          }
        }
      }
    }
  },
  "schemas": {
    "Pet": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const contractBodyAlt = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": {
            "200": "Pet"
          }
        }
      }
    }
  },
  "schemas": {
    "Pet": {
      "type": "object",
      "properties": {
        "id": { "type": "integer" },
        "name": { "type": "string" }
      }
    }
  }
}`

const contractBodyBadServiceName = `{
  "consumes": {
    "Payments-API": {
      "rest": {
        "/invoices": {
          "get": {
            "responses": {
              "200": "Invoice"
            }
          }
        }
      }
    }
  },
  "schemas": {
    "Invoice": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const contractBodyParamEndpoint = `{
  "provides": {
    "rest": {
      "/users/{userId}": {
        "get": {
          "responses": {
            "200": "User"
          }
        }
      }
    }
  },
  "schemas": {
    "User": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const contractBodyEndpointsFragment = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": {
            "200": "Pet"
          }
        }
      }
    }
  }
}`

const contractBodySchemasFragment = `{
  "schemas": {
    "Pet": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const petsEndpointsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
`

const petsSchemasYAML = `schemas:
  Pets:
    type: array
    items:
      ref: Pet
  Pet:
    type: object
    properties:
      petId:
        type: integer
`

const storeEndpointsYAML = `provides:
  rest:
    /pets:
      delete:
        responses:
          204: Pets
`

const storeTrailingSlashYAML = `provides:
  rest:
    /pets/:
      get:
        responses:
          200: Pets
`

const billingSchemasYAML = `schemas:
  Pet:
    type: object
    properties:
      petId:
        type: integer
`

const petsRequestAndCreatedYAML = `provides:
  rest:
    /pets:
      post:
        request: Pets
        responses:
          201: Pets
`

const petsNotFoundOnlyYAML = `provides:
  rest:
    /pets:
      post:
        responses:
          404: Pets
`

const invoicesConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Invoice
`

const invoicesSchemasYAML = `schemas:
  Invoice:
    type: object
    properties:
      total:
        type: integer
`

func (s *IntegrationSuite) TestHappyPath_PublishContract() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBody}))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(1, s.countRows("contracts"))
	s.Equal(1, s.countRows("contract_versions"))
	s.Equal(1, s.countRows("resources"))
	s.GreaterOrEqual(s.countRows("properties"), 1)

	var version string
	err := s.Pool.QueryRow(context.Background(),
		"SELECT version FROM contract_versions LIMIT 1",
	).Scan(&version)
	s.Require().NoError(err)
	s.Equal("1", version)
}

func (s *IntegrationSuite) TestPublish_SameVersionSameContent_Returns200NoNewRow() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBody}))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(1, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublish_SameVersionDifferentContent_Returns409() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBodyAlt}))
	s.Equal(http.StatusConflict, status)
	s.JSONEq(`{"message":"contract version already exists with different content"}`, body)

	s.Equal(1, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_MissingContracts() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", `{"participant":"pets_service","version":"a1b2c3d"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract invalid input"}`, body)
}

func (s *IntegrationSuite) TestPublishContract_EmptyContracts() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", `{"participant":"pets_service","version":"a1b2c3d","contracts":[]}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract invalid input"}`, body)
}

func (s *IntegrationSuite) TestPublishContract_BlankParticipant() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	for _, participant := range []string{"", "   "} {
		status, body := s.post("/api/contracts", s.publishBody(participant, "1", contractFragment{"api.yaml", contractBody}))
		s.Equal(http.StatusBadRequest, status)
		s.JSONEq(`{"message":"contract invalid input"}`, body)
	}
}

func (s *IntegrationSuite) TestPublishContract_BlankSource() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"  ", contractBody}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract invalid input"}`, body)
}

func (s *IntegrationSuite) TestPublishContract_UnsupportedExtension() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"notes.txt", contractBody}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"unsupported contract file: notes.txt (expected .yaml or .yml)"}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_MalformedYAML() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"broken.yaml", "provides: {"}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"malformed contract file: broken.yaml: [1:11] could not find flow mapping end token '}'\n>  1 | provides: {\n                 ^\n"}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_JSONExtensionRejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"broken.json", contractBody}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"unsupported contract file: broken.json (expected .yaml or .yml)"}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_CommitHashVersion() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "a1b2c3d4e5f6", contractFragment{"api.yaml", contractBody}))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	var version string
	err := s.Pool.QueryRow(context.Background(),
		"SELECT version FROM contract_versions LIMIT 1",
	).Scan(&version)
	s.Require().NoError(err)
	s.Equal("a1b2c3d4e5f6", version)
}

func (s *IntegrationSuite) TestPublishContract_ParamEndpoint_RejectedNothingStored() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBodyParamEndpoint}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"endpoint.syntax","path":"provides;rest;/users/{userId}","source":"api.yaml","details":{"key":"/users/{userId}","error":"dynamic path segments must use *"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_BadServiceName_RejectedNothingStored() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1", contractFragment{"api.yaml", contractBodyBadServiceName}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"service.name_syntax","path":"consumes;Payments-API","source":"api.yaml","details":{"key":"Payments-API","error":"must be snake_case"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_UnknownParticipant() {
	status, body := s.post("/api/contracts", s.publishBody("ghost_service", "1", contractFragment{"api.yaml", contractBody}))
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"contract participant not found"}`, body)
}

func (s *IntegrationSuite) TestPublishContract_RefCrossesFragments() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(1, s.countRows("resources"))

	rows, err := s.Pool.Query(context.Background(), "SELECT path FROM properties ORDER BY path")
	s.Require().NoError(err)
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		s.Require().NoError(rows.Scan(&path))
		paths = append(paths, path)
	}
	s.Equal([]string{"$", "$[]", "$[].petId"}, paths)
}

func (s *IntegrationSuite) TestPublishContract_EmptyFragmentContributesNothing() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
		contractFragment{"empty.yaml", ""},
	))
	s.Equal(http.StatusOK, status)

	s.Equal(1, s.countRows("resources"))
}

func (s *IntegrationSuite) TestPublishContract_DifferentMethodsSamePath_Merge() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"store.yaml", storeEndpointsYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(2, s.countRows("resources"))
}

func (s *IntegrationSuite) TestPublishContract_RequestInOneFragmentStatusInAnother_Merge() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsRequestAndCreatedYAML},
		contractFragment{"errors.yaml", petsNotFoundOnlyYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(3, s.countRows("resources"))
}

func (s *IntegrationSuite) TestPublishContract_DuplicateResponseResource_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"store.yaml", petsEndpointsYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"resource.duplicate","path":"provides;rest;/pets;get;responses;200","source":"store.yaml","details":{"resource":"provides GET /pets 200","declaredIn":"pets.yaml"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
	s.Equal(0, s.countRows("resources"))
}

func (s *IntegrationSuite) TestPublishContract_DuplicateRequestResource_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsRequestAndCreatedYAML},
		contractFragment{"store.yaml", petsRequestAndCreatedYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"resource.duplicate","path":"provides;rest;/pets;post;request","source":"store.yaml","details":{"resource":"provides POST /pets request","declaredIn":"pets.yaml"}},`+
		`{"code":"resource.duplicate","path":"provides;rest;/pets;post;responses;201","source":"store.yaml","details":{"resource":"provides POST /pets 201","declaredIn":"pets.yaml"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_IdenticalConsumedResourceInTwoFiles_MergesQuietly() {
	status, _ := s.post("/api/participants", `{"participant":"front"}`)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("front", "1",
		contractFragment{"a.yaml", invoicesConsumerYAML},
		contractFragment{"b.yaml", invoicesConsumerYAML},
		contractFragment{"schemas.yaml", invoicesSchemasYAML},
	))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(1, s.countRows("contracts"))
	s.Equal(1, s.countRows("resources"))
}

func (s *IntegrationSuite) TestPublishContract_TrailingSlashCountsAsDuplicate() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"store.yaml", storeTrailingSlashYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"resource.duplicate","path":"provides;rest;/pets;get;responses;200","source":"store.yaml","details":{"resource":"provides GET /pets 200","declaredIn":"pets.yaml"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_DuplicateSchema_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"pets.yaml", petsEndpointsYAML},
		contractFragment{"schemas.yaml", petsSchemasYAML},
		contractFragment{"billing.yaml", billingSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.duplicate","path":"schemas;Pet","source":"schemas.yaml","details":{"schema":"Pet","declaredIn":"billing.yaml"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_SameContentSplitInFragments_AliasesTheSnapshot() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("pets_service", "v42", contractFragment{"api.yaml", contractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "v43",
		contractFragment{"endpoints.yaml", contractBodyEndpointsFragment},
		contractFragment{"schemas.yaml", contractBodySchemasFragment},
	))
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"contract publish successful"}`, body)

	s.Equal(1, s.countRows("contracts"))
	s.Equal(2, s.countRows("contract_versions"))
}

const arrayWithoutItemsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
schemas:
  Pets:
    type: array
`

const duplicateEndpointYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
    /pets/:
      get:
        responses:
          200: Pets
`

const duplicateEndpointSchemasYAML = `schemas:
  Pets:
    type: array
    items:
      type: string
`

const manyViolationsYAML = `provides:
  rest:
    /users/*/{orderId}:
      get:
        responses:
          200: Order
    /pets:
      get:
        responses:
          200: Missing
`

const invalidSchemaTypesYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: strng
      tags:
        type: array
        items: {}
`

const invalidStatusCodesYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          -1: Pet
          999: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

func (s *IntegrationSuite) TestPublishContract_InvalidSchemaType_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", invalidSchemaTypesYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.invalid_type","path":"schemas;Pet;properties;id;type","source":"api.yaml","details":{"value":"strng","allowed":"object, array, string, integer, float, boolean"}},`+
		`{"code":"schema.invalid_type","path":"schemas;Pet;properties;tags;items","source":"api.yaml","details":{"value":"","allowed":"object, array, string, integer, float, boolean"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_StatusCodeOutOfRange_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", invalidStatusCodesYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"status.out_of_range","path":"provides;rest;/pets;get;responses;-1","source":"api.yaml","details":{"key":"-1","error":"must be between 100 and 599"}},`+
		`{"code":"status.out_of_range","path":"provides;rest;/pets;get;responses;999","source":"api.yaml","details":{"key":"999","error":"must be between 100 and 599"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_ArrayWithoutItems_RejectedBrokerStaysUp() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", arrayWithoutItemsYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.array_without_items","path":"schemas;Pets","source":"api.yaml","details":null}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))

	status, _ = s.post("/api/participants", `{"participant":"still_alive"}`)
	s.Equal(http.StatusOK, status)
}

func (s *IntegrationSuite) TestPublishContract_BothProvidedSpellingsInOneFile_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", duplicateEndpointYAML},
		contractFragment{"schemas.yaml", duplicateEndpointSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"resource.duplicate","path":"provides;rest;/pets;get;responses;200","source":"api.yaml","details":{"resource":"provides GET /pets 200","declaredIn":"api.yaml"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_ShapeViolation_ReportedBeforeContextualRules() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", manyViolationsYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"endpoint.syntax","path":"provides;rest;/users/*/{orderId}","source":"api.yaml","details":{"key":"/users/*/{orderId}","error":"dynamic path segments must use *"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

const unknownMethodYAML = `provides:
  rest:
    /pets:
      patch:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const messageBlockYAML = `provides:
  message:
    pets.created:
      payload: Pet
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const scalarMethodYAML = `provides:
  rest:
    /pets:
      get: text
`

const anchoredSchemasYAML = `schemas:
  Pet: &pet
    type: object
    properties:
      id:
        type: string
  Owner: *pet
`

const multiDocumentYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
---
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const parentAndNestedEndpointsOutOfRangeYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          999: Pet
    /pets/*:
      get:
        responses:
          999: Pet
`

const parentEndpointOutOfRangeYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          999: Pet
`

func (s *IntegrationSuite) TestPublishContract_UnknownMethod_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", unknownMethodYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;rest;/pets;patch","source":"api.yaml","details":{"key":"patch"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_MessageBlock_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", messageBlockYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;message","source":"api.yaml","details":{"key":"message"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_ScalarMethod_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", scalarMethodYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"value.invalid_kind","path":"provides;rest;/pets;get","source":"api.yaml","details":{"expected":"mapping","got":"string"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_AnchorsAndAliases_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"schemas.yaml", anchoredSchemasYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"malformed contract file: schemas.yaml: anchors and aliases are not supported"}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_MultipleDocuments_Rejected() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"api.yaml", multiDocumentYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"malformed contract file: api.yaml: multiple documents are not supported"}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_ShapeViolations_KeepDocumentOrderPerSource() {
	status, _ := s.post("/api/participants", petsParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/contracts", s.publishBody("pets_service", "1",
		contractFragment{"b.yaml", parentEndpointOutOfRangeYAML},
		contractFragment{"a.yaml", parentAndNestedEndpointsOutOfRangeYAML},
	))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"status.out_of_range","path":"provides;rest;/pets;get;responses;999","source":"a.yaml","details":{"key":"999","error":"must be between 100 and 599"}},`+
		`{"code":"status.out_of_range","path":"provides;rest;/pets/*;get;responses;999","source":"a.yaml","details":{"key":"999","error":"must be between 100 and 599"}},`+
		`{"code":"status.out_of_range","path":"provides;rest;/pets;get;responses;999","source":"b.yaml","details":{"key":"999","error":"must be between 100 and 599"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("contracts"))
}

func (s *IntegrationSuite) TestPublishContract_ShapeViolation_ReportedBeforeParticipantLookup() {
	status, body := s.post("/api/contracts", s.publishBody("ghost_service", "1", contractFragment{"api.yaml", contractBodyParamEndpoint}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"endpoint.syntax","path":"provides;rest;/users/{userId}","source":"api.yaml","details":{"key":"/users/{userId}","error":"dynamic path segments must use *"}}`+
		`]}`, body)

	s.Equal(0, s.countRows("participants"))
}
