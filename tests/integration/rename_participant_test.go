package integration_test

import (
	"context"
	"fmt"
	"net/http"
)

const (
	renamePetsBody          = `{"participant":"pets_service"}`
	renameOrdersBody        = `{"participant":"orders_service"}`
	renameProductionEnvBody = `{"environment":"production"}`
	renameV1DeploymentBody  = `{"participant":"pets_service","version":"v1","environment":"production"}`
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

func (this *IntegrationSuite) TestRenameParticipant_SuccessPreservesIdentityAndReferences() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", renameProductionEnvBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", renameV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	originalID := this.renameParticipantID("pets_service")
	contractsBefore := this.countRows("contracts")
	resourcesBefore := this.countRows("resources")
	deploymentsBefore := this.countRows("deployments")
	this.Require().Positive(resourcesBefore)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant renamed"}`, body)

	var (
		idAfter   int64
		nameAfter string
	)
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id, name FROM participants WHERE id = $1`, originalID,
	).Scan(&idAfter, &nameAfter)
	this.Require().NoError(err)
	this.Equal(originalID, idAfter)
	this.Equal("orders_service", nameAfter)
	this.Equal(1, this.countRows("participants"))

	this.Equal(contractsBefore, this.countRows("contracts"))
	this.Equal(resourcesBefore, this.countRows("resources"))
	this.Equal(deploymentsBefore, this.countRows("deployments"))
	this.Equal(contractsBefore, this.renameRowsReferencing("contracts", originalID))
	this.Equal(resourcesBefore, this.renameRowsReferencing("resources", originalID))
	this.Equal(deploymentsBefore, this.renameRowsReferencing("deployments", originalID))
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

func (this *IntegrationSuite) renameRowsReferencing(table string, participantID int64) int {
	var count int
	err := this.Pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s WHERE participant_id = $1", table), participantID,
	).Scan(&count)
	this.Require().NoError(err)
	return count
}
