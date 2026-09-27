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

func (s *IntegrationSuite) publishOrdersContract() {
	status, _ := s.post("/api/participants", ordersParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("orders_service", "1", contractFragment{"api.yaml", ordersContractBody}))
	s.Require().Equal(http.StatusOK, status)
}

func (s *IntegrationSuite) loadOrdersResource() model.PersistedResource {
	repo := repository.NewContractRepository(s.Pool)

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "orders_service", "1")
	s.Require().True(found)
	s.Require().Len(contract.Resources, 1)

	for _, resource := range contract.Resources {
		return resource
	}
	return model.PersistedResource{}
}

func (s *IntegrationSuite) TestLoadContract_AbsentOptionalFieldsMarshalAsNull() {
	s.publishOrdersContract()

	resource := s.loadOrdersResource()

	s.False(resource.ConsumedProvider.Valid, "expected ConsumedProvider to be null for a provided resource")
	s.Empty(resource.ParticipantVersion, "expected ParticipantVersion to be empty when not deployed")

	provider, err := json.Marshal(resource.ConsumedProvider)
	s.Require().NoError(err)
	s.Equal("null", string(provider))
}

func (s *IntegrationSuite) TestLoadContract_LegacyEmptyStringFieldsSurfaceAsNull() {
	s.publishOrdersContract()

	_, err := s.Pool.Exec(context.Background(),
		`UPDATE resources SET consumed_provider = '', response_status_code = ''`,
	)
	s.Require().NoError(err)

	resource := s.loadOrdersResource()

	s.False(resource.ConsumedProvider.Valid, "expected ConsumedProvider to be null for empty value")
	s.False(resource.ResponseStatusCode.Valid, "expected ResponseStatusCode to be null for empty value")

	statusCode, err := json.Marshal(resource.ResponseStatusCode)
	s.Require().NoError(err)
	s.Equal("null", string(statusCode))
}

func (s *IntegrationSuite) TestLoadContract_PopulatedOptionalFieldsArePreserved() {
	s.publishOrdersContract()

	_, err := s.Pool.Exec(context.Background(),
		`UPDATE resources SET consumed_provider = 'pets'`,
	)
	s.Require().NoError(err)

	resource := s.loadOrdersResource()

	s.True(resource.ConsumedProvider.Valid)
	s.Equal("pets", resource.ConsumedProvider.String)
	s.True(resource.ResponseStatusCode.Valid)
	s.Equal("200", resource.ResponseStatusCode.String)
}
