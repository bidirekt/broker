package integration_test

import (
	"net/http"
)

const productionEnvironmentBody = `{"environment":"production"}`

func (this *IntegrationSuite) TestHappyPath_CreateEnvironment() {
	status, body := this.post("/api/environments", productionEnvironmentBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environment created"}`, body)

	this.Equal(1, this.countRows("environments"))
}

func (this *IntegrationSuite) TestIdempotent_DuplicateEnvironmentName() {
	status, _ := this.post("/api/environments", productionEnvironmentBody)
	this.Equal(http.StatusOK, status)

	status, body := this.post("/api/environments", productionEnvironmentBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environment already exists"}`, body)

	this.Equal(1, this.countRows("environments"))
}

func (this *IntegrationSuite) TestUnhappyPath_MissingEnvironmentName() {
	status, body := this.post("/api/environments", `{}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"environment invalid input"}`, body)

	this.Equal(0, this.countRows("environments"))
}

func (this *IntegrationSuite) TestUnhappyPath_EnvironmentNameUnderParticipantKey() {
	status, body := this.post("/api/environments", `{"participant":"production"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"environment invalid input"}`, body)

	this.Equal(0, this.countRows("environments"))
}
