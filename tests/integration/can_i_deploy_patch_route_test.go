package integration_test

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
)

const patchRouteProviderContract = `provides:
  rest:
    /pets/*:
      patch:
        request: PetChanges
        responses:
          200: Pet
schemas:
  PetChanges:
    type: object
    properties:
      name:
        type: string
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const patchRouteConsumerContract = `consumes:
  pets_api:
    rest:
      /pets/*:
        patch:
          request: PetChanges
          responses:
            200: Pet
schemas:
  PetChanges:
    type: object
    properties:
      nickname:
        type: string
  Pet:
    type: object
    properties:
      id:
        type: string
      photoUrl:
        type: string
`

func (this *IntegrationSuite) mustPostForPatchRoute(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) patchRouteResources() []string {
	rows, err := this.Pool.Query(context.Background(),
		`SELECT concat_ws(' ', direction, consumed_provider, interaction, method, endpoint, response_status_code)
		   FROM resources
		  ORDER BY direction, interaction`)
	this.Require().NoError(err)

	resources, err := pgx.CollectRows(rows, pgx.RowTo[string])
	this.Require().NoError(err)

	return resources
}

func (this *IntegrationSuite) TestPatchRoute_PublishesOnBothSidesAndCanIDeployReportsItsBreaksUnderPatch() {
	this.mustPostForPatchRoute("/api/environments", `{"environment":"production"}`)
	this.mustPostForPatchRoute("/api/participants", `{"participant":"pets_api"}`)
	this.mustPostForPatchRoute("/api/participants", `{"participant":"pets_web"}`)

	status, body := this.post("/api/contracts",
		this.publishBody("pets_api", "1.0.0", contractFragment{"api.yaml", patchRouteProviderContract}))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	status, body = this.post("/api/contracts",
		this.publishBody("pets_web", "1.0.0", contractFragment{"web.yaml", patchRouteConsumerContract}))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal([]string{
		"consumes pets_api rest_request patch /pets/*",
		"consumes pets_api rest_response patch /pets/* 200",
		"provides rest_request patch /pets/*",
		"provides rest_response patch /pets/* 200",
	}, this.patchRouteResources())

	this.mustPostForPatchRoute("/api/deployments",
		`{"participant":"pets_api","version":"1.0.0","environment":"production"}`)

	status, body = this.post("/api/can-i-deploy",
		`{"participant":"pets_web","version":"1.0.0","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "pets_web",
		"version": "1.0.0",
		"environment": "production",
		"deployable": false,
		"results": {
			"pets_api": {
				"deployable": false,
				"participantVersion": "1.0.0",
				"endpoints": {
					"/pets/*": {
						"patch": {
							"request": [{
								"reason": "property_missing_in_consumer",
								"role": "consumer",
								"details": {"property": "$.name", "consumerName": "pets_web", "providerName": "pets_api", "propertyType": "string"}
							}],
							"200": [{
								"reason": "property_missing_in_provider",
								"role": "consumer",
								"details": {"property": "$.photoUrl", "consumerName": "pets_web", "providerName": "pets_api", "propertyType": "string"}
							}]
						}
					}
				}
			}
		}
	}`, body)
}
