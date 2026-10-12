package integration_test

import (
	"context"
	"net/http"
)

const numberTypeParticipantBody = `{"participant":"priced_service"}`

const pricedProductYAML = `provides:
  rest:
    /products:
      get:
        responses:
          200: PricedProduct
schemas:
  PricedProduct:
    type: object
    properties:
      price:
        type: number
`

func (this *IntegrationSuite) TestPublishContract_NumberProperty_StoredWithTheNumberType() {
	status, _ := this.post("/api/participants", numberTypeParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("priced_service", "1",
		contractFragment{"products.yaml", pricedProductYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	var storedType string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT field_versions.type
		   FROM fields
		   JOIN field_versions ON field_versions.field_id = fields.id
		  WHERE fields.path = 'price'`,
	).Scan(&storedType))
	this.Equal("number", storedType)
}
