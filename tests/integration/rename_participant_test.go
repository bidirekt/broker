package integration_test

import (
	"context"
	"net/http"
)

const (
	renamePetsBody   = `{"participant":"pets_service"}`
	renameOrdersBody = `{"participant":"orders_service"}`
)

const renameV1ContractBody = `
{
  "provides": {
    "rest": {
      "/things": {
        "get": {
          "responses": {
            "200": "Thing"
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

func (this *IntegrationSuite) TestRenameParticipant_NewNameNotSnakeCase_Rejected() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"Pets-Service"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant name must be snake_case"}`, body)

	// the participant is still findable under its old name: nothing was renamed
	this.Positive(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_BeforePublishPreservesIdentity() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant renamed"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("orders_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_AfterPublishIsRejected() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"participant has published contracts, create a new participant and switch every consumer's consumes to it"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_OntoExistingNameIsRejectedNeverMerged() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	status, _ = this.post("/api/participants", renameOrdersBody)
	this.Require().Equal(http.StatusOK, status)

	petsID := this.renameParticipantID("pets_service")
	ordersID := this.renameParticipantID("orders_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"participant already exists"}`, body)

	this.Equal(2, this.countRows("participants"))
	this.Equal(petsID, this.renameParticipantID("pets_service"))
	this.Equal(ordersID, this.renameParticipantID("orders_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_UnknownParticipantReturns404() {
	status, body := this.post("/api/participants/rename", `{"oldName":"unknown_service","newName":"orders_service"}`)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"participant not found"}`, body)

	this.Equal(0, this.countRows("participants"))
}

func (this *IntegrationSuite) TestRenameParticipant_MissingNewNameReturns400() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant invalid input"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.NotZero(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_EmptyNewNameReturns400() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":""}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant invalid input"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.NotZero(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_SameNameIsNoOpSuccess() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"pets_service"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant renamed"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) renameParticipantID(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}
