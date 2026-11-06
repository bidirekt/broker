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
	showCompatibilityReaderEmail    = "reader@x.com"
	showCompatibilityReaderPassword = "reader-correct-horse-battery"
)

const showCompatibilityAPIContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
    /pets/*:
      get:
        responses:
          200: ~
          404: ~
`

const showCompatibilityAPIRemovedContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

const showCompatibilityWebContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
            404: ~
`

const showCompatibilityReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

var showCompatibilityColumnName = regexp.MustCompile(`<th scope="col"><a class="badge env" href="/environments/[0-9]+">([a-z0-9_]+)</a></th>`)

func (this *IntegrationSuite) mustPostForShowCompatibility(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForShowCompatibility(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowCompatibility("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowCompatibility("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) publishForShowCompatibility(participant, version, content string) {
	this.mustPostForShowCompatibility("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForShowCompatibility(participant, version, environment string) {
	this.mustPostForShowCompatibility("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForShowCompatibility(participant, version, environment string) (checkID string) {
	this.mustPostForShowCompatibility("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)

	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&id))

	return strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) environmentIDForShowCompatibility(environment string) string {
	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, environment).Scan(&environmentID))

	return strconv.FormatInt(environmentID, 10)
}

func (this *IntegrationSuite) gridForShowCompatibility() (staleID, breakingID, stagingID string) {
	this.createForShowCompatibility([]string{"staging", "production"}, []string{"petstore_web", "petstore_api", "petstore_reviews"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	this.publishForShowCompatibility("petstore_api", "1.5.0", showCompatibilityAPIRemovedContract)
	this.publishForShowCompatibility("petstore_web", "2.2.0", showCompatibilityWebContract)
	this.publishForShowCompatibility("petstore_reviews", "1.0.0", showCompatibilityReviewsContract)
	this.deployForShowCompatibility("petstore_api", "1.4.0", "production")
	this.deployForShowCompatibility("petstore_web", "2.2.0", "production")

	this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")
	staleID = this.checkForShowCompatibility("petstore_api", "1.4.0", "production")
	this.publishForShowCompatibility("petstore_web", "2.3.0", showCompatibilityWebContract)
	this.deployForShowCompatibility("petstore_web", "2.3.0", "production")
	this.deployForShowCompatibility("petstore_reviews", "1.0.0", "production")
	this.deployForShowCompatibility("petstore_reviews", "1.0.0", "staging")
	this.deployForShowCompatibility("petstore_api", "1.4.0", "production")
	breakingID = this.checkForShowCompatibility("petstore_api", "1.5.0", "production")
	stagingID = this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")

	return staleID, breakingID, stagingID
}

func (this *IntegrationSuite) TestShowCompatibility_ShowsTheVersionsByEnvironmentsGrid() {
	staleID, breakingID, stagingID := this.gridForShowCompatibility()

	status, header, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<a href="/compatibility" aria-current="page">Compatibility</a>`)

	this.Contains(body, `<form method="get" action="/compatibility" class="filters compact">`)
	this.Contains(body, `<option value="petstore_api" selected>petstore_api</option><option value="petstore_reviews">petstore_reviews</option><option value="petstore_web">petstore_web</option>`)
	this.Contains(body, `<button type="submit" class="btn primary">Show</button>`)

	production := `<th scope="col"><a class="badge env" href="/environments/` + this.environmentIDForShowCompatibility("production") + `">production</a></th>`
	staging := `<th scope="col"><a class="badge env" href="/environments/` + this.environmentIDForShowCompatibility("staging") + `">staging</a></th>`
	this.Contains(body, production)
	this.Contains(body, staging)
	this.Less(strings.Index(body, production), strings.Index(body, staging))
	this.Contains(body, `<div class="scroll"><table class="rt">`)

	newest := `<a class="ver" href="/participants/petstore_api/versions/1.5.0">1.5.0</a>`
	oldest := `<a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a>`
	this.Contains(body, newest)
	this.Contains(body, oldest)
	this.Less(strings.Index(body, newest), strings.Index(body, oldest))
	this.Contains(body, `<span class="muted">· 2 versions</span>`)

	this.Contains(body, `<a class="res bad" href="/checks/`+breakingID+`">`)
	this.Contains(body, `cannot deploy · 1 break</a>`)
	this.Contains(body, `<span class="rc-sub">check <a href="/checks/`+breakingID+`">#`+breakingID+`</a> · <time class="time" datetime="`)
	this.Contains(body, `<td><a class="name" href="/participants/petstore_web">petstore_web</a></td>
                <td class="rl">consumer</td>
                <td class="ver">2.3.0</td>
                <td class="st-cell"><span class="st bad">`)
	this.Contains(body, `1 break</span></td>`)
	this.Contains(body, `<td class="ver">2.2.0</td>
                <td class="st-cell"><span class="st ok">`)

	this.Contains(body, `<a class="res ok" href="/checks/`+staleID+`">`)
	this.Equal(1, strings.Count(body, `<span class="badge deployed">deployed version</span>`))
	this.NotContains(body, `<span class="badge env">deployed version</span>`)
	this.Contains(body, `<span>Since this check, 2 deployments to production: petstore_reviews, petstore_web. Ask again before deploying.</span>`)
	this.Equal(1, strings.Count(body, `class="rc-stale"`))

	this.Contains(body, `href="/checks/`+stagingID+`"`)
	this.Equal(3, strings.Count(body, `<td class="rc ok"`)+strings.Count(body, `<td class="rc bad"`))
	this.Contains(body, `not checked yet</span>`)
	this.Contains(body, `<p class="rc-sub">no can-i-deploy for 1.5.0 in staging yet</p>`)
	this.Contains(body, `showing 1–2 of 2`)
}

func (this *IntegrationSuite) TestShowCompatibility_ShowsOnlyTheLatestCheckPerVersionAndEnvironment() {
	this.createForShowCompatibility([]string{"staging"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	olderID := this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")
	latestID := this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Contains(body, `href="/checks/`+latestID+`"`)
	this.NotContains(body, `href="/checks/`+olderID+`"`)
	this.Contains(body, `<p class="rc-sub">nothing deployed in staging shares an endpoint</p>`)
}

func (this *IntegrationSuite) TestShowCompatibility_CountsTheSameBreaksAsTheCheckPage() {
	_, breakingID, _ := this.gridForShowCompatibility()
	cookie := this.signInAsAdmin()

	_, _, grid := this.getPage("/compatibility?participant=petstore_api", cookie)
	_, _, check := this.getPage("/checks/"+breakingID, cookie)

	this.Contains(grid, `cannot deploy · 1 break</a>`)
	this.Contains(check, `<div><b>1</b><span>break</span></div>`)
}

func (this *IntegrationSuite) TestShowCompatibility_StaleNoteIgnoresTheCheckedParticipantsOwnDeployments() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	this.publishForShowCompatibility("petstore_api", "1.5.0", showCompatibilityAPIContract)
	this.checkForShowCompatibility("petstore_api", "1.4.0", "production")
	this.deployForShowCompatibility("petstore_api", "1.5.0", "production")

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.NotContains(body, `Since this check`)
}

func (this *IntegrationSuite) TestShowCompatibility_WithoutParticipantShowsTheFirstAlphabetically() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_web", "petstore_admin", "petstore_api"})

	status, _, body := this.getPage("/compatibility", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<option value="petstore_admin" selected>petstore_admin</option><option value="petstore_api">petstore_api</option>`)
}

func (this *IntegrationSuite) TestShowCompatibility_UnknownParticipantIsNotFound() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_api"})
	cookie := this.signInAsAdmin()

	for _, query := range []string{"?participant=ghost", "?participant=", "?participant=PETSTORE_API"} {
		status, header, body := this.getPage("/compatibility"+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), query)
		this.Contains(body, "Not Found", query)
	}
}

func (this *IntegrationSuite) TestShowCompatibility_NoParticipantsAsksToCreateOne() {
	status, _, body := this.getPage("/compatibility", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h2 class="h2">No participants yet</h2>`)
	this.Contains(body, `<code class="cmd">bidirekt create-participant</code>`)
	this.NotContains(body, `<form method="get"`)

	status, _, _ = this.getPage("/compatibility?page=2", this.signInAsAdmin())
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestShowCompatibility_NoEnvironmentsAsksToCreateOne() {
	this.createForShowCompatibility(nil, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)

	status, _, body := this.getPage("/compatibility", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h2 class="h2">No environments yet</h2>`)
	this.Contains(body, `<code class="cmd">bidirekt create-environment</code>`)
	this.NotContains(body, `<table`)
}

func (this *IntegrationSuite) TestShowCompatibility_NeverPublishedHasNothingToCompare() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_admin"})

	status, _, body := this.getPage("/compatibility?participant=petstore_admin", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h2 class="h2">Nothing to compare</h2>`)
	this.Contains(body, `<p><a class="name" href="/participants/petstore_admin">petstore_admin</a> was never published, so it has no versions yet.</p>`)
	this.Contains(body, `<code class="cmd">bidirekt publish</code>`)
	this.NotContains(body, `<table`)
}

func (this *IntegrationSuite) TestShowCompatibility_CheckWithoutReportShowsNoRoleAndNoCount() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_api", "petstore_web"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	_, err := this.Pool.Exec(context.Background(), `
		WITH c AS (
			INSERT INTO compatibility_checks (participant_id, contract_id, version, environment_id, deployable)
			SELECT cv.participant_id, cv.contract_id, cv.version, e.id, false
			FROM contract_versions cv CROSS JOIN environments e
			RETURNING id
		)
		INSERT INTO compatibility_check_results (check_id, counterpart_name, counterpart_participant_id, counterpart_version, deployable)
		SELECT c.id, 'petstore_web', p.id, '2.2.0', false FROM c CROSS JOIN participants p WHERE p.name = 'petstore_web'`)
	this.Require().NoError(err)

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Contains(body, `cannot deploy</a>`)
	this.NotContains(body, `cannot deploy · `)
	this.Contains(body, `<td class="rl"></td>`)
	this.Contains(body, `not compatible</span>`)
	this.NotContains(body, `break</span>`)
}

func (this *IntegrationSuite) TestShowCompatibility_MoreThanTwoEnvironmentsStack() {
	this.createForShowCompatibility([]string{"production", "qa", "staging"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)

	_, _, body := this.getPage("/compatibility", this.signInAsAdmin())

	this.Contains(body, `<div class="scroll"><table class="rt stacked">`)
	this.Equal(3, strings.Count(body, `<td class="rc none"`))
}

func (this *IntegrationSuite) TestShowCompatibility_EnvironmentColumnsSortedByteWise() {
	this.createForShowCompatibility([]string{"stagingeu", "staging_us", "staging2"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)

	_, _, body := this.getPage("/compatibility", this.signInAsAdmin())

	var names []string
	for _, match := range showCompatibilityColumnName.FindAllStringSubmatch(body, -1) {
		names = append(names, match[1])
	}
	this.Equal([]string{"staging2", "staging_us", "stagingeu"}, names)
}

func (this *IntegrationSuite) TestShowCompatibility_PaginatesTheVersions() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1", showCompatibilityAPIContract)
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO contract_versions (participant_id, version, contract_id, contract_content)
		SELECT participant_id, (g + 1)::text, contract_id, contract_content
		FROM contract_versions CROSS JOIN generate_series(1, 25) g`)
	this.Require().NoError(err)
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage("/compatibility?participant=petstore_api", cookie)
	this.Require().Equal(http.StatusOK, status, first)
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `>26</a>`)
	this.NotContains(first, `versions/1">1</a>`)
	this.Contains(first, `href="/compatibility?page=2&amp;participant=petstore_api" rel="next"`)

	status, _, second := this.getPage("/compatibility?participant=petstore_api&page=2", cookie)
	this.Require().Equal(http.StatusOK, status, second)
	this.Contains(second, `showing 26–26 of 26`)
	this.Contains(second, `<a class="ver" href="/participants/petstore_api/versions/1">1</a>`)
	this.Contains(second, `href="/compatibility?page=1&amp;participant=petstore_api" rel="prev"`)

	for _, query := range []string{"page=3", "page=0", "page=01", "page=+1", "page=", "page=abc"} {
		status, _, _ := this.getPage("/compatibility?participant=petstore_api&"+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

func (this *IntegrationSuite) TestShowCompatibility_ReadUserSeesThePageWithoutWriteActions() {
	this.gridForShowCompatibility()
	this.createActiveUser(showCompatibilityReaderEmail, model.PermissionRead, showCompatibilityReaderPassword)

	status, _, body := this.getPage("/compatibility?participant=petstore_api",
		this.signInAsUser(showCompatibilityReaderEmail, showCompatibilityReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `cannot deploy · 1 break</a>`)
	this.Equal(1, strings.Count(body, `method="post"`))
	this.Contains(body, `<form method="post" action="/sign-out">`)
}

func (this *IntegrationSuite) TestShowCompatibility_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage("/compatibility", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}
