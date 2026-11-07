package integration_test

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/jackc/pgx/v5"
)

const concurrentPublishBaseContract = `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
schemas:
  Order:
    type: object
    properties:
      id:
        type: string
`

type concurrentPublishContent struct {
	contract  string
	resources []string
	schemas   model.Schemas
}

var concurrentPublishWithCustomers = concurrentPublishContent{
	contract: `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
    /customers:
      get:
        responses:
          200: Customer
schemas:
  Order:
    type: object
    properties:
      id:
        type: string
  Customer:
    type: object
    properties:
      name:
        type: string
`,
	resources: []string{"get /customers 200", "get /orders 200"},
	schemas: model.Schemas{
		"Customer": {"": {Type: "object"}, "name": {Type: "string"}},
		"Order":    {"": {Type: "object"}, "id": {Type: "string"}},
	},
}

var concurrentPublishWithTotal = concurrentPublishContent{
	contract: `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
schemas:
  Order:
    type: object
    properties:
      id:
        type: string
      total:
        type: integer
`,
	resources: []string{"get /orders 200"},
	schemas: model.Schemas{
		"Order": {"": {Type: "object"}, "id": {Type: "string"}, "total": {Type: "integer"}},
	},
}

func (this *IntegrationSuite) mustPostForConcurrentPublish(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) concurrentPublishBody(version string, content concurrentPublishContent) string {
	return this.publishBody("orders_api", version, contractFragment{"api.yaml", content.contract})
}

func (this *IntegrationSuite) concurrentPublishConnect() *pgx.Conn {
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, this.Pool.Config().ConnConfig)
	this.Require().NoError(err)
	this.T().Cleanup(func() { _ = conn.Close(ctx) })

	return conn
}

func (this *IntegrationSuite) concurrentPublishAwaitLockWaiters(count int) {
	lockWaiters := this.concurrentPublishConnect()

	this.Require().Eventually(func() bool {
		var waiting int
		err := lockWaiters.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) concurrentPublish(bodies ...string) (statuses []int, responses []string) {
	ctx := context.Background()
	snapshotWritesOnHold, err := this.concurrentPublishConnect().Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = snapshotWritesOnHold.Rollback(ctx) }()
	_, err = snapshotWritesOnHold.Exec(ctx, `LOCK TABLE contracts IN SHARE MODE`)
	this.Require().NoError(err)

	statuses = make([]int, len(bodies))
	responses = make([]string, len(bodies))
	var published sync.WaitGroup
	for index, body := range bodies {
		published.Go(func() {
			statuses[index], responses[index] = this.post("/api/contracts", body)
		})
	}
	this.concurrentPublishAwaitLockWaiters(len(bodies))

	this.Require().NoError(snapshotWritesOnHold.Rollback(ctx))
	published.Wait()

	return statuses, responses
}

func (this *IntegrationSuite) assertConcurrentPublishRebuilds(version string, content concurrentPublishContent) {
	contract, found := repository.NewContractRepository(this.Pool).GetContractByNameAndVersion(context.Background(), "orders_api", version)
	this.Require().True(found, "version %s", version)

	var resources []string
	for _, resource := range contract.Resources {
		if !resource.Removed {
			resources = append(resources, resource.Method+" "+resource.Endpoint+" "+resource.ResponseStatusCode.String)
		}
	}
	slices.Sort(resources)

	this.Equal(content.resources, resources, "version %s", version)
	this.Equal(content.schemas, contract.Schemas, "version %s", version)
}

func (this *IntegrationSuite) TestPublish_ConcurrentNewVersionsOfDifferentContent_EachRebuildsToWhatWasPublished() {
	this.mustPostForConcurrentPublish("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForConcurrentPublish("/api/contracts", this.publishBody("orders_api", "1", contractFragment{"api.yaml", concurrentPublishBaseContract}))

	statuses, responses := this.concurrentPublish(
		this.concurrentPublishBody("2", concurrentPublishWithCustomers),
		this.concurrentPublishBody("3", concurrentPublishWithTotal),
	)

	this.Equal([]int{http.StatusOK, http.StatusOK}, statuses, responses)
	this.assertConcurrentPublishRebuilds("2", concurrentPublishWithCustomers)
	this.assertConcurrentPublishRebuilds("3", concurrentPublishWithTotal)
	this.Equal(3, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublish_ConcurrentFirstPublishes_EachRebuildsToWhatWasPublished() {
	this.mustPostForConcurrentPublish("/api/participants", `{"participant":"orders_api"}`)

	statuses, responses := this.concurrentPublish(
		this.concurrentPublishBody("1", concurrentPublishWithCustomers),
		this.concurrentPublishBody("2", concurrentPublishWithTotal),
	)

	this.Equal([]int{http.StatusOK, http.StatusOK}, statuses, responses)
	this.assertConcurrentPublishRebuilds("1", concurrentPublishWithCustomers)
	this.assertConcurrentPublishRebuilds("2", concurrentPublishWithTotal)
	this.Equal(2, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublish_ConcurrentSameVersionAndContent_BothSucceed() {
	this.mustPostForConcurrentPublish("/api/participants", `{"participant":"orders_api"}`)

	statuses, responses := this.concurrentPublish(
		this.concurrentPublishBody("1", concurrentPublishWithCustomers),
		this.concurrentPublishBody("1", concurrentPublishWithCustomers),
	)

	this.Equal([]int{http.StatusOK, http.StatusOK}, statuses, responses)
	this.assertConcurrentPublishRebuilds("1", concurrentPublishWithCustomers)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestPublish_ConcurrentSameVersionWithDifferentContent_OneSucceedsAndOneConflicts() {
	this.mustPostForConcurrentPublish("/api/participants", `{"participant":"orders_api"}`)
	contents := []concurrentPublishContent{concurrentPublishWithCustomers, concurrentPublishWithTotal}

	statuses, responses := this.concurrentPublish(
		this.concurrentPublishBody("1", contents[0]),
		this.concurrentPublishBody("1", contents[1]),
	)

	this.Require().ElementsMatch([]int{http.StatusOK, http.StatusConflict}, statuses, responses)
	published := slices.Index(statuses, http.StatusOK)
	this.JSONEq(`{"message":"contract version already exists with different content"}`, responses[1-published])
	this.assertConcurrentPublishRebuilds("1", contents[published])
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestPublish_ConcurrentSameContentUnderTwoVersions_BothSucceedAndOneAliasesTheOther() {
	this.mustPostForConcurrentPublish("/api/participants", `{"participant":"orders_api"}`)

	statuses, responses := this.concurrentPublish(
		this.concurrentPublishBody("1", concurrentPublishWithCustomers),
		this.concurrentPublishBody("2", concurrentPublishWithCustomers),
	)

	this.Equal([]int{http.StatusOK, http.StatusOK}, statuses, responses)
	this.assertConcurrentPublishRebuilds("1", concurrentPublishWithCustomers)
	this.assertConcurrentPublishRebuilds("2", concurrentPublishWithCustomers)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))
}
