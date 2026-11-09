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
	showEnvironmentReaderEmail    = "reader@x.com"
	showEnvironmentReaderPassword = "reader-correct-horse-battery"
)

const showEnvironmentAPIContract = `provides:
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

const showEnvironmentWebBreakingContract = `consumes:
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

const showEnvironmentWebCompatibleContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
            404: ~
`

const showEnvironmentReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

var showEnvironmentParticipantName = regexp.MustCompile(`<td data-l="Participant"><a class="name" href="/participants/[a-z0-9_]+">([a-z0-9_]+)</a></td>`)

func (this *IntegrationSuite) mustPostForShowEnvironment(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForShowEnvironment(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowEnvironment("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowEnvironment("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) publishForShowEnvironment(participant, version, content string) {
	this.mustPostForShowEnvironment("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForShowEnvironment(participant, version, environment string) {
	this.mustPostForShowEnvironment("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForShowEnvironment(participant, version, environment string) (checkID int64) {
	this.mustPostForShowEnvironment("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)

	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return checkID
}

func (this *IntegrationSuite) environmentPathForShowEnvironment(environment string) string {
	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, environment).Scan(&environmentID))

	return "/environments/" + strconv.FormatInt(environmentID, 10)
}

func (this *IntegrationSuite) productionForShowEnvironment() (compatibleID, breakingID int64) {
	this.createForShowEnvironment([]string{"production", "staging"}, []string{"petstore_web", "petstore_api", "petstore_reviews", "petstore_admin"})
	this.publishForShowEnvironment("petstore_api", "1.3.0", showEnvironmentAPIContract)
	this.publishForShowEnvironment("petstore_api", "1.4.0", showEnvironmentAPIContract)
	this.deployForShowEnvironment("petstore_api", "1.3.0", "production")
	this.deployForShowEnvironment("petstore_api", "1.4.0", "production")
	this.deployForShowEnvironment("petstore_api", "1.3.0", "production")
	this.deployForShowEnvironment("petstore_api", "1.4.0", "production")
	this.publishForShowEnvironment("petstore_reviews", "1.0.0", showEnvironmentReviewsContract)
	this.deployForShowEnvironment("petstore_reviews", "1.0.0", "staging")
	this.publishForShowEnvironment("petstore_web", "2.2.0", showEnvironmentWebCompatibleContract)
	this.deployForShowEnvironment("petstore_web", "2.2.0", "production")
	this.publishForShowEnvironment("petstore_web", "2.3.0", showEnvironmentWebBreakingContract)

	compatibleID = this.checkForShowEnvironment("petstore_web", "2.2.0", "production")
	breakingID = this.checkForShowEnvironment("petstore_web", "2.3.0", "production")

	return compatibleID, breakingID
}

func (this *IntegrationSuite) TestShowEnvironment_ShowsWhatIsDeployedTheHistoryAndTheChecks() {
	compatibleID, breakingID := this.productionForShowEnvironment()
	stagingID := this.checkForShowEnvironment("petstore_web", "2.3.0", "staging")

	status, header, body := this.getPage(this.environmentPathForShowEnvironment("production"), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))

	this.Contains(body, `<h1 class="name">production</h1>`)
	this.Contains(body, `2 participants deployed · last deployment `)

	apiDeployed := `<td data-l="Participant"><a class="name" href="/participants/petstore_api">petstore_api</a></td>
        <td data-l="Version"><a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a></td>`
	webDeployed := `<td data-l="Participant"><a class="name" href="/participants/petstore_web">petstore_web</a></td>
        <td data-l="Version"><a class="ver" href="/participants/petstore_web/versions/2.2.0">2.2.0</a></td>`
	this.Contains(body, apiDeployed)
	this.Contains(body, webDeployed)
	this.Less(strings.Index(body, apiDeployed), strings.Index(body, webDeployed))
	this.NotContains(body, `>petstore_reviews</a></td>`)

	this.Contains(body, `<span class="muted">5 deployments, newest first</span>`)
	this.Equal(2, strings.Count(body, `<span class="badge rb">`))
	this.Contains(body, `showing 1–5 of 5`)

	breakingLink := `<a class="mono" href="/checks/` + strconv.FormatInt(breakingID, 10) + `">#` + strconv.FormatInt(breakingID, 10) + `</a>`
	compatibleLink := `<a class="mono" href="/checks/` + strconv.FormatInt(compatibleID, 10) + `">#` + strconv.FormatInt(compatibleID, 10) + `</a>`
	this.Contains(body, breakingLink)
	this.Contains(body, compatibleLink)
	this.Less(strings.Index(body, breakingLink), strings.Index(body, compatibleLink))
	this.NotContains(body, `href="/checks/`+strconv.FormatInt(stagingID, 10)+`"`)

	this.Regexp(`(?s)<h2 class="h2">Checks</h2>.*?<div class="scroll"><table class="t stack">`, body)
	this.Equal(3, strings.Count(body, `<div class="scroll"><table class="t stack">`))
	this.Equal(strings.Count(body, `<table`), strings.Count(body, `<div class="scroll"><table`))
	this.Contains(body, `<td data-l="Participant"><a class="name" href="/participants/petstore_web">petstore_web</a> <a class="ver" href="/participants/petstore_web/versions/2.3.0">2.3.0</a></td>`)
	this.Contains(body, `no · 5 breaks</span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><span class="badge cpb no">ghost <span class="v">no version</span> · 1 break</span></span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><a class="badge cpb no" href="/participants/petstore_admin">petstore_admin <span class="v">no version</span> · 1 break</a></span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><a class="badge cpb no" href="/participants/petstore_api">petstore_api <span class="v">1.4.0</span> · 2 breaks</a></span>`)
	this.Contains(body, `<a class="badge more" href="/checks/`+strconv.FormatInt(breakingID, 10)+`">+1 more</a>`)
	this.Contains(body, `yes</span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><a class="badge cpb" href="/participants/petstore_api">petstore_api <span class="v">1.4.0</span></a></span>`)
	this.Contains(body, `showing 1–2 of 2`)
	this.Contains(body, `just now</time>`)
}

func (this *IntegrationSuite) TestShowEnvironment_CountsTheSameBreaksAsTheCheckPage() {
	_, breakingID := this.productionForShowEnvironment()
	cookie := this.signInAsAdmin()

	_, _, environment := this.getPage(this.environmentPathForShowEnvironment("production"), cookie)
	_, _, check := this.getPage("/checks/"+strconv.FormatInt(breakingID, 10), cookie)

	this.Contains(environment, `no · 5 breaks</span>`)
	this.Contains(check, `<div><b>5</b><span>breaks</span></div>`)
}

func (this *IntegrationSuite) TestShowEnvironment_VersionLinksArePathEscapedAndDotVersionsHaveNoURL() {
	this.createForShowEnvironment([]string{"production"}, []string{"petstore_api", "petstore_web"})
	this.publishForShowEnvironment("petstore_api", "1.0/beta rc", showEnvironmentAPIContract)
	this.deployForShowEnvironment("petstore_api", "1.0/beta rc", "production")
	this.publishForShowEnvironment("petstore_web", "..", showEnvironmentWebCompatibleContract)
	this.deployForShowEnvironment("petstore_web", "..", "production")

	_, _, body := this.getPage(this.environmentPathForShowEnvironment("production"), this.signInAsAdmin())

	this.Contains(body, `<a class="ver" href="/participants/petstore_api/versions/1.0%2Fbeta%20rc">1.0/beta rc</a>`)
	this.Contains(body, `<span class="ver">..</span>`)
	this.NotContains(body, `/participants/petstore_web/versions/`)
}

func (this *IntegrationSuite) TestShowEnvironment_DeployedNowSortedByteWise() {
	participants := []string{"ordersapi", "orders_web", "orders2"}
	this.createForShowEnvironment([]string{"production"}, participants)
	for _, participant := range participants {
		this.publishForShowEnvironment(participant, "1.0.0", showEnvironmentReviewsContract)
		this.deployForShowEnvironment(participant, "1.0.0", "production")
	}

	_, _, body := this.getPage(this.environmentPathForShowEnvironment("production"), this.signInAsAdmin())

	_, deployedNow, _ := strings.Cut(body, `<h2 class="h2">Deployed now</h2>`)
	deployedNow, _, _ = strings.Cut(deployedNow, `<h2 class="h2">History</h2>`)
	var names []string
	for _, match := range showEnvironmentParticipantName.FindAllStringSubmatch(deployedNow, -1) {
		names = append(names, match[1])
	}
	this.Equal([]string{"orders2", "orders_web", "ordersapi"}, names)
}

func (this *IntegrationSuite) TestShowEnvironment_JustCreatedSaysNothingDeployedYetAndHasNoChecks() {
	this.createForShowEnvironment([]string{"qa"}, nil)
	path := this.environmentPathForShowEnvironment("qa")
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage(path, cookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<p class="sub">Nothing deployed yet</p>`)
	this.Contains(body, `<code class="cmd">bidirekt record-deployment</code>`)
	this.Contains(body, `<h2 class="h2">No checks yet</h2>`)
	this.Contains(body, `<code class="cmd">bidirekt can-i-deploy</code>`)
	this.NotContains(body, `Deployed now`)
	this.NotContains(body, `<table`)

	for _, query := range []string{"?history_page=2", "?checks_page=2"} {
		status, _, _ := this.getPage(path+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

func (this *IntegrationSuite) TestShowEnvironment_ChecksWithNothingDeployedShowTheChecks() {
	this.createForShowEnvironment([]string{"qa"}, []string{"petstore_reviews"})
	this.publishForShowEnvironment("petstore_reviews", "1.0.0", showEnvironmentReviewsContract)
	this.checkForShowEnvironment("petstore_reviews", "1.0.0", "qa")

	_, _, body := this.getPage(this.environmentPathForShowEnvironment("qa"), this.signInAsAdmin())

	this.Contains(body, `<p class="sub">Nothing deployed yet</p>`)
	this.Contains(body, `<span class="muted">nothing deployed shares a resource</span>`)
	this.NotContains(body, `No checks yet`)
}

func (this *IntegrationSuite) TestShowEnvironment_CheckWithoutAStoredReportShowsNoCountsOrRoles() {
	this.productionForShowEnvironment()
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL, environment_name = NULL`)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `UPDATE compatibility_check_results SET role = NULL`)
	this.Require().NoError(err)

	status, _, body := this.getPage(this.environmentPathForShowEnvironment("production"), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="badge no">`)
	this.Contains(body, `no</span>`)
	this.NotContains(body, ` break`)
	this.NotContains(body, `class="role"`)
	this.NotContains(body, `nothing deployed shares a resource`)
	this.Contains(body, `<a class="badge cpb" href="/participants/petstore_api">petstore_api <span class="v">1.4.0</span></a>`)
	this.Contains(body, `<span class="badge cpb quiet">ghost <span class="v">no version</span></span>`)
}

func (this *IntegrationSuite) TestShowEnvironment_PaginatesHistoryAndChecksIndependently() {
	this.createForShowEnvironment([]string{"production"}, []string{"petstore_reviews"})
	this.publishForShowEnvironment("petstore_reviews", "1.0.0", showEnvironmentReviewsContract)
	this.deployForShowEnvironment("petstore_reviews", "1.0.0", "production")
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO deployments (participant_id, version, environment_id, rollback, deployed_at)
		SELECT p.id, '1.0.0', e.id, true, '2026-01-01T00:00:00Z'
		FROM participants p CROSS JOIN environments e CROSS JOIN generate_series(1, 25)`)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `
		INSERT INTO compatibility_checks (participant_id, contract_id, version, environment_id, deployable, created_at)
		SELECT cv.participant_id, cv.contract_id, cv.version, e.id, true, '2026-01-01T00:00:00Z'
		FROM contract_versions cv CROSS JOIN environments e CROSS JOIN generate_series(1, 26)`)
	this.Require().NoError(err)
	path := this.environmentPathForShowEnvironment("production")
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage(path, cookie)
	this.Require().Equal(http.StatusOK, status, first)
	this.Contains(first, `<span class="muted">26 deployments, newest first</span>`)
	this.Contains(first, `showing 1–25 of 26`)
	this.Equal(25, strings.Count(first, `<a class="mono" href="/checks/`))
	this.Contains(first, `href="`+path+`?history_page=2" rel="next"`)
	this.Contains(first, `href="`+path+`?checks_page=2" rel="next"`)
	this.Less(strings.Index(first, `href="/checks/26"`), strings.Index(first, `href="/checks/25"`))
	this.NotContains(first, `href="/checks/1"`)

	status, _, second := this.getPage(path+"?history_page=2&checks_page=2", cookie)
	this.Require().Equal(http.StatusOK, status, second)
	this.Equal(1, strings.Count(second, `<span class="badge rb">`))
	this.Contains(second, `showing 26–26 of 26`)
	this.Equal(1, strings.Count(second, `<a class="mono" href="/checks/`))
	this.Contains(second, `href="/checks/1"`)
	this.Contains(second, `href="`+path+`?checks_page=2&amp;history_page=1" rel="prev"`)

	for _, query := range []string{"history_page=3", "checks_page=3", "history_page=0", "checks_page=01", "history_page=+1", "checks_page=", "history_page=abc"} {
		status, _, _ := this.getPage(path+"?"+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

func (this *IntegrationSuite) TestShowEnvironment_UnknownOrInvalidIDIsNotFound() {
	this.createForShowEnvironment([]string{"production"}, nil)
	cookie := this.signInAsAdmin()

	for _, id := range []string{"999999", "0", "01", "+1", "-1", "abc"} {
		status, header, body := this.getPage("/environments/"+id, cookie)
		this.Equal(http.StatusNotFound, status, id)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), id)
		this.Contains(body, "Not Found", id)
	}
}

func (this *IntegrationSuite) TestShowEnvironment_ReadUserSeesThePageWithoutWriteActions() {
	this.productionForShowEnvironment()
	this.createActiveUser(showEnvironmentReaderEmail, model.PermissionRead, showEnvironmentReaderPassword)

	status, _, body := this.getPage(this.environmentPathForShowEnvironment("production"),
		this.signInAsUser(showEnvironmentReaderEmail, showEnvironmentReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `no · 5 breaks</span>`)
	this.NotContains(body, `action="/environments`)
	this.NotContains(body, `/rename`)
}

func (this *IntegrationSuite) TestShowEnvironment_SignedOutIsRedirectedToSignIn() {
	this.createForShowEnvironment([]string{"production"}, nil)

	status, header, _ := this.getPage(this.environmentPathForShowEnvironment("production"), "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fenvironments%2F1", header.Get("Location"))
}

func (this *IntegrationSuite) TestShowEnvironment_ChecksIndexExists() {
	var indexes int
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_indexes WHERE tablename = 'compatibility_checks' AND indexdef LIKE '%(environment_id, created_at DESC)%'`,
	).Scan(&indexes))

	this.Equal(1, indexes)
}
