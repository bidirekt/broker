package integration_test

import (
	"context"
	"net/http"
	"regexp"
	"strconv"

	"github.com/bidirekt/broker/internal/model"
)

const (
	participantDeploymentsReaderEmail    = "deployments-reader@x.com"
	participantDeploymentsReaderPassword = "deployments-reader-correct-horse"
	participantDeploymentsPath           = "/participants/petstore_api/deployments"
)

const participantDeploymentsAPIv1Contract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
consumes:
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
  petstore_web:
    rest:
      /config:
        get:
          responses:
            200: ~
`

const participantDeploymentsAPIv2Contract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
    /pets/*:
      get:
        responses:
          200: ~
consumes:
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
  petstore_web:
    rest:
      /config:
        get:
          responses:
            200: ~
`

const participantDeploymentsAPIConsumerContract = `provides:
  rest:
    /config:
      get:
        responses:
          200: ~
consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

var (
	participantDeploymentsRow     = regexp.MustCompile(`(?s)<tr>\s*<td data-l="Environment">(.*?)</tr>`)
	participantDeploymentsEnvCell = regexp.MustCompile(`<td data-l="Environment"><a class="badge env" href="/environments/\d+">([^<]+)</a>`)
	participantDeploymentsVerCell = regexp.MustCompile(`<a class="ver" href="[^"]+">([^<]+)</a>`)
)

func (this *IntegrationSuite) mustPostForParticipantDeployments(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForParticipantDeployments(participant, version, content string) {
	this.mustPostForParticipantDeployments("/api/contracts",
		this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForParticipantDeployments(participant, version, environment string) {
	this.mustPostForParticipantDeployments("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) createForParticipantDeployments(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForParticipantDeployments("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForParticipantDeployments("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) environmentIDForParticipantDeployments(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) historyForParticipantDeployments() {
	this.createForParticipantDeployments(
		[]string{"production", "staging"},
		[]string{"petstore_api", "petstore_web", "petstore_admin", "petstore_reviews"},
	)
	this.publishForParticipantDeployments("petstore_api", "1.3.0", participantDeploymentsAPIv1Contract)
	this.deployForParticipantDeployments("petstore_api", "1.3.0", "production")
	this.publishForParticipantDeployments("petstore_api", "1.4.0", participantDeploymentsAPIv2Contract)
	this.deployForParticipantDeployments("petstore_api", "1.4.0", "production")
	this.deployForParticipantDeployments("petstore_api", "1.3.0", "production")
	this.deployForParticipantDeployments("petstore_api", "1.4.0", "staging")
	this.publishForParticipantDeployments("petstore_web", "2.3.0", participantDeploymentsAPIConsumerContract)
	this.deployForParticipantDeployments("petstore_web", "2.3.0", "production")
	this.publishForParticipantDeployments("petstore_admin", "0.1.0", participantDeploymentsAPIConsumerContract)
	this.mustPostForParticipantDeployments("/api/can-i-deploy",
		`{"participant":"petstore_api","version":"1.4.0","environment":"production"}`)
}

func (this *IntegrationSuite) deploymentRowsForParticipantDeployments(body string) (environments, versions []string, rows []string) {
	for _, match := range participantDeploymentsRow.FindAllStringSubmatch(body, -1) {
		row := match[0]
		rows = append(rows, row)
		environments = append(environments, participantDeploymentsEnvCell.FindStringSubmatch(row)[1])
		versions = append(versions, participantDeploymentsVerCell.FindStringSubmatch(row)[1])
	}

	return environments, versions, rows
}

func (this *IntegrationSuite) TestListParticipantDeployments_ShowsTheHeaderAndTheHistoryNewestFirst() {
	this.historyForParticipantDeployments()
	productionID := this.environmentIDForParticipantDeployments("production")
	stagingID := this.environmentIDForParticipantDeployments("staging")

	status, header, body := this.getPage(participantDeploymentsPath, this.signInAsAdmin())

	this.Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))

	this.Contains(body, `<h1 class="mono">petstore_api</h1>`)
	this.Contains(body, `Latest <a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a>`)
	this.Regexp(`(?s)<a class="badge env" href="/environments/`+productionID+`">production <span class="v">1.3.0</span></a>.*<a class="badge env" href="/environments/`+stagingID+`">staging <span class="v">1.4.0</span></a>`, body)

	this.Contains(body, `<a href="/participants/petstore_api">Versions <span class="count">2</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_api/deployments" aria-current="page">Deployments <span class="count">4</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_api/checks">Checks <span class="count">1</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_api/relations">Relations <span class="count">2</span></a>`)

	environments, versions, rows := this.deploymentRowsForParticipantDeployments(body)
	this.Equal([]string{"staging", "production", "production", "production"}, environments)
	this.Equal([]string{"1.4.0", "1.3.0", "1.4.0", "1.3.0"}, versions)
	this.Contains(rows[0], `<a class="badge env" href="/environments/`+stagingID+`">staging</a>`)
	this.Contains(rows[1], `<a class="badge env" href="/environments/`+productionID+`">production</a>`)
	this.Contains(rows[1], `<span class="badge rb">`)
	for _, row := range []string{rows[0], rows[2], rows[3]} {
		this.NotContains(row, `<span class="badge rb">`)
	}
	this.Contains(body, "showing 1–4 of 4")
}

func (this *IntegrationSuite) TestListParticipantDeployments_FiltersByEnvironment() {
	this.historyForParticipantDeployments()
	productionID := this.environmentIDForParticipantDeployments("production")

	status, _, body := this.getPage(participantDeploymentsPath+"?environment="+productionID, this.signInAsAdmin())

	this.Equal(http.StatusOK, status)
	environments, versions, _ := this.deploymentRowsForParticipantDeployments(body)
	this.Equal([]string{"production", "production", "production"}, environments)
	this.Equal([]string{"1.3.0", "1.4.0", "1.3.0"}, versions)
	this.Contains(body, "showing 1–3 of 3")
	this.Contains(body, `<a class="chip" href="/participants/petstore_api/deployments?environment=`+productionID+`" aria-current="page">production</a>`)
	this.Contains(body, `<a class="chip" href="/participants/petstore_api/deployments">All</a>`)
	this.Contains(body, `Deployments <span class="count">4</span>`)
}

func (this *IntegrationSuite) TestListParticipantDeployments_InvalidOrUnknownEnvironmentIsNotFound() {
	this.historyForParticipantDeployments()
	cookie := this.signInAsAdmin()

	for _, query := range []string{"?environment=999", "?environment=0", "?environment=01", "?environment=%2B1", "?environment=", "?environment=abc"} {
		status, header, body := this.getPage(participantDeploymentsPath+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), query)
		this.Contains(body, "Not Found", query)
	}
}

func (this *IntegrationSuite) TestListParticipantDeployments_PaginatesAndKeepsTheEnvironmentFilter() {
	this.createForParticipantDeployments([]string{"production", "staging"}, []string{"petstore_api"})
	this.publishForParticipantDeployments("petstore_api", "1.3.0", participantDeploymentsAPIv1Contract)
	productionID := this.environmentIDForParticipantDeployments("production")
	stagingID := this.environmentIDForParticipantDeployments("staging")
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO deployments (participant_id, version, environment_id, rollback, deployed_at)
		SELECT p.id, '1.3.0', $1::bigint, false, now() - make_interval(mins => n)
		FROM participants p, generate_series(1, 26) n
		WHERE p.name = 'petstore_api'`, productionID)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `
		INSERT INTO deployments (participant_id, version, environment_id, rollback, deployed_at)
		SELECT id, '1.3.0', $1::bigint, false, now() FROM participants WHERE name = 'petstore_api'`, stagingID)
	this.Require().NoError(err)
	cookie := this.signInAsAdmin()
	filtered := participantDeploymentsPath + "?environment=" + productionID

	status, _, body := this.getPage(filtered, cookie)
	this.Equal(http.StatusOK, status)
	_, versions, _ := this.deploymentRowsForParticipantDeployments(body)
	this.Len(versions, 25)
	this.Contains(body, "showing 1–25 of 26")
	this.Contains(body, `href="/participants/petstore_api/deployments?environment=`+productionID+`&amp;page=2" rel="next"`)

	status, _, body = this.getPage(filtered+"&page=2", cookie)
	this.Equal(http.StatusOK, status)
	_, versions, _ = this.deploymentRowsForParticipantDeployments(body)
	this.Len(versions, 1)
	this.Contains(body, "showing 26–26 of 26")
	this.Contains(body, `href="/participants/petstore_api/deployments?environment=`+productionID+`&amp;page=1" rel="prev"`)

	status, _, body = this.getPage(participantDeploymentsPath+"?page=2", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, "showing 26–27 of 27")

	for _, path := range []string{filtered + "&page=3", participantDeploymentsPath + "?page=0", participantDeploymentsPath + "?page=01"} {
		status, _, _ = this.getPage(path, cookie)
		this.Equal(http.StatusNotFound, status, path)
	}
}

func (this *IntegrationSuite) TestListParticipantDeployments_NeverPublishedShowsThePublishCommand() {
	this.createForParticipantDeployments([]string{"production"}, []string{"petstore_api"})

	status, _, body := this.getPage(participantDeploymentsPath, this.signInAsAdmin())

	this.Equal(http.StatusOK, status)
	this.Contains(body, `<p class="sub">never published</p>`)
	this.Contains(body, `Versions <span class="count">0</span>`)
	this.Contains(body, `Deployments <span class="count">0</span>`)
	this.Contains(body, `Checks <span class="count">0</span>`)
	this.Contains(body, `Relations <span class="count">0</span>`)
	this.Contains(body, `<code class="cmd">bidirekt publish</code>`)
	this.NotContains(body, `<table`)

	status, _, body = this.getPage(participantDeploymentsPath+"?page=2", this.signInAsAdmin())
	this.Equal(http.StatusNotFound, status, body)
}

func (this *IntegrationSuite) TestListParticipantDeployments_PublishedButNeverDeployedShowsNoDeployments() {
	this.createForParticipantDeployments([]string{"production"}, []string{"petstore_api"})
	this.publishForParticipantDeployments("petstore_api", "1.3.0", participantDeploymentsAPIv1Contract)
	productionID := this.environmentIDForParticipantDeployments("production")
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage(participantDeploymentsPath, cookie)

	this.Equal(http.StatusOK, status)
	this.Contains(body, `<span class="badge quiet">not deployed</span>`)
	this.Contains(body, "No deployments yet")
	this.Contains(body, `<code class="cmd">bidirekt record-deployment</code>`)

	status, _, body = this.getPage(participantDeploymentsPath+"?environment="+productionID, cookie)

	this.Equal(http.StatusOK, status)
	this.Contains(body, `No deployments in <a class="name" href="/environments/`+productionID+`">production</a>`)
	this.Contains(body, `<code class="cmd">bidirekt record-deployment</code>`)
}

func (this *IntegrationSuite) TestListParticipantDeployments_UnknownParticipantIsNotFound() {
	status, header, body := this.getPage("/participants/ghost/deployments", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "Not Found")
}

func (this *IntegrationSuite) TestListParticipantDeployments_SignedOutIsRedirectedToSignIn() {
	this.historyForParticipantDeployments()

	status, header, _ := this.getPage(participantDeploymentsPath, "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestListParticipantDeployments_ReadUserSeesTheHistoryWithoutWriteActions() {
	this.historyForParticipantDeployments()
	this.createActiveUser(participantDeploymentsReaderEmail, model.PermissionRead, participantDeploymentsReaderPassword)

	status, _, body := this.getPage(participantDeploymentsPath,
		this.signInAsUser(participantDeploymentsReaderEmail, participantDeploymentsReaderPassword))

	this.Equal(http.StatusOK, status)
	this.Contains(body, "showing 1–4 of 4")
	this.NotContains(body, `method="post" action="/participants`)
}

func (this *IntegrationSuite) TestListParticipantDeployments_LinksArePathEscaped() {
	this.createForParticipantDeployments([]string{"production"}, []string{"orders_web"})
	this.publishForParticipantDeployments("orders_web", "1.0.0+build", participantDeploymentsAPIv1Contract)
	this.deployForParticipantDeployments("orders_web", "1.0.0+build", "production")
	productionID := this.environmentIDForParticipantDeployments("production")

	status, _, body := this.getPage("/participants/orders_web/deployments", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<a class="chip" href="/participants/orders_web/deployments" aria-current="page">All</a>`)
	this.Contains(body, `<a class="chip" href="/participants/orders_web/deployments?environment=`+productionID+`">production</a>`)
	this.Contains(body, `<td data-l="Version"><div class="badges"><a class="ver" href="/participants/orders_web/versions/1.0.0&#43;build">1.0.0&#43;build</a></div></td>`)
}
