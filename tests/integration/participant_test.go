package integration_test

import (
	"net/http"
)

const petsParticipantBody = `{"participant":"pets_service"}`

func (this *IntegrationSuite) TestHappyPath_CreateParticipant() {
	status, body := this.post("/api/participants", petsParticipantBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant created"}`, body)

	this.Equal(1, this.countRows("participants"))
}

func (this *IntegrationSuite) TestCreateParticipant_NameNotSnakeCase_Rejected() {
	for _, name := range []string{"Pets-Service", "pets service", "PETS", "pets_"} {
		status, body := this.post("/api/participants", `{"participant":"`+name+`"}`)
		this.Equal(http.StatusBadRequest, status)
		this.JSONEq(`{"message":"participant name must be snake_case"}`, body)
	}

	this.Equal(0, this.countRows("participants"))
}

func (this *IntegrationSuite) TestIdempotent_DuplicateParticipantName() {
	status, _ := this.post("/api/participants", petsParticipantBody)
	this.Equal(http.StatusOK, status)

	status, body := this.post("/api/participants", petsParticipantBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant already exists"}`, body)

	this.Equal(1, this.countRows("participants"))
}
