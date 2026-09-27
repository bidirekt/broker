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

func (s *IntegrationSuite) TestRenameParticipant_NewNameNotSnakeCase_Rejected() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service","newName":"Pets-Service"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"participant name must be snake_case"}`, body)

	// the participant is still findable under its old name: nothing was renamed
	s.Positive(s.renameParticipantID("pets_service"))
}

func (s *IntegrationSuite) TestRenameParticipant_SuccessPreservesIdentityAndReferences() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/environments", renameProductionEnvBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/deployments", renameV1DeploymentBody)
	s.Require().Equal(http.StatusOK, status)

	originalID := s.renameParticipantID("pets_service")
	contractsBefore := s.countRows("contracts")
	resourcesBefore := s.countRows("resources")
	deploymentsBefore := s.countRows("deployments")
	s.Require().Positive(resourcesBefore)

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"participant renamed"}`, body)

	var (
		idAfter   int64
		nameAfter string
	)
	err := s.Pool.QueryRow(context.Background(),
		`SELECT id, name FROM participants WHERE id = $1`, originalID,
	).Scan(&idAfter, &nameAfter)
	s.Require().NoError(err)
	s.Equal(originalID, idAfter)
	s.Equal("orders_service", nameAfter)
	s.Equal(1, s.countRows("participants"))

	s.Equal(contractsBefore, s.countRows("contracts"))
	s.Equal(resourcesBefore, s.countRows("resources"))
	s.Equal(deploymentsBefore, s.countRows("deployments"))
	s.Equal(contractsBefore, s.renameRowsReferencing("contracts", originalID))
	s.Equal(resourcesBefore, s.renameRowsReferencing("resources", originalID))
	s.Equal(deploymentsBefore, s.renameRowsReferencing("deployments", originalID))
}

func (s *IntegrationSuite) TestRenameParticipant_OntoExistingNameIsRejectedNeverMerged() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)
	status, _ = s.post("/api/participants", renameOrdersBody)
	s.Require().Equal(http.StatusOK, status)

	petsID := s.renameParticipantID("pets_service")
	ordersID := s.renameParticipantID("orders_service")

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	s.Equal(http.StatusConflict, status)
	s.JSONEq(`{"message":"participant already exists"}`, body)

	s.Equal(2, s.countRows("participants"))
	s.Equal(petsID, s.renameParticipantID("pets_service"))
	s.Equal(ordersID, s.renameParticipantID("orders_service"))
}

func (s *IntegrationSuite) TestRenameParticipant_UnknownParticipantReturns404() {
	status, body := s.post("/api/participants/rename", `{"oldName":"unknown_service","newName":"orders_service"}`)
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"participant not found"}`, body)

	s.Equal(0, s.countRows("participants"))
}

func (s *IntegrationSuite) TestRenameParticipant_MissingNewNameReturns400() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"participant invalid input"}`, body)

	s.Equal(1, s.countRows("participants"))
	s.NotZero(s.renameParticipantID("pets_service"))
}

func (s *IntegrationSuite) TestRenameParticipant_EmptyNewNameReturns400() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service","newName":""}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"participant invalid input"}`, body)

	s.Equal(1, s.countRows("participants"))
	s.NotZero(s.renameParticipantID("pets_service"))
}

func (s *IntegrationSuite) TestRenameParticipant_SameNameIsNoOpSuccess() {
	status, _ := s.post("/api/participants", renamePetsBody)
	s.Require().Equal(http.StatusOK, status)
	originalID := s.renameParticipantID("pets_service")

	status, body := s.post("/api/participants/rename", `{"oldName":"pets_service","newName":"pets_service"}`)
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"participant renamed"}`, body)

	s.Equal(1, s.countRows("participants"))
	s.Equal(originalID, s.renameParticipantID("pets_service"))
}

func (s *IntegrationSuite) renameParticipantID(name string) int64 {
	var id int64
	err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *IntegrationSuite) renameRowsReferencing(table string, participantID int64) int {
	var count int
	err := s.Pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s WHERE participant_id = $1", table), participantID,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}
