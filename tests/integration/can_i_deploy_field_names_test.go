package integration_test

import (
	"encoding/json"
	"net/http"
)

type fieldNamesLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type fieldNamesResult struct {
	Endpoints map[string]map[string]map[string][]fieldNamesLeaf `json:"endpoints"`
}

type fieldNamesResponse struct {
	Results map[string]fieldNamesResult `json:"results"`
}

const fieldNamesAPIContract = `provides:
  rest:
    /invoices/*:
      get:
        responses:
          200: Invoice
schemas:
  Invoice:
    type: object
    properties:
      total: { type: string }
      billing: { ref: PostalAddress }
  PostalAddress:
    type: object
    properties:
      street: { type: string }
`

const fieldNamesSPAContract = `consumes:
  invoices_api:
    rest:
      /invoices/*:
        get:
          responses:
            200: InvoiceView
schemas:
  InvoiceView:
    type: object
    properties:
      total: { type: integer }
      billing: { ref: Address }
  Address:
    type: object
    properties:
      city: { type: string }
`

func (this *IntegrationSuite) mustPostForFieldNames(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForFieldNames(participant string) fieldNamesResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"1.0.0","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s: %s", participant, body)

	var parsed fieldNamesResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) TestCanIDeploy_PropertyBreaksNameTheFieldOfEachSide() {
	this.mustPostForFieldNames("/api/environments", `{"environment":"production"}`)
	this.mustPostForFieldNames("/api/participants", `{"participant":"invoices_api"}`)
	this.mustPostForFieldNames("/api/participants", `{"participant":"invoices_spa"}`)
	this.mustPostForFieldNames("/api/contracts",
		this.publishBody("invoices_api", "1.0.0", contractFragment{"api.yaml", fieldNamesAPIContract}))
	this.mustPostForFieldNames("/api/deployments",
		`{"participant":"invoices_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForFieldNames("/api/contracts",
		this.publishBody("invoices_spa", "1.0.0", contractFragment{"spa.yaml", fieldNamesSPAContract}))

	expected := func(role string) []fieldNamesLeaf {
		return []fieldNamesLeaf{
			{Reason: "property_missing_in_provider", Role: role, Details: map[string]string{
				"property":      "$.billing.city",
				"consumerField": "Address.city",
				"consumerName":  "invoices_spa",
				"providerName":  "invoices_api",
				"propertyType":  "string",
			}},
			{Reason: "property_type_mismatch", Role: role, Details: map[string]string{
				"property":             "$.total",
				"consumerField":        "InvoiceView.total",
				"providerField":        "Invoice.total",
				"consumerName":         "invoices_spa",
				"providerName":         "invoices_api",
				"consumerPropertyType": "integer",
				"providerPropertyType": "string",
			}},
		}
	}

	fromTheConsumer := this.canIDeployForFieldNames("invoices_spa")
	this.Equal(expected("consumer"), fromTheConsumer.Results["invoices_api"].Endpoints["/invoices/*"]["get"]["200"])

	this.mustPostForFieldNames("/api/deployments",
		`{"participant":"invoices_spa","version":"1.0.0","environment":"production"}`)

	replayedToTheProvider := this.canIDeployForFieldNames("invoices_api")
	this.Equal(expected("provider"), replayedToTheProvider.Results["invoices_spa"].Endpoints["/invoices/*"]["get"]["200"])
}
