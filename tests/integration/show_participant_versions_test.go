package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	showParticipantVersionsReaderEmail    = "reader@x.com"
	showParticipantVersionsReaderPassword = "reader-correct-horse-battery"
)

const showParticipantVersionsAPIContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

const showParticipantVersionsWebContract = `provides:
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
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
  ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
`

const showParticipantVersionsWebConsumerContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
consumes:
  petstore_web:
    rest:
      /config:
        get:
          responses:
            200: ~
`

func (this *IntegrationSuite) mustPostForShowParticipantVersions(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForShowParticipantVersions(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowParticipantVersions("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowParticipantVersions("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) publishForShowParticipantVersions(participant, version, content string) {
	this.mustPostForShowParticipantVersions("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForShowParticipantVersions(participant, version, environment string) {
	this.mustPostForShowParticipantVersions("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) seedForShowParticipantVersions() {
	this.createForShowParticipantVersions(
		[]string{"staging", "production"},
		[]string{"petstore_api", "petstore_web", "petstore_reviews", "petstore_billing", "petstore_admin"},
	)
	this.publishForShowParticipantVersions("petstore_api", "1.0.0", showParticipantVersionsAPIContract)
	this.publishForShowParticipantVersions("petstore_web", "2.1.0", showParticipantVersionsWebContract)
	this.publishForShowParticipantVersions("petstore_web", "2.2.0", showParticipantVersionsWebContract)
	this.publishForShowParticipantVersions("petstore_web", "2.3.0", showParticipantVersionsWebContract)
	this.deployForShowParticipantVersions("petstore_web", "2.1.0", "production")
	this.deployForShowParticipantVersions("petstore_web", "2.2.0", "production")
	this.deployForShowParticipantVersions("petstore_web", "2.3.0", "staging")
	this.publishForShowParticipantVersions("petstore_reviews", "1.0.0", showParticipantVersionsWebConsumerContract)
	this.deployForShowParticipantVersions("petstore_reviews", "1.0.0", "staging")
	this.publishForShowParticipantVersions("petstore_billing", "1.0.0", showParticipantVersionsWebConsumerContract)
	this.mustPostForShowParticipantVersions("/api/can-i-deploy", `{"participant":"petstore_web","version":"2.3.0","environment":"production"}`)
}

func (this *IntegrationSuite) environmentPathForShowParticipantVersions(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) TestShowParticipantVersions_ShowsTheHeaderWithLatestVersionDeployedNowAndTabCounts() {
	this.seedForShowParticipantVersions()
	production := this.environmentPathForShowParticipantVersions("production")
	staging := this.environmentPathForShowParticipantVersions("staging")

	status, header, body := this.getPage("/participants/petstore_web", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<nav class="crumbs" aria-label="Breadcrumbs"><a href="/participants">participants</a><span aria-hidden="true">/</span><span aria-current="page">petstore_web</span></nav>`)
	this.Contains(body, `<h1 class="mono">petstore_web</h1>`)
	this.Contains(body, `<span>Latest <a class="ver" href="/participants/petstore_web/versions/2.3.0">2.3.0</a></span>`+"\n      "+`<span aria-hidden="true">·</span>`+"\n      "+`<a class="badge env" href="`+production+`">production <span class="v">2.2.0</span></a><a class="badge env" href="`+staging+`">staging <span class="v">2.3.0</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web" aria-current="page">Versions <span class="count">3</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/deployments">Deployments <span class="count">3</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/checks">Checks <span class="count">1</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/relations">Relations <span class="count">3</span></a>`)
}

func (this *IntegrationSuite) TestShowParticipantVersions_RelationsCountsOnlyDeployedConsumersOfTheirDeployedVersion() {
	this.createForShowParticipantVersions([]string{"production"}, []string{"petstore_web", "petstore_reviews"})
	this.publishForShowParticipantVersions("petstore_web", "1.0.0", showParticipantVersionsAPIContract)
	this.publishForShowParticipantVersions("petstore_reviews", "1.0.0", showParticipantVersionsWebConsumerContract)
	this.publishForShowParticipantVersions("petstore_reviews", "2.0.0", showParticipantVersionsAPIContract)
	this.deployForShowParticipantVersions("petstore_reviews", "2.0.0", "production")

	_, _, body := this.getPage("/participants/petstore_web", this.signInAsAdmin())

	this.Contains(body, `Relations <span class="count">0</span>`)

	this.deployForShowParticipantVersions("petstore_reviews", "1.0.0", "production")

	_, _, body = this.getPage("/participants/petstore_web", this.signInAsAdmin())

	this.Contains(body, `Relations <span class="count">1</span>`)
}

func (this *IntegrationSuite) TestShowParticipantVersions_ListsVersionsNewestFirstWithPublishTimeAndWhereDeployedNow() {
	this.seedForShowParticipantVersions()
	production := this.environmentPathForShowParticipantVersions("production")
	staging := this.environmentPathForShowParticipantVersions("staging")

	_, _, body := this.getPage("/participants/petstore_web", this.signInAsAdmin())

	rows := []string{
		`<td data-l="Version"><a class="ver" href="/participants/petstore_web/versions/2.3.0">2.3.0</a></td>`,
		`<td data-l="Version"><a class="ver" href="/participants/petstore_web/versions/2.2.0">2.2.0</a></td>`,
		`<td data-l="Version"><a class="ver" href="/participants/petstore_web/versions/2.1.0">2.1.0</a></td>`,
	}
	previous := -1
	for _, row := range rows {
		at := strings.Index(body, row)
		this.Require().Greater(at, previous, row)
		previous = at
	}
	this.Equal(3, strings.Count(body, `<td data-l="Published"><time class="time" datetime="`))
	this.Contains(body, `just now</time></td>`)
	this.Contains(body, `<td data-l="Deployed now"><span class="badges"><a class="badge env" href="`+staging+`">staging</a></span></td>`)
	this.Contains(body, `<td data-l="Deployed now"><span class="badges"><a class="badge env" href="`+production+`">production</a></span></td>`)
	this.Contains(body, `<td data-l="Deployed now"><span class="muted">not deployed</span></td>`)
	this.Contains(body, `showing 1–3 of 3`)
}

func (this *IntegrationSuite) TestShowParticipantVersions_VersionLinksArePathEscapedAndDotVersionsHaveNoURL() {
	this.createForShowParticipantVersions(nil, []string{"petstore_api"})
	for _, version := range []string{".", "..", "1.0/beta rc"} {
		this.publishForShowParticipantVersions("petstore_api", version, showParticipantVersionsAPIContract)
	}

	_, _, body := this.getPage("/participants/petstore_api", this.signInAsAdmin())

	this.Contains(body, `<a class="ver" href="/participants/petstore_api/versions/1.0%2Fbeta%20rc">1.0/beta rc</a>`)
	this.Contains(body, `<td data-l="Version"><span class="ver">..</span></td>`)
	this.Contains(body, `<td data-l="Version"><span class="ver">.</span></td>`)
	this.Equal(2, strings.Count(body, `/participants/petstore_api/versions/`))
}

func (this *IntegrationSuite) TestShowParticipantVersions_PublishedButNotDeployedSaysNotDeployed() {
	this.createForShowParticipantVersions([]string{"production"}, []string{"petstore_api"})
	this.publishForShowParticipantVersions("petstore_api", "1.0.0", showParticipantVersionsAPIContract)

	_, _, body := this.getPage("/participants/petstore_api", this.signInAsAdmin())

	this.Contains(body, `<span>Latest <a class="ver" href="/participants/petstore_api/versions/1.0.0">1.0.0</a></span>`+"\n      "+`<span aria-hidden="true">·</span>`+"\n      "+`<span class="badge quiet">not deployed</span>`)
}

func (this *IntegrationSuite) TestShowParticipantVersions_NeverPublishedShowsTheEmptyStateWithBidirektPublish() {
	this.createForShowParticipantVersions(nil, []string{"petstore_admin"})
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/participants/petstore_admin", cookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<p class="sub">never published</p>`)
	this.NotContains(body, `Latest`)
	this.Contains(body, `Versions <span class="count">0</span>`)
	this.Contains(body, `Deployments <span class="count">0</span>`)
	this.Contains(body, `Checks <span class="count">0</span>`)
	this.Contains(body, `Relations <span class="count">0</span>`)
	this.Contains(body, `<h2 class="h2">No versions yet</h2>`)
	this.Contains(body, `<code class="cmd">bidirekt publish</code>`)
	this.NotContains(body, `<table`)

	status, _, _ = this.getPage("/participants/petstore_admin?page=2", cookie)
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestShowParticipantVersions_PaginatesTwentyFivePerPage() {
	this.createForShowParticipantVersions(nil, []string{"petstore_api"})
	for i := 1; i <= 26; i++ {
		this.publishForShowParticipantVersions("petstore_api", "1."+strconv.Itoa(i)+".0", showParticipantVersionsAPIContract)
	}
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/participants/petstore_api", cookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal(25, strings.Count(body, `<td data-l="Version">`))
	this.Contains(body, `>1.26.0</a>`)
	this.NotContains(body, `>1.1.0</a>`)
	this.Contains(body, `showing 1–25 of 26`)
	this.Contains(body, `<a href="/participants/petstore_api?page=2" rel="next">Next</a>`)

	status, _, body = this.getPage("/participants/petstore_api?page=2", cookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal(1, strings.Count(body, `<td data-l="Version">`))
	this.Contains(body, `>1.1.0</a>`)
	this.Contains(body, `showing 26–26 of 26`)
	this.Contains(body, `<a href="/participants/petstore_api?page=1" rel="prev">Previous</a>`)

	for _, page := range []string{"3", "0", "01", "+1", "", "abc"} {
		status, _, _ = this.getPage("/participants/petstore_api?page="+page, cookie)
		this.Equal(http.StatusNotFound, status, page)
	}
}

func (this *IntegrationSuite) TestShowParticipantVersions_UnknownParticipantIsNotFound() {
	status, header, body := this.getPage("/participants/ghost", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "Not Found")
}

func (this *IntegrationSuite) TestShowParticipantVersions_ReadUserSeesThePageWithoutWriteActions() {
	this.seedForShowParticipantVersions()
	this.createActiveUser(showParticipantVersionsReaderEmail, model.PermissionRead, showParticipantVersionsReaderPassword)

	status, _, body := this.getPage("/participants/petstore_web", this.signInAsUser(showParticipantVersionsReaderEmail, showParticipantVersionsReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h1 class="mono">petstore_web</h1>`)
	this.NotContains(body, `action="/participants`)
}

func (this *IntegrationSuite) TestShowParticipantVersions_SignedOutIsRedirectedToSignIn() {
	this.createForShowParticipantVersions(nil, []string{"petstore_web"})

	status, header, _ := this.getPage("/participants/petstore_web", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fparticipants%2Fpetstore_web", header.Get("Location"))
}

func (this *IntegrationSuite) TestShowParticipantVersions_CountsOnlyTheParticipantsOwnRows() {
	this.seedForShowParticipantVersions()

	_, _, body := this.getPage("/participants/petstore_reviews", this.signInAsAdmin())

	this.Contains(body, `Versions <span class="count">1</span>`)
	this.Contains(body, `Deployments <span class="count">1</span>`)
	this.Contains(body, `Checks <span class="count">0</span>`)
	this.Contains(body, `Relations <span class="count">1</span>`)
}
