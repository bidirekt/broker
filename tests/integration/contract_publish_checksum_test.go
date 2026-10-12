package integration_test

import (
	"net/http"
	"strings"
)

const checksumOrdersContract = `provides:
  rest:
    /orders:
      post:
        request: Order
        responses:
          201: ~
schemas:
  Order:
    type: object
    properties:
      billing:
        ref: Address
  Address:
    type: object
    properties:
      city:
        type: string
`

const checksumOrdersReorderedContract = `schemas:
  Address:
    properties:
      city:
        type: string
        description: the city the invoice goes to
    type: object
  Order:
    properties:
      billing:
        ref: Address
    type: object
provides:
  rest:
    /orders:
      post:
        responses:
          201: ~
        request: Order
`

const checksumOrdersSplitRestContract = `provides:
  rest:
    /orders:
      post:
        request: Order
        responses:
          201: ~
`

const checksumOrdersSplitSchemasContract = `schemas:
  Order:
    type: object
    properties:
      billing:
        ref: Address
  Address:
    type: object
    properties:
      city:
        type: string
`

func (this *IntegrationSuite) mustPostForChecksum(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) TestPublish_SameVersionWithARenamedSchema_Returns409() {
	this.mustPostForChecksum("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", checksumOrdersContract}))

	status, body := this.post("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", strings.ReplaceAll(checksumOrdersContract, "Address", "PostalAddress")}))

	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"contract version already exists with different content"}`, body)
	this.Equal(1, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublish_RenamedSchemaInANewVersion_CreatesASnapshot() {
	this.mustPostForChecksum("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", checksumOrdersContract}))
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "2",
		contractFragment{"api.yaml", strings.ReplaceAll(checksumOrdersContract, "Address", "PostalAddress")}))

	this.Equal(2, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublish_SameContentInANewVersion_AliasesTheSnapshot() {
	this.mustPostForChecksum("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", checksumOrdersContract}))
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "2",
		contractFragment{"api.yaml", checksumOrdersContract}))

	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))
	this.Equal(2, this.countRows("schemas"))
}

func (this *IntegrationSuite) TestPublish_KeyOrderAndDescriptions_DoNotChangeTheChecksum() {
	this.mustPostForChecksum("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", checksumOrdersContract}))

	status, body := this.post("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", checksumOrdersReorderedContract}))

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublish_FileOrder_DoesNotChangeTheChecksum() {
	this.mustPostForChecksum("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForChecksum("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"rest.yaml", checksumOrdersSplitRestContract},
		contractFragment{"schemas.yaml", checksumOrdersSplitSchemasContract}))

	status, body := this.post("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"schemas.yaml", checksumOrdersSplitSchemasContract},
		contractFragment{"rest.yaml", checksumOrdersSplitRestContract}))

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
}
