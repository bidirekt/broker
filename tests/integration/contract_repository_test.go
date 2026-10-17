package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
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

func (this *IntegrationSuite) TestContractRepository_NullableFieldRoundTripsThroughEveryRead() {
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
	this.Equal(model.Field{Type: "string", Nullable: true}, byVersion.Schemas["Account"]["avatar"], "contract by version")
	this.Equal(model.Field{Type: "string"}, byVersion.Schemas["Account"]["nickname"], "contract by version")
	this.Equal(byVersion.Schemas, providerResource.Schemas, "contract by version")

	latest, found := repo.GetLatestContractByName(ctx, "accounts_api")
	this.Require().True(found)
	this.True(latest.Schemas["Account"]["avatar"].Nullable, "latest contract")
	this.True(onlyNullableRoundTripResource(latest).Schemas["Account"]["avatar"].Nullable, "latest contract")

	provider, err := repo.GetProviderResourceByConsumerResource(ctx, providerResource.ProviderHash, environmentID)
	this.Require().NoError(err)
	this.True(provider.Schemas["Account"]["avatar"].Nullable, "provider resource of a consumer")
	this.False(provider.Schemas["Account"]["nickname"].Nullable, "provider resource of a consumer")

	consumers := repo.GetConsumersResourcesByProviderHashAndEnvironmentID(ctx, providerResource.ProviderHash, environmentID)
	this.Require().Len(consumers, 1)
	this.True(consumers[0].Schemas["Account"]["avatar"].Nullable, "consumers of a provider")
}

func (this *IntegrationSuite) TestContractRepository_OnlyNullabilityChanged_WritesAModifiedFieldVersion() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForNullableRoundTrip("/api/participants", `{"participant":"accounts_api"}`)
	this.publishNullableAccountsProvider("1", false)
	this.publishNullableAccountsProvider("2", true)

	rows, err := this.Pool.Query(ctx, `
		SELECT fv.change_type, fv.nullable
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		WHERE f.path = 'avatar'
		ORDER BY fv.contract_id`)
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
	this.False(first.Schemas["Account"]["avatar"].Nullable)

	second, found := repo.GetContractByNameAndVersion(ctx, "accounts_api", "2")
	this.Require().True(found)
	this.True(second.Schemas["Account"]["avatar"].Nullable)
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

func (this *IntegrationSuite) assertEveryReadReturnsTheFileResourceWithAnUndescribedBody(providerVersion string) {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)
	environmentID := this.undescribedBodyProductionID()
	undescribed := []model.ResourceBody{{Source: "api.yaml"}}

	byVersion, found := repo.GetContractByNameAndVersion(ctx, "files_api", providerVersion)
	this.Require().True(found, "contract by version")
	this.Require().Len(byVersion.Resources, 1, "contract by version")
	providerResource := onlyUndescribedBodyResource(byVersion)
	this.Equal("/files/*", providerResource.Endpoint, "contract by version")
	this.Equal(undescribed, providerResource.Bodies, "contract by version")
	this.Equal(model.Schemas{}, byVersion.Schemas, "contract by version")

	latest, found := repo.GetLatestContractByName(ctx, "files_api")
	this.Require().True(found, "latest contract")
	this.Require().Len(latest.Resources, 1, "latest contract")
	this.Equal(undescribed, onlyUndescribedBodyResource(latest).Bodies, "latest contract")

	provider, err := repo.GetProviderResourceByConsumerResource(ctx, providerResource.ProviderHash, environmentID)
	this.Require().NoError(err, "provider resource of a consumer")
	this.Equal("/files/*", provider.Endpoint, "provider resource of a consumer")
	this.Equal(undescribed, provider.Bodies, "provider resource of a consumer")
	this.Equal(map[string]string{"production": providerVersion}, provider.DeployedVersions, "provider resource of a consumer")

	consumers := repo.GetConsumersResourcesByProviderHashAndEnvironmentID(ctx, providerResource.ProviderHash, environmentID)
	this.Require().Len(consumers, 1, "consumers of a provider")
	this.Equal("files_web", consumers[0].ParticipantName, "consumers of a provider")
	this.Equal(undescribed, consumers[0].Bodies, "consumers of a provider")
}

func (this *IntegrationSuite) TestContractRepository_UndescribedBody_ReturnedByEveryRead() {
	this.mustPostForUndescribedBody("/api/environments", `{"environment":"production"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_web"}`)
	this.publishUndescribedBodyContract("files_api", "1", undescribedBodyProviderContract)
	this.publishUndescribedBodyContract("files_web", "1", undescribedBodyConsumerContract)
	this.deployUndescribedBodyVersion("files_api", "1")
	this.deployUndescribedBodyVersion("files_web", "1")

	this.assertEveryReadReturnsTheFileResourceWithAnUndescribedBody("1")
	this.Equal(2, this.countRows("resource_bodies"))
}

func (this *IntegrationSuite) TestContractRepository_ResourceWhoseBodyBecameUndescribed_KeepsEachVersionsBody() {
	this.mustPostForUndescribedBody("/api/environments", `{"environment":"production"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_web"}`)
	this.publishUndescribedBodyContract("files_api", "1", describedBodyProviderContract)
	this.publishUndescribedBodyContract("files_api", "2", undescribedBodyProviderContract)
	this.publishUndescribedBodyContract("files_web", "1", describedBodyConsumerContract)
	this.publishUndescribedBodyContract("files_web", "2", undescribedBodyConsumerContract)
	this.deployUndescribedBodyVersion("files_api", "2")
	this.deployUndescribedBodyVersion("files_web", "2")

	this.assertEveryReadReturnsTheFileResourceWithAnUndescribedBody("2")

	first, found := repository.NewContractRepository(this.Pool).GetContractByNameAndVersion(context.Background(), "files_api", "1")
	this.Require().True(found)
	this.Equal([]model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, onlyUndescribedBodyResource(first).Bodies)
	this.Equal(model.Schemas{"Pet": {"": {Type: "object"}, "name": {Type: "string"}}}, first.Schemas)
}

func (this *IntegrationSuite) TestContractRepository_NewSnapshot_DoesNotAddTheUndescribedResourceAgain() {
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

func (this *IntegrationSuite) TestContractRepository_RemovedResource_ComesBackRemovedWithoutBodyInItsVersion() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForUndescribedBody("/api/participants", `{"participant":"files_api"}`)
	this.publishUndescribedBodyContract("files_api", "1", undescribedBodyProviderWithHealthContract)
	this.publishUndescribedBodyContract("files_api", "2", undescribedBodyProviderContract)

	first, found := repo.GetContractByNameAndVersion(ctx, "files_api", "1")
	this.Require().True(found)
	this.Len(first.Resources, 2)
	this.Contains(first.Schemas, "Health")

	second, found := repo.GetContractByNameAndVersion(ctx, "files_api", "2")
	this.Require().True(found)
	this.Require().Len(second.Resources, 2)
	for _, resource := range second.Resources {
		if resource.Endpoint == "/health" {
			this.True(resource.Removed)
			this.Empty(resource.Bodies)
			continue
		}
		this.False(resource.Removed)
		this.Equal([]model.ResourceBody{{Source: "api.yaml"}}, resource.Bodies)
	}
	this.Equal(model.Schemas{}, second.Schemas)

	latest, found := repo.GetLatestContractByName(ctx, "files_api")
	this.Require().True(found)
	this.Len(latest.Resources, 1)
}

const schemaRowsOrdersContract = `provides:
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
      items:
        type: array
        items:
          type: object
          properties:
            sku:
              type: string
      tags:
        type: map
        values:
          type: string
  Address:
    type: object
    properties:
      city:
        type: string
  Unused:
    type: string
`

func (this *IntegrationSuite) mustPostForSchemaRows(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishSchemaRowsOrders(version, content string) {
	this.mustPostForSchemaRows("/api/contracts", this.publishBody("orders_api", version, contractFragment{"api.yaml", content}))
}

func (this *IntegrationSuite) schemaRowsStrings(query string, args ...any) []string {
	rows, err := this.Pool.Query(context.Background(), query, args...)
	this.Require().NoError(err)
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		this.Require().NoError(rows.Scan(&value))
		values = append(values, value)
	}
	this.Require().NoError(rows.Err())

	return values
}

func (this *IntegrationSuite) schemaRowsFieldEvents(contractID int) []string {
	return this.schemaRowsStrings(`
		SELECT s.name || '.' || f.path || ':' || fv.change_type
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		JOIN schemas s ON s.id = f.schema_id
		WHERE fv.contract_id = $1
		ORDER BY s.name || '.' || f.path || ':' || fv.change_type COLLATE "C"`, contractID)
}

func (this *IntegrationSuite) schemaRowsSchemaEvents(contractID int) []string {
	return this.schemaRowsStrings(`
		SELECT s.name || ':' || sv.change_type
		FROM schema_versions sv
		JOIN schemas s ON s.id = sv.schema_id
		WHERE sv.contract_id = $1
		ORDER BY s.name || ':' || sv.change_type COLLATE "C"`, contractID)
}

func (this *IntegrationSuite) TestContractRepository_FirstPublish_WritesTheReachableSchemasFieldsAndBodies() {
	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsOrdersContract)

	this.Equal([]string{"Address:added", "Order:added"}, this.schemaRowsSchemaEvents(1))
	this.Equal([]string{
		"Address.:added",
		"Address.city:added",
		"Order.:added",
		"Order.billing:added",
		"Order.items:added",
		"Order.items[].sku:added",
		"Order.items[]:added",
		"Order.tags:added",
		"Order.tags{}:added",
	}, this.schemaRowsFieldEvents(1))
	this.Equal([]string{"billing:Address"}, this.schemaRowsStrings(`
		SELECT f.path || ':' || fv.ref
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		WHERE fv.ref IS NOT NULL AND fv.type IS NULL`))
	this.Equal([]string{"api.yaml:Order", "api.yaml:~"}, this.schemaRowsStrings(`
		SELECT rb.source || ':' || COALESCE(s.name, '~')
		FROM resource_bodies rb
		LEFT JOIN schemas s ON s.id = rb.schema_id
		ORDER BY rb.source || ':' || COALESCE(s.name, '~') COLLATE "C"`))
}

func (this *IntegrationSuite) TestContractRepository_SecondPublish_WritesOnlyTheChangeEventsAndEveryBody() {
	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsOrdersContract)

	changed := strings.Replace(schemaRowsOrdersContract,
		"      city:\n        type: string\n",
		"      city:\n        type: string\n        nullable: true\n      zip:\n        type: string\n", 1)
	changed = strings.Replace(changed, "            sku:\n              type: string\n", "", 1)
	this.publishSchemaRowsOrders("2", changed)

	this.Empty(this.schemaRowsSchemaEvents(2))
	this.Equal([]string{
		"Address.city:modified",
		"Address.zip:added",
		"Order.items[].sku:removed",
	}, this.schemaRowsFieldEvents(2))
	this.Equal(0, len(this.schemaRowsStrings(`SELECT change_type FROM resource_versions WHERE contract_id = 2`)))
	this.Equal(2, len(this.schemaRowsStrings(`SELECT source FROM resource_bodies WHERE contract_id = 2`)))
}

func (this *IntegrationSuite) TestContractRepository_RenamedSchema_IsRemovedAndAdded() {
	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsOrdersContract)
	this.publishSchemaRowsOrders("2", strings.ReplaceAll(schemaRowsOrdersContract, "Address", "PostalAddress"))

	this.Equal([]string{"Address:removed", "PostalAddress:added"}, this.schemaRowsSchemaEvents(2))
	this.Equal([]string{
		"Address.:removed",
		"Address.city:removed",
		"Order.billing:modified",
		"PostalAddress.:added",
		"PostalAddress.city:added",
	}, this.schemaRowsFieldEvents(2))
}

func (this *IntegrationSuite) TestContractRepository_SchemaAddedBack_DoesNotBringBackItsOldFields() {
	ctx := context.Background()

	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsOrdersContract)
	this.publishSchemaRowsOrders("2", strings.ReplaceAll(schemaRowsOrdersContract, "Address", "PostalAddress"))
	this.publishSchemaRowsOrders("3", strings.Replace(schemaRowsOrdersContract,
		"      city:\n        type: string\n", "      line:\n        type: string\n", 1))

	third, found := repository.NewContractRepository(this.Pool).GetContractByNameAndVersion(ctx, "orders_api", "3")
	this.Require().True(found)
	this.Equal(model.Schema{"": {Type: "object"}, "line": {Type: "string"}}, third.Schemas["Address"])
	this.NotContains(third.Schemas, "PostalAddress")
	this.Equal(1, len(this.schemaRowsStrings(`SELECT name FROM schemas WHERE name = 'Address'`)))
}

func (this *IntegrationSuite) TestContractRepository_ReadsByVersion_ReturnEachVersionsFields() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsOrdersContract)
	this.publishSchemaRowsOrders("2", strings.Replace(schemaRowsOrdersContract,
		"      city:\n        type: string\n", "      city:\n        type: integer\n        optional: true\n", 1))

	first, found := repo.GetContractByNameAndVersion(ctx, "orders_api", "1")
	this.Require().True(found)
	second, found := repo.GetContractByNameAndVersion(ctx, "orders_api", "2")
	this.Require().True(found)

	this.Equal(model.Schemas{
		"Order": {
			"":            {Type: "object"},
			"billing":     {Ref: "Address"},
			"items":       {Type: "array"},
			"items[]":     {Type: "object"},
			"items[].sku": {Type: "string"},
			"tags":        {Type: "map"},
			"tags{}":      {Type: "string"},
		},
		"Address": {"": {Type: "object"}, "city": {Type: "string"}},
	}, first.Schemas)
	this.Equal(model.Field{Type: "integer", Optional: true}, second.Schemas["Address"]["city"])

	for _, resource := range second.Resources {
		this.Equal(reflect.ValueOf(second.Schemas).Pointer(), reflect.ValueOf(resource.Schemas).Pointer())
	}
}

func schemaRowsExponentialContract(levels int) string {
	var contract strings.Builder
	contract.WriteString("provides:\n  rest:\n    /deep:\n      get:\n        responses:\n          200: S0\nschemas:\n")
	for level := 0; level < levels; level++ {
		fmt.Fprintf(&contract, "  S%d:\n    type: object\n    properties:\n      left:\n        ref: S%d\n      right:\n        ref: S%d\n",
			level, level+1, level+1)
	}
	fmt.Fprintf(&contract, "  S%d:\n    type: string\n", levels)

	return contract.String()
}

func (this *IntegrationSuite) TestContractRepository_ExponentialSchema_WritesOnlyTheDeclaredFields() {
	this.mustPostForSchemaRows("/api/participants", `{"participant":"orders_api"}`)
	this.publishSchemaRowsOrders("1", schemaRowsExponentialContract(22))

	this.Equal(23, this.countRows("schemas"))
	this.Equal(67, this.countRows("fields"))
	this.Equal(67, this.countRows("field_versions"))

	contract, found := repository.NewContractRepository(this.Pool).GetContractByNameAndVersion(context.Background(), "orders_api", "1")
	this.Require().True(found)
	this.Len(contract.Schemas, 23)
	this.Equal(model.Schema{"": {Type: "object"}, "left": {Ref: "S1"}, "right": {Ref: "S1"}}, contract.Schemas["S0"])
}

const schemaRowsAccountsProviderContract = `provides:
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
`

const schemaRowsAccountsWebContract = `consumes:
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
`

const schemaRowsAccountsWidgetContract = `consumes:
  accounts_api:
    rest:
      /accounts:
        get:
          responses:
            200: Profile
schemas:
  Profile:
    type: object
`

func (this *IntegrationSuite) TestContractRepository_CounterpartReads_CarryTheCatalogAtTheDeployedVersion() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool)

	this.mustPostForSchemaRows("/api/environments", `{"environment":"production"}`)
	this.mustPostForSchemaRows("/api/participants", `{"participant":"accounts_api"}`)
	this.mustPostForSchemaRows("/api/participants", `{"participant":"accounts_web"}`)
	this.mustPostForSchemaRows("/api/contracts", this.publishBody("accounts_api", "1",
		contractFragment{"api.yaml", fmt.Sprintf(schemaRowsAccountsProviderContract, false)}))
	this.mustPostForSchemaRows("/api/contracts", this.publishBody("accounts_api", "2",
		contractFragment{"api.yaml", fmt.Sprintf(schemaRowsAccountsProviderContract, true)}))
	this.mustPostForSchemaRows("/api/contracts", this.publishBody("accounts_web", "1",
		contractFragment{"widget.yaml", schemaRowsAccountsWidgetContract},
		contractFragment{"web.yaml", schemaRowsAccountsWebContract}))
	this.mustPostForSchemaRows("/api/deployments", `{"participant":"accounts_api","version":"1","environment":"production"}`)
	this.mustPostForSchemaRows("/api/deployments", `{"participant":"accounts_web","version":"1","environment":"production"}`)

	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(ctx, `SELECT id FROM environments WHERE name = 'production'`).Scan(&environmentID))

	web, found := repo.GetContractByNameAndVersion(ctx, "accounts_web", "1")
	this.Require().True(found)
	this.Require().Len(web.Resources, 1)
	var consumer model.PersistedResource
	for _, resource := range web.Resources {
		consumer = resource
	}
	this.Equal([]model.ResourceBody{
		{Source: "web.yaml", SchemaName: null.StringFrom("Account")},
		{Source: "widget.yaml", SchemaName: null.StringFrom("Profile")},
	}, consumer.Bodies)

	provider, err := repo.GetProviderResourceByConsumerResource(ctx, consumer.ProviderHash, environmentID)
	this.Require().NoError(err)
	this.Equal(model.Field{Type: "string"}, provider.Schemas["Account"]["avatar"])
	this.Equal([]model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Account")}}, provider.Bodies)

	consumers := repo.GetConsumersResourcesByProviderHashAndEnvironmentID(ctx, consumer.ProviderHash, environmentID)
	this.Require().Len(consumers, 1)
	this.Equal(model.Schemas{
		"Account": {"": {Type: "object"}, "avatar": {Type: "string"}},
		"Profile": {"": {Type: "object"}},
	}, consumers[0].Schemas)
	this.Equal(null.StringFrom("1"), consumers[0].ParticipantVersion)

	latest, found := repo.GetLatestContractByName(ctx, "accounts_api")
	this.Require().True(found)
	this.True(latest.Schemas["Account"]["avatar"].Nullable)
	counterparts := repo.LoadCounterparts(ctx, latest, environmentID)
	this.Require().Len(counterparts.Consumers[consumer.ProviderHash], 1)
	this.Equal(consumers[0].Schemas, counterparts.Consumers[consumer.ProviderHash][0].Schemas)
}
