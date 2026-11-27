package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const reservedNameGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const reservedNameSubgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
}
`

func (this *IntegrationSuite) reservedNameSubgraphBody() string {
	return this.publishBody("petstore_api", "1",
		contractFragment{"graph.yaml", reservedNameGraphYAML},
		contractFragment{"schema.graphqls", reservedNameSubgraphSDL},
	)
}

func (this *IntegrationSuite) publishReservedNameSubgraph() {
	status, body := this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status, body)

	status, body = this.post("/api/contracts", this.reservedNameSubgraphBody())
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) reservedNameParticipants(name string) int {
	var count int
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM participants WHERE name = $1`, name).Scan(&count))

	return count
}

func (this *IntegrationSuite) TestCreateParticipant_Api_AGraphName_IsAConflictWithTheMessage() {
	this.publishReservedNameSubgraph()

	status, body := this.post("/api/participants", `{"participant":"petstore_graph"}`)

	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"\"petstore_graph\" is a graph name; pick another participant name"}`, body)
	this.Equal(0, this.reservedNameParticipants("petstore_graph"))
}

func (this *IntegrationSuite) TestCreateParticipant_Api_AFreeName_IsStillCreated() {
	this.publishReservedNameSubgraph()

	status, body := this.post("/api/participants", `{"participant":"petstore_web"}`)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant created"}`, body)
	this.Equal(1, this.reservedNameParticipants("petstore_web"))
}

func (this *IntegrationSuite) TestCreateParticipant_Ui_AGraphName_IsAConflictWithTheMessage() {
	this.publishReservedNameSubgraph()
	cookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)
	_, _, form := this.getPage("/participants", cookie)

	status, _, body := this.postForm("/participants/new-participant", cookie, this.csrfTokenFrom(form), url.Values{"name": {"petstore_graph"}})

	this.Equal(http.StatusConflict, status)
	this.Contains(body, `<span>&#34;petstore_graph&#34; is a graph name; pick another participant name</span>`)
	this.Equal(0, this.reservedNameParticipants("petstore_graph"))
}

func (this *IntegrationSuite) TestCreateParticipant_AndAPublishOfTheSameGraph_OnlyOneIsAccepted() {
	ctx := context.Background()
	status, body := this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status, body)

	nameHolder, err := pgx.ConnectConfig(ctx, this.Pool.Config().ConnConfig)
	this.Require().NoError(err)
	defer func() { _ = nameHolder.Close(ctx) }()
	nameOnHold, err := nameHolder.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = nameOnHold.Rollback(ctx) }()
	_, err = nameOnHold.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('participant_name:petstore_graph'))`)
	this.Require().NoError(err)

	var publishStatus, createStatus int
	var publishBody, createBody string
	var requests sync.WaitGroup
	requests.Go(func() { publishStatus, publishBody = this.post("/api/contracts", this.reservedNameSubgraphBody()) })
	requests.Go(func() { createStatus, createBody = this.post("/api/participants", `{"participant":"petstore_graph"}`) })

	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event = 'advisory'`,
		).Scan(&waiting)
		return err == nil && waiting == 2
	}, 10*time.Second, 10*time.Millisecond)

	this.Require().NoError(nameOnHold.Rollback(ctx))
	requests.Wait()

	accepted := 0
	if publishStatus == http.StatusOK {
		accepted++
	}
	if createStatus == http.StatusOK {
		accepted++
	}

	this.Equal(1, accepted, "publish: %d %s; create-participant: %d %s", publishStatus, publishBody, createStatus, createBody)
	this.Equal(1, this.reservedNameParticipants("petstore_graph")+this.countRows("graphql_graph_members"))
}
