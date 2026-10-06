package integration_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const ordersParticipantBody = `{"participant":"orders_service"}`

const ordersContractBody = `{
  "provides": {
    "rest": {
      "/orders": {
        "get": {
          "responses": {
            "200": "Order"
          }
        }
      }
    }
  },
  "schemas": {
    "Order": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

func (this *IntegrationSuite) publishOrdersContract() {
	status, _ := this.post("/api/participants", ordersParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("orders_service", "1", contractFragment{"api.yaml", ordersContractBody}))
	this.Require().Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) loadOrdersResource() model.PersistedResource {
	repo := repository.NewContractRepository(this.Pool)

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "orders_service", "1")
	this.Require().True(found)
	this.Require().Len(contract.Resources, 1)

	for _, resource := range contract.Resources {
		return resource
	}
	return model.PersistedResource{}
}

func (this *IntegrationSuite) TestLoadContract_AbsentOptionalFieldsMarshalAsNull() {
	this.publishOrdersContract()

	resource := this.loadOrdersResource()

	this.False(resource.ConsumedProvider.Valid, "expected ConsumedProvider to be null for a provided resource")
	this.Empty(resource.ParticipantVersion, "expected ParticipantVersion to be empty when not deployed")

	provider, err := json.Marshal(resource.ConsumedProvider)
	this.Require().NoError(err)
	this.Equal("null", string(provider))
}

func (this *IntegrationSuite) TestLoadContract_LegacyEmptyStringFieldsSurfaceAsNull() {
	this.publishOrdersContract()

	_, err := this.Pool.Exec(context.Background(),
		`UPDATE resources SET consumed_provider = '', response_status_code = ''`,
	)
	this.Require().NoError(err)

	resource := this.loadOrdersResource()

	this.False(resource.ConsumedProvider.Valid, "expected ConsumedProvider to be null for empty value")
	this.False(resource.ResponseStatusCode.Valid, "expected ResponseStatusCode to be null for empty value")

	statusCode, err := json.Marshal(resource.ResponseStatusCode)
	this.Require().NoError(err)
	this.Equal("null", string(statusCode))
}

func (this *IntegrationSuite) TestLoadContract_PopulatedOptionalFieldsArePreserved() {
	this.publishOrdersContract()

	_, err := this.Pool.Exec(context.Background(),
		`UPDATE resources SET consumed_provider = 'pets'`,
	)
	this.Require().NoError(err)

	resource := this.loadOrdersResource()

	this.True(resource.ConsumedProvider.Valid)
	this.Equal("pets", resource.ConsumedProvider.String)
	this.True(resource.ResponseStatusCode.Valid)
	this.Equal("200", resource.ResponseStatusCode.String)
}
