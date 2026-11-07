package integration_test

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	showCheckReaderEmail    = "reader@x.com"
	showCheckReaderPassword = "reader-correct-horse-battery"
)

const showCheckAPIv1Contract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
    /pets/*:
      get:
        responses:
          200: Pet
          404: ~
    /pets/*/adopt:
      post:
        responses:
          204: ~
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
`

const showCheckWebConsumerContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pets
      /pets/*:
        get:
          responses:
            200: Pet
      /pets/*/adopt:
        post:
          responses:
            200: ~
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
  petstore_admin:
    rest:
      /admin:
        get:
          responses:
            200: ~
  ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
      status: { type: string }
`

const showCheckReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

const showCheckWebDeployedContract = `provides:
  rest:
    /config:
      get:
        responses:
          200: ~
consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
            404: ~
      /pets/*/adopt:
        post:
          responses:
            204: ~
`

const showCheckReviewsConsumerContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
`

const showCheckAPIv2Contract = `provides:
  rest:
    /pets/*:
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

const (
	showCheckChevron = `<span class="chev"><svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M6 3.5l4.5 4.5-4.5 4.5"/></svg></span>`
	showCheckLegend  = `<p class="legend"><span><span class="sw"></span>Fix in the participant under check</span><span><span class="sw o"></span>Or fix in the counterpart</span><span><span class="role">PROVIDER</span> it consumes from</span><span><span class="role">CONSUMER</span> consumes it</span></p>`
)

var showCheckSummary = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

func (this *IntegrationSuite) mustPostForShowCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForShowCheck(participant, version, content string) {
	this.mustPostForShowCheck("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForShowCheck(participant, version, environment string) {
	this.mustPostForShowCheck("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForShowCheck(participant, version string) (checkPath string) {
	this.mustPostForShowCheck("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return "/checks/" + strconv.FormatInt(checkID, 10)
}

func (this *IntegrationSuite) createForShowCheck(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowCheck("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowCheck("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) consumerSideCheckForShowCheck() (checkPath string) {
	this.createForShowCheck([]string{"production", "staging"}, []string{"petstore_api", "petstore_reviews", "petstore_web", "petstore_admin"})
	this.publishForShowCheck("petstore_api", "1.4.0", showCheckAPIv1Contract)
	this.deployForShowCheck("petstore_api", "1.4.0", "production")
	this.publishForShowCheck("petstore_reviews", "1.0.0", showCheckReviewsContract)
	this.deployForShowCheck("petstore_reviews", "1.0.0", "staging")
	this.publishForShowCheck("petstore_web", "2.3.0", showCheckWebConsumerContract)

	return this.checkForShowCheck("petstore_web", "2.3.0")
}

func (this *IntegrationSuite) providerSideCheckForShowCheck() (checkPath string) {
	this.createForShowCheck([]string{"production"}, []string{"petstore_api", "petstore_reviews", "petstore_web"})
	this.publishForShowCheck("petstore_api", "1.4.0", showCheckAPIv1Contract)
	this.deployForShowCheck("petstore_api", "1.4.0", "production")
	this.publishForShowCheck("petstore_web", "2.2.0", showCheckWebDeployedContract)
	this.deployForShowCheck("petstore_web", "2.2.0", "production")
	this.publishForShowCheck("petstore_reviews", "1.0.0", showCheckReviewsConsumerContract)
	this.deployForShowCheck("petstore_reviews", "1.0.0", "production")
	this.publishForShowCheck("petstore_api", "1.5.0", showCheckAPIv2Contract)

	return this.checkForShowCheck("petstore_api", "1.5.0")
}

func (this *IntegrationSuite) TestShowCheck_ShowsAConsumerSideNoWithItsBreaksReasonsAndPlaces() {
	checkPath := this.consumerSideCheckForShowCheck()

	status, header, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<a href="/participants" aria-current="page">Participants</a>`)
	this.Contains(body, `no · cannot be deployed`)
	this.Contains(body, `<h1><span class="name">petstore_web</span> <span class="ver">2.3.0</span> cannot be deployed to <span class="name">production</span></h1>`)
	this.Contains(body, ` UTC · just now</time>`)
	this.Contains(body, `<div><b>5</b><span>breaks</span></div>`)
	this.Contains(body, `<div><b>4 of 4</b><span>counterparts break</span></div>`)
	this.Contains(body, `<div><b>2</b><span>places in the contracts</span></div>`)
	this.Equal(4, strings.Count(body, `<details class="cp" open>`))
	this.Equal(4, strings.Count(body, `<summary>`+showCheckChevron+`<span class="role">`))
	this.Contains(body, showCheckLegend)

	this.Contains(body, `<p class="ix-name">response 200</p>`)
	this.Contains(body, `<b class="name">petstore_web</b> reads response 200, but <b class="name">petstore_api</b> answers only 204`)
	this.Contains(body, `petstore_api 1.4.0, deployed in production, declares the endpoint but not this interaction (declared: 204)`)
	this.Equal(3, strings.Count(body, `<p class="muted">not deployed in production</p>`))
	this.Contains(body, `<b class="name">petstore_web</b> calls <code>GET /admin</code>, but <b class="name">petstore_admin</b> is not deployed in <b class="name">production</b>`)
	this.Contains(body, `<b class="name">petstore_web</b> calls <code>GET /ghosts</code>, but <b class="name">ghost</b> is not deployed in <b class="name">production</b>`)

	this.Contains(body, `<span class="prop">FIELD</span><span>Pet.status</span><span class="badge">2 places</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> reads it, but <b class="name">petstore_api</b> doesn&#39;t provide it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop reading it, or handle its absence in the code and mark it optional</span></div>`)
	this.Contains(body, `<div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>provide it</span></div>`)
	this.Contains(body, `<div class="fix other only"><span class="side">Fix in <b>petstore_reviews</b></span><span>deploy it to production first</span></div>`)
	this.Contains(body, `<li><span class="meth">GET</span><span>/pets</span><span class="at">response 200</span><span>at &#34;$[].status&#34;</span></li>`)
	this.Contains(body, `<li><span class="meth">GET</span><span>/pets/*</span><span class="at">response 200</span><span>at &#34;$.status&#34;</span></li>`)

	this.Less(strings.Index(body, `<p class="group-label">Endpoints</p>`), strings.Index(body, `<p class="group-label">Fields</p>`))
}

func (this *IntegrationSuite) environmentPathForShowCheck(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) TestShowCheck_LinksTheCheckEnvironmentButNotTheSummaryAndNamesNoOtherEnvironment() {
	checkPath := this.consumerSideCheckForShowCheck()

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	summaries := showCheckSummary.FindAllStringSubmatch(body, -1)
	this.Require().Len(summaries, 4)
	for _, summary := range summaries {
		this.NotContains(summary[1], "<a ")
	}
	this.Contains(summaries[0][1], `<span class="role">PROVIDER</span><span class="who-cp">ghost</span><span class="muted">no version</span>`)
	this.Contains(summaries[1][1], `<span class="who-cp">petstore_admin</span><span class="muted">no version</span>`)
	this.Contains(summaries[2][1], `<span class="who-cp">petstore_api</span><span class="ver">1.4.0</span><span class="badge env">production</span>`)
	this.Contains(summaries[2][1], `no · 2 breaks`)
	this.Contains(summaries[3][1], `no · 1 break</span>`)
	this.Contains(body, `<div><dt>Environment</dt><dd><a class="name" href="`+this.environmentPathForShowCheck("production")+`">production</a></dd></div>`)
	this.NotContains(body, "staging")
}

func (this *IntegrationSuite) TestShowCheck_LinksTheCheckedParticipantAndItsVersion() {
	checkPath := this.consumerSideCheckForShowCheck()

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `<nav class="crumbs" aria-label="Breadcrumbs"><a href="/participants">participants</a><span aria-hidden="true">/</span><a href="/participants/petstore_web">petstore_web</a><span aria-hidden="true">/</span><a href="/participants/petstore_web/versions/2.3.0">2.3.0</a><span aria-hidden="true">/</span><span aria-current="page">check #`)
	this.Contains(body, `<div><dt>Participant</dt><dd><a class="name" href="/participants/petstore_web">petstore_web</a></dd></div>`)
	this.Contains(body, `<div><dt>Version</dt><dd><a class="ver" href="/participants/petstore_web/versions/2.3.0">2.3.0</a></dd></div>`)
}

func (this *IntegrationSuite) TestShowCheck_LinksRegisteredCounterpartsOutsideTheSummaryAndTheirVersionsOnlyWhenTheyHaveOne() {
	checkPath := this.consumerSideCheckForShowCheck()

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `</summary>
  <div class="cp-links"><a href="/participants/petstore_api">petstore_api</a><span aria-hidden="true">·</span><a href="/participants/petstore_api/versions/1.4.0">version 1.4.0</a></div>`)
	this.Contains(body, `</summary>
  <div class="cp-links"><a href="/participants/petstore_admin">petstore_admin</a></div>`)
	this.Contains(body, `</summary>
  <div class="cp-links"><a href="/participants/petstore_reviews">petstore_reviews</a></div>`)
	this.Equal(3, strings.Count(body, `<div class="cp-links">`))
	this.NotContains(body, `href="/participants/ghost`)
}

func (this *IntegrationSuite) TestShowCheck_VersionLinksArePathEscapedAndDotVersionsHaveNoURL() {
	this.createForShowCheck([]string{"production"}, []string{"petstore_api", "petstore_reviews"})
	this.publishForShowCheck("petstore_api", "..", showCheckAPIv1Contract)
	this.deployForShowCheck("petstore_api", "..", "production")
	this.publishForShowCheck("petstore_reviews", "1.0/beta rc", showCheckReviewsConsumerContract)
	checkPath := this.checkForShowCheck("petstore_reviews", "1.0/beta rc")

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `<a href="/participants/petstore_reviews/versions/1.0%2Fbeta%20rc">1.0/beta rc</a>`)
	this.Contains(body, `<a class="ver" href="/participants/petstore_reviews/versions/1.0%2Fbeta%20rc">1.0/beta rc</a>`)
	this.Contains(body, `<a class="name" href="/participants/petstore_api">petstore_api</a><span class="ver">..</span>`)
	this.NotContains(body, `/participants/petstore_api/versions/`)
}

func (this *IntegrationSuite) TestShowCheck_DotVersionOfTheCheckedParticipantHasNoURL() {
	this.createForShowCheck([]string{"production"}, []string{"petstore_reviews"})
	this.publishForShowCheck("petstore_reviews", ".", showCheckReviewsContract)
	checkPath := this.checkForShowCheck("petstore_reviews", ".")

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `<a href="/participants/petstore_reviews">petstore_reviews</a><span aria-hidden="true">/</span><span>.</span><span aria-hidden="true">/</span>`)
	this.Contains(body, `<div><dt>Version</dt><dd><span class="ver">.</span></dd></div>`)
	this.NotContains(body, `/versions/`)
}

func (this *IntegrationSuite) TestShowCheck_ShowsAProviderSideNoWithRemovedOncePerEndpointAndTheCompatibleCounterparts() {
	checkPath := this.providerSideCheckForShowCheck()

	status, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<div><b>2</b><span>breaks</span></div>`)
	this.Contains(body, `<div><b>1 of 2</b><span>counterparts break</span></div>`)
	this.NotContains(body, `in the contracts`)
	this.Equal(1, strings.Count(body, `<details class="cp" open>`))
	this.Contains(body, `<span class="role">PROVIDER · CONSUMER</span><span class="who-cp">petstore_web</span>`)

	this.Equal(1, strings.Count(body, `<b class="name">petstore_api</b> removed <code>POST /pets/*/adopt</code>, but <b class="name">petstore_web</b> still calls it`))
	removedAt := strings.Index(body, `<span>/pets/*/adopt</span></div>`)
	this.Require().Positive(removedAt)
	this.NotContains(body[removedAt:removedAt+120], `ix-name`)
	this.Contains(body, `<p class="ix-name">response 404</p>`)
	this.Contains(body, `<b class="name">petstore_api</b> now answers <code>GET /pets/*</code> only with 200, but <b class="name">petstore_web</b> still reads response 404`)

	this.Contains(body, `Also compared with, and compatible:`)
	this.Contains(body, `<span class="role">CONSUMER</span><a class="name" href="/participants/petstore_reviews">petstore_reviews</a> <a class="ver" href="/participants/petstore_reviews/versions/1.0.0">1.0.0</a>`)
}

func (this *IntegrationSuite) TestShowCheck_YesListsTheComparedCounterparts() {
	this.createForShowCheck([]string{"production"}, []string{"petstore_api", "petstore_reviews"})
	this.publishForShowCheck("petstore_api", "1.4.0", showCheckAPIv1Contract)
	this.deployForShowCheck("petstore_api", "1.4.0", "production")
	this.publishForShowCheck("petstore_reviews", "1.0.0", showCheckReviewsConsumerContract)
	checkPath := this.checkForShowCheck("petstore_reviews", "1.0.0")

	status, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `yes · can be deployed`)
	this.Contains(body, `<span class="ver">1.0.0</span> can be deployed to <span class="name">production</span>`)
	this.Contains(body, `<h2 class="h2">compared with</h2>`)
	this.Contains(body, `<a class="name" href="/participants/petstore_api">petstore_api</a><a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a><span class="role">PROVIDER</span>`)
	this.NotContains(body, `class="stats"`)
	this.NotContains(body, `<details`)
}

func (this *IntegrationSuite) TestShowCheck_YesWithNothingToCompareSaysNothingSharesAnEndpoint() {
	this.createForShowCheck([]string{"production"}, []string{"petstore_reviews"})
	this.publishForShowCheck("petstore_reviews", "1.0.0", showCheckReviewsContract)
	checkPath := this.checkForShowCheck("petstore_reviews", "1.0.0")

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `nothing deployed in <span class="name">production</span> shares an endpoint with <span class="name">petstore_reviews</span>`)
}

func (this *IntegrationSuite) TestShowCheck_KeepsTheEnvironmentNameOfTheCheck() {
	checkPath := this.consumerSideCheckForShowCheck()
	_, err := this.Pool.Exec(context.Background(), `UPDATE environments SET name = 'live' WHERE name = 'production'`)
	this.Require().NoError(err)

	_, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Contains(body, `cannot be deployed to <span class="name">production</span>`)
	this.Contains(body, `<a class="name" href="`+this.environmentPathForShowCheck("live")+`">production</a>`)
	this.NotContains(body, `live`)
}

func (this *IntegrationSuite) TestShowCheck_CheckWithoutAStoredReportSaysSoWithTheCurrentEnvironmentName() {
	checkPath := this.providerSideCheckForShowCheck()
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL, environment_name = NULL`)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `UPDATE compatibility_check_results SET role = NULL`)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `UPDATE environments SET name = 'live'`)
	this.Require().NoError(err)

	status, _, body := this.getPage(checkPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `cannot be deployed to <span class="name">live</span>`)
	this.Contains(body, `the details of this check were not stored`)
	this.Contains(body, `<a class="name" href="/participants/petstore_reviews">petstore_reviews</a><a class="ver" href="/participants/petstore_reviews/versions/1.0.0">1.0.0</a>`)
	this.Contains(body, `<a class="name" href="/participants/petstore_web">petstore_web</a><a class="ver" href="/participants/petstore_web/versions/2.2.0">2.2.0</a>`)
	this.NotContains(body, `class="role"`)
	this.NotContains(body, `class="stats"`)
	this.NotContains(body, `<details`)
}

func (this *IntegrationSuite) TestShowCheck_ReadUserSeesThePage() {
	checkPath := this.consumerSideCheckForShowCheck()
	this.createActiveUser(showCheckReaderEmail, model.PermissionRead, showCheckReaderPassword)

	status, _, body := this.getPage(checkPath, this.signInAsUser(showCheckReaderEmail, showCheckReaderPassword))

	this.Equal(http.StatusOK, status)
	this.Contains(body, `cannot be deployed to`)
	this.NotContains(body, `method="post" action="/checks`)
}

func (this *IntegrationSuite) TestShowCheck_SignedOutIsRedirectedToSignIn() {
	checkPath := this.consumerSideCheckForShowCheck()

	status, header, _ := this.getPage(checkPath, "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestShowCheck_UnknownOrInvalidIdIsNotFound() {
	this.consumerSideCheckForShowCheck()
	cookie := this.signInAsAdmin()

	for _, path := range []string{"/checks/999", "/checks/0", "/checks/01", "/checks/+1", "/checks/abc"} {
		status, header, body := this.getPage(path, cookie)
		this.Equal(http.StatusNotFound, status, path)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), path)
		this.Contains(body, "Not Found", path)
	}
}
