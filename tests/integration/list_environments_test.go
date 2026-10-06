package integration_test

import (
	"net/http"
)

func (this *IntegrationSuite) TestListEnvironments_NoneIsAnEmptyArray() {
	status, body := this.get("/api/environments")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environments listed successfully","environments":[]}`, body)
}

func (this *IntegrationSuite) TestListEnvironments_SortedByteWise() {
	for _, name := range []string{"staging", "production", "QA"} {
		status, _ := this.post("/api/environments", `{"environment":"`+name+`"}`)
		this.Require().Equal(http.StatusOK, status)
	}

	status, body := this.get("/api/environments")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"environments listed successfully","environments":["QA","production","staging"]}`, body)
}
