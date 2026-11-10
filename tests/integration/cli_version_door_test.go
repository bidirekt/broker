package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

const cliVersionDoorHeader = "Bidirekt-Cli-Version"

func (this *IntegrationSuite) cliVersionDoorListParticipants(cliVersion string) (status int, body string) {
	req := httptest.NewRequest(http.MethodGet, "/api/participants", nil)
	if cliVersion != "" {
		req.Header.Set(cliVersionDoorHeader, cliVersion)
	}

	status, _, body = this.sendApiRequest(req, this.writerToken)

	return status, body
}

func (this *IntegrationSuite) cliVersionDoorBrokerVersion() string {
	status, _, body := this.sendApiRequest(httptest.NewRequest(http.MethodGet, "/health", nil), "")
	this.Require().Equal(http.StatusOK, status)

	var health struct {
		BrokerVersion string `json:"brokerVersion"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &health))

	return health.BrokerVersion
}

func (this *IntegrationSuite) TestCliVersionDoor_RefusesACliBelowTheMinimumNamingTheBrokerVersionHealthShows() {
	status, body := this.cliVersionDoorListParticipants("0.0.9")

	this.Equal(http.StatusUpgradeRequired, status)
	expected, err := json.Marshal(map[string]string{
		"message": "bidirekt 0.0.9 doesn't work with this broker (" + this.cliVersionDoorBrokerVersion() + "); upgrade bidirekt to 0.1.0 or later",
	})
	this.Require().NoError(err)
	this.JSONEq(string(expected), body)
}

func (this *IntegrationSuite) TestCliVersionDoor_RefusesBeforeAuthentication() {
	req := httptest.NewRequest(http.MethodGet, "/api/participants", nil)
	req.Header.Set(cliVersionDoorHeader, "0.0.9")

	status, _, _ := this.sendApiRequest(req, "")

	this.Equal(http.StatusUpgradeRequired, status)
}

func (this *IntegrationSuite) TestCliVersionDoor_LetsASupportedMissingOrInvalidVersionThrough() {
	for _, cliVersion := range []string{"0.1.0", "0.2.0-rc.1", "", "not-a-version"} {
		status, body := this.cliVersionDoorListParticipants(cliVersion)

		this.Equal(http.StatusOK, status, cliVersion)
		this.JSONEq(`{"message":"participants listed successfully","participants":[]}`, body, cliVersion)
	}
}

func (this *IntegrationSuite) TestCliVersionDoor_DoesNotCheckPages() {
	req := httptest.NewRequest(http.MethodGet, "/participants", nil)
	req.Header.Set(cliVersionDoorHeader, "0.0.9")

	status, _, _ := this.sendPageRequest(req, this.signInAsAdmin())

	this.Equal(http.StatusOK, status)
}
