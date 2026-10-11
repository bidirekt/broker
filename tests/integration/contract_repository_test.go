package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
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

const nullableAccountsProviderContract = `provides:
  rest:
    /accounts:
      get:
        responses:
          200: Account
schemas:
  Account:
    type: object
    properties:
      avatar:
        type: string
        nullable: %t
      nickname:
        type: string
`

const nullableAccountsConsumerContract = `consumes:
  accounts_api:
    rest:
      /accounts:
        get:
          responses:
            200: Account
schemas:
  Account:
    type: object
    properties:
      avatar:
        type: string
        nullable: true
`

func (this *IntegrationSuite) mustPostForNullableRoundTrip(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishNullableAccountsProvider(version string, avatarNullable bool) {
	this.mustPostForNullableRoundTrip("/api/contracts", this.publishBody("accounts_api", version,
		contractFragment{"api.yaml", fmt.Sprintf(nullableAccountsProviderContract, avatarNullable)}))
}

func onlyNullableRoundTripResource(contract *model.PersistedContract) model.PersistedResource {
	for _, resource := range contract.Resources {
		return resource
	}
	return model.PersistedResource{}
}

func (this *IntegrationSuite) TestContractRepository_NullablePropertyRoundTripsThroughEveryRead() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForNullableRoundTrip("/api/environments", `{"environment":"production"}`)
	this.mustPostForNullableRoundTrip("/api/participants", `{"participant":"accounts_api"}`)
	this.mustPostForNullableRoundTrip("/api/participants", `{"participant":"accounts_web"}`)
	this.publishNullableAccountsProvider("1", true)
	this.mustPostForNullableRoundTrip("/api/contracts", this.publishBody("accounts_web", "1",
		contractFragment{"web.yaml", nullableAccountsConsumerContract}))
	this.mustPostForNullableRoundTrip("/api/deployments", `{"participant":"accounts_api","version":"1","environment":"production"}`)
	this.mustPostForNullableRoundTrip("/api/deployments", `{"participant":"accounts_web","version":"1","environment":"production"}`)

	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(ctx,
		`SELECT id FROM environments WHERE name = 'production'`).Scan(&environmentID))

	byVersion, found := repo.GetContractByNameAndVersion(ctx, "accounts_api", "1")
	this.Require().True(found)
	providerResource := onlyNullableRoundTripResource(byVersion)
	this.True(providerResource.Properties["$.avatar"].Nullable, "contract by version")
	this.False(providerResource.Properties["$.avatar"].Optional, "contract by version")
	this.False(providerResource.Properties["$.nickname"].Nullable, "contract by version")

	latest, found := repo.GetLatestContractByName(ctx, "accounts_api")
	this.Require().True(found)
	this.True(onlyNullableRoundTripResource(latest).Properties["$.avatar"].Nullable, "latest contract")

	provider, err := repo.GetProviderResourceByConsumerResource(ctx, providerResource.ProviderHash, environmentID)
	this.Require().NoError(err)
	this.True(provider.Properties["$.avatar"].Nullable, "provider resource of a consumer")
	this.False(provider.Properties["$.nickname"].Nullable, "provider resource of a consumer")

	consumers := repo.GetConsumersResourcesByProviderHashAndEnvironmentID(ctx, providerResource.ProviderHash, environmentID)
	this.Require().Len(consumers, 1)
	this.True(consumers[0].Properties["$.avatar"].Nullable, "consumers of a provider")
}

func (this *IntegrationSuite) TestContractRepository_OnlyNullabilityChanged_WritesAModifiedPropertyVersion() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForNullableRoundTrip("/api/participants", `{"participant":"accounts_api"}`)
	this.publishNullableAccountsProvider("1", false)
	this.publishNullableAccountsProvider("2", true)

	rows, err := this.Pool.Query(ctx, `
		SELECT pv.change_type, pv.nullable
		FROM property_versions pv
		JOIN properties p ON p.id = pv.property_id
		WHERE p.path = '$.avatar'
		ORDER BY pv.contract_id`)
	this.Require().NoError(err)
	defer rows.Close()

	type avatarVersion struct {
		changeType string
		nullable   bool
	}
	var versions []avatarVersion
	for rows.Next() {
		var version avatarVersion
		this.Require().NoError(rows.Scan(&version.changeType, &version.nullable))
		versions = append(versions, version)
	}
	this.Require().NoError(rows.Err())

	this.Equal([]avatarVersion{{"added", false}, {"modified", true}}, versions)

	first, found := repo.GetContractByNameAndVersion(ctx, "accounts_api", "1")
	this.Require().True(found)
	this.False(onlyNullableRoundTripResource(first).Properties["$.avatar"].Nullable)

	second, found := repo.GetContractByNameAndVersion(ctx, "accounts_api", "2")
	this.Require().True(found)
	this.True(onlyNullableRoundTripResource(second).Properties["$.avatar"].Nullable)
}

const undescribedBodyProviderContract = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: ~
`

const describedBodyProviderContract = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const undescribedBodyProviderWithHealthContract = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: ~
    /health:
      get:
        responses:
          200: Health
schemas:
  Health:
    type: object
    properties:
      status:
        type: string
`

const undescribedBodyConsumerContract = `consumes:
  files_api:
    rest:
      /files/*:
        get:
          responses:
            200: ~
`

const describedBodyConsumerContract = `consumes:
  files_api:
    rest:
      /files/*:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

func (this *IntegrationSuite) mustPostForUndescribedBody(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishUndescribedBodyContract(participant, version, content string) {
	this.mustPostForUndescribedBody("/api/contracts", this.publishBody(participant, version, contractFragment{"api.yaml", content}))
}

func (this *IntegrationSuite) deployUndescribedBodyVersion(participant, version string) {
	this.mustPostForUndescribedBody("/api/deployments",
		fmt.Sprintf(`{"participant":%q,"version":%q,"environment":"production"}`, participant, version))
}

func (this *IntegrationSuite) undescribedBodyProductionID() int64 {
	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = 'production'`).Scan(&environmentID))

	return environmentID
}

func onlyUndescribedBodyResource(contract *model.PersistedContract) model.PersistedResource {
	for _, resource := range contract.Resources {
		return resource
	}
	return model.PersistedResource{}
}

func (this *IntegrationSuite) assertEveryReadReturnsTheFileResourceWithoutProperties(providerVersion string) {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)
	environmentID := this.undescribedBodyProductionID()

	byVersion, found := repo.GetContractByNameAndVersion(ctx, "files_api", providerVersion)
	this.Require().True(found, "contract by version")
	this.Require().Len(byVersion.Resources, 1, "contract by version")
	providerResource := onlyUndescribedBodyResource(byVersion)
	this.Equal("/files/*", providerResource.Endpoint, "contract by version")
	this.Equal(map[string]model.Property{}, providerResource.Properties, "contract by version")

	latest, found := repo.GetLatestContractByName(ctx, "files_api")
	this.Require().True(found, "latest contract")
	this.Require().Len(latest.Resources, 1, "latest contract")
	this.Equal(map[string]model.Property{}, onlyUndescribedBodyResource(latest).Properties, "latest contract")

	provider, err := repo.GetProviderResourceByConsumerResource(ctx, providerResource.ProviderHash, environmentID)
	this.Require().NoError(err, "provider resource of a consumer")
	this.Equal("/files/*", provider.Endpoint, "provider resource of a consumer")
	this.Equal(map[string]model.Property{}, provider.Properties, "provider resource of a consumer")
	this.Equal(map[string]string{"production": providerVersion}, provider.DeployedVersions, "provider resource of a consumer")

	consumers := repo.GetConsumersResourcesByProviderHashAndEnvironmentID(ctx, providerResource.ProviderHash, environmentID)
	this.Require().Len(consumers, 1, "consumers of a provider")
	this.Equal("files_web", consumers[0].ParticipantName, "consumers of a provider")
	this.Equal(map[string]model.Property{}, consumers[0].Properties, "consumers of a provider")
}

func (this *IntegrationSuite) TestContractRepository_ResourceWithoutProperties_ReturnedByEveryRead() {
	this.mustPostForUndescribedBody("/api/environments", `{"environment":"production"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_web"}`)
	this.publishUndescribedBodyContract("files_api", "1", undescribedBodyProviderContract)
	this.publishUndescribedBodyContract("files_web", "1", undescribedBodyConsumerContract)
	this.deployUndescribedBodyVersion("files_api", "1")
	this.deployUndescribedBodyVersion("files_web", "1")

	this.assertEveryReadReturnsTheFileResourceWithoutProperties("1")
}

func (this *IntegrationSuite) TestContractRepository_ResourceWhoseBodyBecameUndescribed_ReturnedWithoutProperties() {
	this.mustPostForUndescribedBody("/api/environments", `{"environment":"production"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_web"}`)
	this.publishUndescribedBodyContract("files_api", "1", describedBodyProviderContract)
	this.publishUndescribedBodyContract("files_api", "2", undescribedBodyProviderContract)
	this.publishUndescribedBodyContract("files_web", "1", describedBodyConsumerContract)
	this.publishUndescribedBodyContract("files_web", "2", undescribedBodyConsumerContract)
	this.deployUndescribedBodyVersion("files_api", "2")
	this.deployUndescribedBodyVersion("files_web", "2")

	this.assertEveryReadReturnsTheFileResourceWithoutProperties("2")

	first, found := repository.NewContractRepository(this.Pool).GetContractByNameAndVersion(context.Background(), "files_api", "1")
	this.Require().True(found)
	this.Equal(map[string]model.Property{
		"$":      {ID: onlyUndescribedBodyResource(first).Properties["$"].ID, Path: "$", Type: "object"},
		"$.name": {ID: onlyUndescribedBodyResource(first).Properties["$.name"].ID, Path: "$.name", Type: "string"},
	}, onlyUndescribedBodyResource(first).Properties)
}

func (this *IntegrationSuite) TestContractRepository_NewSnapshot_DoesNotAddTheResourceWithoutPropertiesAgain() {
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.publishUndescribedBodyContract("files_api", "1", undescribedBodyProviderContract)
	this.publishUndescribedBodyContract("files_api", "2", undescribedBodyProviderWithHealthContract)

	rows, err := this.Pool.Query(context.Background(), `
		SELECT rv.change_type
		FROM resource_versions rv
		JOIN resources r ON r.id = rv.resource_id
		WHERE r.endpoint = '/files/*'
		ORDER BY rv.contract_id`)
	this.Require().NoError(err)
	defer rows.Close()

	var changes []string
	for rows.Next() {
		var change string
		this.Require().NoError(rows.Scan(&change))
		changes = append(changes, change)
	}
	this.Require().NoError(rows.Err())

	this.Equal([]string{"added"}, changes)
	this.Equal(2, this.countRows("contracts"))
}
