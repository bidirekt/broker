package integration_test

import (
	"context"
	"net/http"

	"github.com/bidirekt/broker/internal/model"
)

const (
	renameEnvironmentPath        = "/api/environments/rename"
	renameEnvironmentParticipant = "orders"
	renameEnvironmentEmail       = "rhea@x.com"
	renameEnvironmentPassword    = "rename-environment-correct-horse"
	renameEnvironmentYAML        = `provides:
  rest:
    /orders:
      get:
        responses:
          204: ~
`
)

func (this *IntegrationSuite) renameEnvironmentCreate(name string) (id int64) {
	status, body := this.post("/api/environments", `{"environment":"`+name+`"}`)
	this.Require().Equal(http.StatusOK, status, body)

	return this.renameEnvironmentID(name)
}

func (this *IntegrationSuite) renameEnvironmentID(name string) (id int64) {
	err := this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return 0
	}

	return id
}

func (this *IntegrationSuite) TestRenameEnvironment_RenamesAndKeepsTheIdentity() {
	productionID := this.renameEnvironmentCreate("production")

	status, body := this.post(renameEnvironmentPath, `{"oldName":"production","newName":"prod"}`)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environment renamed"}`, body)
	this.Equal(1, this.countRows("environments"))
	this.Equal(productionID, this.renameEnvironmentID("prod"))
	this.Zero(this.renameEnvironmentID("production"))
}

func (this *IntegrationSuite) TestRenameEnvironment_SameNameAnswersRenamedAndChangesNothing() {
	productionID := this.renameEnvironmentCreate("production")

	status, body := this.post(renameEnvironmentPath, `{"oldName":"production","newName":"production"}`)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environment renamed"}`, body)
	this.Equal(1, this.countRows("environments"))
	this.Equal(productionID, this.renameEnvironmentID("production"))
}

func (this *IntegrationSuite) TestRenameEnvironment_EmptyOrPaddedNameIsInvalidInput() {
	productionID := this.renameEnvironmentCreate("production")

	for name, request := range map[string]string{
		"empty old name":            `{"oldName":"","newName":"prod"}`,
		"missing old name":          `{"newName":"prod"}`,
		"trailing space old name":   `{"oldName":"production ","newName":"prod"}`,
		"leading tab old name":      `{"oldName":"\tproduction","newName":"prod"}`,
		"empty new name":            `{"oldName":"production","newName":""}`,
		"missing new name":          `{"oldName":"production"}`,
		"leading space new name":    `{"oldName":"production","newName":" prod"}`,
		"trailing newline new name": `{"oldName":"production","newName":"prod\n"}`,
		"malformed body":            `{"oldName":`,
	} {
		this.Run(name, func() {
			status, body := this.post(renameEnvironmentPath, request)

			this.Equal(http.StatusBadRequest, status)
			this.JSONEq(`{"message":"environment invalid input"}`, body)
		})
	}

	this.Equal(1, this.countRows("environments"))
	this.Equal(productionID, this.renameEnvironmentID("production"))
}

func (this *IntegrationSuite) TestRenameEnvironment_UnknownOldNameIsNotFound() {
	status, body := this.post(renameEnvironmentPath, `{"oldName":"production","newName":"prod"}`)

	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"environment not found"}`, body)
	this.Equal(0, this.countRows("environments"))
}

func (this *IntegrationSuite) TestRenameEnvironment_TakenNewNameIsAlreadyExistsAndMergesNothing() {
	productionID := this.renameEnvironmentCreate("production")
	stagingID := this.renameEnvironmentCreate("staging")

	status, body := this.post(renameEnvironmentPath, `{"oldName":"production","newName":"staging"}`)

	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"environment already exists"}`, body)
	this.Equal(2, this.countRows("environments"))
	this.Equal(productionID, this.renameEnvironmentID("production"))
	this.Equal(stagingID, this.renameEnvironmentID("staging"))
}

func (this *IntegrationSuite) TestRenameEnvironment_ReadTokenIsForbidden() {
	productionID := this.renameEnvironmentCreate("production")
	this.createActiveUser(renameEnvironmentEmail, model.PermissionRead, renameEnvironmentPassword)
	token := this.signInToApi(renameEnvironmentEmail, renameEnvironmentPassword)

	status, _, body := this.postWithToken(renameEnvironmentPath, token, `{"oldName":"production","newName":"prod"}`)

	this.Equal(http.StatusForbidden, status)
	this.JSONEq(`{"message":"forbidden"}`, body)
	this.Equal(productionID, this.renameEnvironmentID("production"))
}

func (this *IntegrationSuite) TestRenameEnvironment_DeploymentsAndChecksStayAttached() {
	productionID := this.renameEnvironmentCreate("production")
	for _, request := range []struct{ path, body string }{
		{"/api/participants", `{"participant":"` + renameEnvironmentParticipant + `"}`},
		{"/api/contracts", this.publishBody(renameEnvironmentParticipant, "v1", contractFragment{"api.yaml", renameEnvironmentYAML})},
		{"/api/can-i-deploy", `{"participant":"` + renameEnvironmentParticipant + `","version":"v1","environment":"production"}`},
		{"/api/deployments", `{"participant":"` + renameEnvironmentParticipant + `","version":"v1","environment":"production"}`},
	} {
		status, body := this.post(request.path, request.body)
		this.Require().Equal(http.StatusOK, status, request.path+": "+body)
	}
	this.Require().Equal(1, this.countRows("compatibility_checks"))
	this.Require().Equal(1, this.countRows("deployments"))

	status, body := this.post(renameEnvironmentPath, `{"oldName":"production","newName":"prod"}`)
	this.Require().Equal(http.StatusOK, status, body)

	ctx := context.Background()
	var deploymentEnvironmentID, currentDeploymentEnvironmentID, checkEnvironmentID int64
	var checkEnvironmentName string
	this.Require().NoError(this.Pool.QueryRow(ctx, `SELECT environment_id FROM deployments`).Scan(&deploymentEnvironmentID))
	this.Require().NoError(this.Pool.QueryRow(ctx, `SELECT environment_id FROM current_deployments`).Scan(&currentDeploymentEnvironmentID))
	this.Require().NoError(this.Pool.QueryRow(ctx, `SELECT environment_id, environment_name FROM compatibility_checks`).Scan(&checkEnvironmentID, &checkEnvironmentName))
	this.Equal(productionID, deploymentEnvironmentID)
	this.Equal(productionID, currentDeploymentEnvironmentID)
	this.Equal(productionID, checkEnvironmentID)
	this.Equal("production", checkEnvironmentName)

	status, body = this.post("/api/can-i-deploy", `{"participant":"`+renameEnvironmentParticipant+`","version":"v1","environment":"prod"}`)
	this.Equal(http.StatusOK, status, body)
}
