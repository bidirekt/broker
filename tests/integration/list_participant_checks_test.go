package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	participantChecksReaderEmail    = "reader@x.com"
	participantChecksReaderPassword = "reader-correct-horse-battery"
)

const participantChecksAPIContract = `provides:
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

const participantChecksWebBreakingContract = `consumes:
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

const participantChecksWebCompatibleContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
            404: ~
`

const participantChecksReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

func (this *IntegrationSuite) mustPostForParticipantChecks(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForParticipantChecks(participant, version, content string) {
	this.mustPostForParticipantChecks("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForParticipantChecks(participant, version, environment string) {
	this.mustPostForParticipantChecks("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForParticipantChecks(participant, version string) (checkID int64) {
	this.mustPostForParticipantChecks("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)

	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return checkID
}

func (this *IntegrationSuite) createForParticipantChecks(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForParticipantChecks("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForParticipantChecks("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) webChecksForParticipantChecks() (compatibleID, breakingID int64) {
	this.createForParticipantChecks([]string{"production", "staging"}, []string{"petstore_api", "petstore_reviews", "petstore_web", "petstore_admin"})
	this.publishForParticipantChecks("petstore_api", "1.4.0", participantChecksAPIContract)
	this.deployForParticipantChecks("petstore_api", "1.4.0", "production")
	this.publishForParticipantChecks("petstore_reviews", "1.0.0", participantChecksReviewsContract)
	this.deployForParticipantChecks("petstore_reviews", "1.0.0", "staging")
	this.publishForParticipantChecks("petstore_web", "2.2.0", participantChecksWebCompatibleContract)
	this.deployForParticipantChecks("petstore_web", "2.2.0", "production")
	this.publishForParticipantChecks("petstore_web", "2.3.0", participantChecksWebBreakingContract)

	compatibleID = this.checkForParticipantChecks("petstore_web", "2.2.0")
	breakingID = this.checkForParticipantChecks("petstore_web", "2.3.0")

	return compatibleID, breakingID
}

func (this *IntegrationSuite) insertChecksForParticipantChecks(participant string, count int) {
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO compatibility_checks (participant_id, contract_id, version, environment_id, deployable, created_at)
		SELECT cv.participant_id, cv.contract_id, cv.version, e.id, true, '2026-01-01T00:00:00Z'
		FROM contract_versions cv
		JOIN participants p ON p.id = cv.participant_id
		CROSS JOIN environments e
		CROSS JOIN generate_series(1, $2)
		WHERE p.name = $1`, participant, count)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) environmentPathForParticipantChecks(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) TestListParticipantChecks_ShowsTheHeaderAndTheParticipantsOwnChecksNewestFirst() {
	compatibleID, breakingID := this.webChecksForParticipantChecks()
	otherID := this.checkForParticipantChecks("petstore_reviews", "1.0.0")
	production := this.environmentPathForParticipantChecks("production")

	status, header, body := this.getPage("/participants/petstore_web/checks", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))

	this.Contains(body, `<nav class="crumbs" aria-label="Breadcrumbs"><a href="/participants">participants</a><span aria-hidden="true">/</span><span aria-current="page">petstore_web</span></nav>`)
	this.Contains(body, `<h1><span class="name">petstore_web</span></h1>`)
	this.Contains(body, `<span>Latest <span class="ver">2.3.0</span></span>`)
	this.Contains(body, `<a class="badge env" href="`+production+`">production <span class="v">2.2.0</span></a>`)
	this.NotContains(body, `not deployed`)
	this.Contains(body, `<a href="/participants/petstore_web">Versions <span class="count">2</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/deployments">Deployments <span class="count">1</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/checks" aria-current="page">Checks <span class="count">2</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/relations">Relations <span class="count">4</span></a>`)

	breakingLink := `<a class="mono" href="/checks/` + strconv.FormatInt(breakingID, 10) + `">#` + strconv.FormatInt(breakingID, 10) + `</a>`
	compatibleLink := `<a class="mono" href="/checks/` + strconv.FormatInt(compatibleID, 10) + `">#` + strconv.FormatInt(compatibleID, 10) + `</a>`
	this.Contains(body, breakingLink)
	this.Contains(body, compatibleLink)
	this.Less(strings.Index(body, breakingLink), strings.Index(body, compatibleLink))
	this.NotContains(body, `href="/checks/`+strconv.FormatInt(otherID, 10)+`"`)

	this.Contains(body, `<span class="ver">2.3.0</span>`)
	this.Contains(body, `<td data-l="Environment"><a class="name" href="`+production+`">production</a></td>`)

	this.Contains(body, `no · 5 breaks</span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><span class="badge cpb no">ghost <span class="v">no version</span> · 1 break</span></span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><span class="badge cpb no">petstore_admin <span class="v">no version</span> · 1 break</span></span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><span class="badge cpb no">petstore_api <span class="v">1.4.0</span> · 2 breaks</span></span>`)
	this.Contains(body, `<a class="badge more" href="/checks/`+strconv.FormatInt(breakingID, 10)+`">+1 more</a>`)
	this.NotContains(body, `petstore_reviews <span class="v">`)

	this.Contains(body, `yes</span>`)
	this.Contains(body, `<span><span class="role">PROVIDER</span><span class="badge cpb">petstore_api <span class="v">1.4.0</span></span></span>`)
	this.Contains(body, `just now</time>`)
	this.Contains(body, `showing 1–2 of 2`)
}

func (this *IntegrationSuite) TestListParticipantChecks_CountsTheSameBreaksAsTheCheckPage() {
	_, breakingID := this.webChecksForParticipantChecks()
	cookie := this.signInAsAdmin()

	_, _, list := this.getPage("/participants/petstore_web/checks", cookie)
	_, _, check := this.getPage("/checks/"+strconv.FormatInt(breakingID, 10), cookie)

	this.Contains(list, `no · 5 breaks</span>`)
	this.Contains(check, `<div><b>5</b><span>breaks</span></div>`)
}

func (this *IntegrationSuite) TestListParticipantChecks_RelationsCountsOnlyDeployedConsumers() {
	this.webChecksForParticipantChecks()

	status, _, body := this.getPage("/participants/petstore_api/checks", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `Relations <span class="count">1</span>`)
	this.Contains(body, `Checks <span class="count">0</span>`)
}

func (this *IntegrationSuite) TestListParticipantChecks_YesWithNothingToCompareSaysNothingSharesAnEndpoint() {
	this.createForParticipantChecks([]string{"production"}, []string{"petstore_reviews"})
	this.publishForParticipantChecks("petstore_reviews", "1.0.0", participantChecksReviewsContract)
	this.checkForParticipantChecks("petstore_reviews", "1.0.0")

	_, _, body := this.getPage("/participants/petstore_reviews/checks", this.signInAsAdmin())

	this.Contains(body, `<span class="muted">nothing deployed shares an endpoint</span>`)
	this.Contains(body, `<span class="badge quiet">not deployed</span>`)
}

func (this *IntegrationSuite) TestListParticipantChecks_CheckWithoutAStoredReportShowsNoCountsOrRoles() {
	this.webChecksForParticipantChecks()
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL, environment_name = NULL`)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `UPDATE compatibility_check_results SET role = NULL`)
	this.Require().NoError(err)

	status, _, body := this.getPage("/participants/petstore_web/checks", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="badge no">`)
	this.Contains(body, `no</span>`)
	this.NotContains(body, ` break`)
	this.NotContains(body, `class="role"`)
	this.NotContains(body, `nothing deployed shares an endpoint`)
	this.Contains(body, `<span class="badge cpb">petstore_api <span class="v">1.4.0</span></span>`)
	this.Contains(body, `<span class="badge cpb quiet">ghost <span class="v">no version</span></span>`)
}

func (this *IntegrationSuite) TestListParticipantChecks_NeverPublishedShowsThePublishEmptyState() {
	this.createForParticipantChecks(nil, []string{"petstore_admin"})

	status, _, body := this.getPage("/participants/petstore_admin/checks", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span>Latest <span class="muted">never published</span></span>`)
	this.NotContains(body, `not deployed`)
	this.Contains(body, `<code class="cmd">bidirekt publish</code>`)
	this.Contains(body, `Checks <span class="count">0</span>`)
	this.Contains(body, `Relations <span class="count">0</span>`)
	this.NotContains(body, `<table`)
}

func (this *IntegrationSuite) TestListParticipantChecks_PublishedWithoutChecksShowsTheCanIDeployEmptyState() {
	this.createForParticipantChecks([]string{"production"}, []string{"petstore_reviews"})
	this.publishForParticipantChecks("petstore_reviews", "1.0.0", participantChecksReviewsContract)
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/participants/petstore_reviews/checks", cookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="badge quiet">not deployed</span>`)
	this.Contains(body, `<code class="cmd">bidirekt can-i-deploy</code>`)
	this.NotContains(body, `<table`)

	status, _, _ = this.getPage("/participants/petstore_reviews/checks?page=2", cookie)
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestListParticipantChecks_PaginatesNewestFirstAndRefusesInvalidPages() {
	this.createForParticipantChecks([]string{"production"}, []string{"petstore_reviews"})
	this.publishForParticipantChecks("petstore_reviews", "1.0.0", participantChecksReviewsContract)
	this.insertChecksForParticipantChecks("petstore_reviews", 26)
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage("/participants/petstore_reviews/checks", cookie)
	this.Require().Equal(http.StatusOK, status, first)
	this.Equal(25, strings.Count(first, `<a class="mono" href="/checks/`))
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `href="/participants/petstore_reviews/checks?page=2" rel="next"`)
	this.Less(strings.Index(first, `href="/checks/26"`), strings.Index(first, `href="/checks/25"`))
	this.NotContains(first, `href="/checks/1"`)
	this.Contains(first, `2026-01-01</time>`)

	status, _, second := this.getPage("/participants/petstore_reviews/checks?page=2", cookie)
	this.Require().Equal(http.StatusOK, status, second)
	this.Equal(1, strings.Count(second, `<a class="mono" href="/checks/`))
	this.Contains(second, `href="/checks/1"`)
	this.Contains(second, `showing 26–26 of 26`)

	for _, page := range []string{"3", "0", "01", "+1", "", "abc"} {
		status, _, _ := this.getPage("/participants/petstore_reviews/checks?page="+page, cookie)
		this.Equal(http.StatusNotFound, status, page)
	}
}

func (this *IntegrationSuite) TestListParticipantChecks_ReadUserSeesThePageWithoutWriteActions() {
	this.webChecksForParticipantChecks()
	this.createActiveUser(participantChecksReaderEmail, model.PermissionRead, participantChecksReaderPassword)

	status, _, body := this.getPage("/participants/petstore_web/checks", this.signInAsUser(participantChecksReaderEmail, participantChecksReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `no · 5 breaks</span>`)
	this.NotContains(body, `action="/participants`)
	this.NotContains(body, `/rename`)
}

func (this *IntegrationSuite) TestListParticipantChecks_SignedOutIsRedirectedToSignIn() {
	this.createForParticipantChecks(nil, []string{"petstore_web"})

	status, header, _ := this.getPage("/participants/petstore_web/checks", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestListParticipantChecks_UnknownParticipantIsNotFound() {
	status, header, body := this.getPage("/participants/ghost/checks", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "Not Found")
}
