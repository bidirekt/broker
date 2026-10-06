package integration_test

import (
	"net/http"
)

func (this *IntegrationSuite) TestListParticipants_NoneIsAnEmptyArray() {
	status, body := this.get("/api/participants")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participants listed successfully","participants":[]}`, body)
}

func (this *IntegrationSuite) TestListParticipants_SortedByteWise() {
	for _, name := range []string{"ordersapi", "orders_web", "orders2"} {
		status, _ := this.post("/api/participants", `{"participant":"`+name+`"}`)
		this.Require().Equal(http.StatusOK, status)
	}

	status, body := this.get("/api/participants")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participants listed successfully","participants":["orders2","orders_web","ordersapi"]}`, body)
}
