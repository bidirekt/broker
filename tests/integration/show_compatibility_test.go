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
consumes:
  petstore_reviews:
    rest:
      /reviews:
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

const showCompatibilityAdminContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

const showCompatibilityReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

var (
	showCompatibilityColumnName  = regexp.MustCompile(`<th scope="col"><a class="badge env" href="/environments/[0-9]+">([a-z0-9_]+)</a></th>`)
	showCompatibilityRowVersion  = regexp.MustCompile(`<th scope="row"><span class="vh"><a class="ver" href="/participants/petstore_api/versions/[0-9]+">([0-9]+)</a></span>`)
	showCompatibilityBreakCount  = regexp.MustCompile(`<div><b>([0-9]+)</b><span>breaks?</span></div>`)
	showCompatibilityVerdictText = regexp.MustCompile(`no · ([0-9]+) breaks?</a>`)
)

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

func (this *IntegrationSuite) tableForShowCompatibility() (compatibleID, breakingID, stagingID string) {
	this.createForShowCompatibility([]string{"staging", "production"},
		[]string{"petstore_web", "petstore_api", "petstore_admin", "petstore_reviews"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	this.publishForShowCompatibility("petstore_api", "1.5.0", showCompatibilityAPIRemovedContract)
	this.publishForShowCompatibility("petstore_web", "2.2.0", showCompatibilityWebContract)
	this.publishForShowCompatibility("petstore_admin", "3.0.0", showCompatibilityAdminContract)
	this.publishForShowCompatibility("petstore_reviews", "1.0.0", showCompatibilityReviewsContract)
	this.deployForShowCompatibility("petstore_api", "1.4.0", "production")
	this.deployForShowCompatibility("petstore_web", "2.2.0", "production")
	this.deployForShowCompatibility("petstore_admin", "3.0.0", "production")
	this.deployForShowCompatibility("petstore_reviews", "1.0.0", "staging")

	compatibleID = this.checkForShowCompatibility("petstore_api", "1.4.0", "production")
	breakingID = this.checkForShowCompatibility("petstore_api", "1.5.0", "production")
	stagingID = this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")

	return compatibleID, breakingID, stagingID
}

func (this *IntegrationSuite) publishManyForShowCompatibility(count int) {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1", showCompatibilityAPIContract)
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO contract_versions (participant_id, version, contract_id, contract_content)
		SELECT participant_id, (g + 1)::text, contract_id, contract_content
		FROM contract_versions CROSS JOIN generate_series(1, $1::int) g`, count-1)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) rowVersionsForShowCompatibility(body string) []string {
	var versions []string
	for _, match := range showCompatibilityRowVersion.FindAllStringSubmatch(body, -1) {
		versions = append(versions, match[1])
	}

	return versions
}

func (this *IntegrationSuite) TestShowCompatibility_ShowsTwoRowsPerVersion() {
	compatibleID, breakingID, stagingID := this.tableForShowCompatibility()

	status, header, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<a href="/compatibility" aria-current="page">Compatibility</a>`)

	this.Contains(body, `<div class="scroll"><table class="mt">`)
	this.Contains(body, `<th scope="col"><span class="muted">Version of</span> <span class="name">petstore_api</span></th>`)
	this.Contains(body, `<span class="muted">· 2 versions</span>`)
	this.Contains(body, `yes, can deploy</span>`)
	this.Contains(body, `no · breaks</span>`)
	this.Contains(body, `not checked yet</span></p>`)
	this.Equal(2, strings.Count(body, `<tr class="mrow-det">`))
	this.Equal(2, strings.Count(body, `<td colspan="3">`))

	newest := `<th scope="row"><span class="vh"><a class="ver" href="/participants/petstore_api/versions/1.5.0">1.5.0</a></span><span class="time">published <time class="time" datetime="`
	oldest := `<th scope="row"><span class="vh"><a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a></span><span class="time">published <time class="time" datetime="`
	this.Contains(body, newest)
	this.Contains(body, oldest)
	this.Less(strings.Index(body, newest), strings.Index(body, oldest))
	this.Contains(body, `<span class="dep-l">Deployed in</span><span class="badges dep"><span class="muted">not deployed</span></span></th>`)
	this.Contains(body, `<span class="dep-l">Deployed in</span><span class="badges dep"><span class="badge env">production</span></span></th>`)

	this.Contains(body, `<td class="mc bad" data-env="production"><a class="res bad" href="/checks/`+breakingID+`">`)
	this.Contains(body, `<td class="mc ok" data-env="production"><a class="res ok" href="/checks/`+compatibleID+`">`)
	this.Contains(body, `<td class="mc ok" data-env="staging"><a class="res ok" href="/checks/`+stagingID+`">`)
	this.Contains(body, `yes</a><span class="mc-sub">check <a href="/checks/`+compatibleID+`">#`+compatibleID+`</a> · <time class="time" datetime="`)
	this.Contains(body, `<span class="mc-sub">check <a href="/checks/`+breakingID+`">#`+breakingID+`</a> · <time class="time" datetime="`)
	this.Regexp(`<a class="res bad" href="/checks/`+breakingID+`"><svg[^>]*><path[^>]*/></svg>no · 2 breaks</a>`, body)
	this.Contains(body, `<td class="mc none" data-env="staging"><span class="res none">`)
	this.Contains(body, `not checked yet</span><span class="mc-sub">no can-i-deploy for 1.5.0 in staging yet</span></td>`)

	this.NotContains(body, `deployed version`)
	this.NotContains(body, `rc-stale`)
	this.NotContains(body, `Since this check`)
	this.NotContains(body, `class="rl"`)
	this.NotContains(body, `class="rt`)
}

func (this *IntegrationSuite) TestShowCompatibility_ListsEveryCounterpartBreakingFirst() {
	this.tableForShowCompatibility()

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	newestDetails := body[strings.Index(body, `compared with · <span class="ver">1.5.0</span>`):strings.Index(body, `compared with · <span class="ver">1.4.0</span>`)]
	this.Contains(newestDetails, `<div class="mdet-grid n2">`)
	production := newestDetails[strings.Index(newestDetails, `<p class="group-label"><span class="badge env">production</span></p>`):]

	reviews := `<td><a class="name" href="/participants/petstore_reviews">petstore_reviews</a> <span class="ver">no version</span></td>`
	web := `<td><a class="name" href="/participants/petstore_web">petstore_web</a> <span class="ver">2.2.0</span></td>`
	admin := `<td><a class="name" href="/participants/petstore_admin">petstore_admin</a> <span class="ver">3.0.0</span></td>`
	this.Contains(production, reviews)
	this.Contains(production, web)
	this.Contains(production, admin)
	this.Less(strings.Index(production, reviews), strings.Index(production, web))
	this.Less(strings.Index(production, web), strings.Index(production, admin))
	this.Contains(production, `1 break</span></td>`)
	this.Contains(production, `compatible</span></td>`)
	this.Contains(production, `<p class="muted small">no version: not deployed in production</p>`)
	this.NotContains(body, `more</`)
}

func (this *IntegrationSuite) TestShowCompatibility_OpensOnlyTheNewestVersionsDetails() {
	this.tableForShowCompatibility()

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	opened := `<details class="mdets" open>`
	closed := `<details class="mdets">`
	this.Equal(1, strings.Count(body, opened))
	this.Equal(1, strings.Count(body, closed))
	this.Less(strings.Index(body, opened), strings.Index(body, `compared with · <span class="ver">1.5.0</span>`))
	this.Less(strings.Index(body, `compared with · <span class="ver">1.5.0</span>`), strings.Index(body, closed))
	this.Less(strings.Index(body, closed), strings.Index(body, `compared with · <span class="ver">1.4.0</span>`))
}

func (this *IntegrationSuite) TestShowCompatibility_DetailsSayWhenThereIsNothingToList() {
	this.tableForShowCompatibility()

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	newestDetails := body[strings.Index(body, `compared with · <span class="ver">1.5.0</span>`):strings.Index(body, `compared with · <span class="ver">1.4.0</span>`)]
	this.Contains(newestDetails, `<p class="muted small">not checked yet</p>`)

	oldestDetails := body[strings.Index(body, `compared with · <span class="ver">1.4.0</span>`):]
	this.Contains(oldestDetails, `<p class="muted small">nothing deployed in staging shares a resource</p>`)
	this.NotContains(oldestDetails, `not checked yet`)
}

func (this *IntegrationSuite) TestShowCompatibility_ShowsOnlyTheLatestCheckPerVersionAndEnvironment() {
	this.createForShowCompatibility([]string{"staging"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	olderID := this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")
	latestID := this.checkForShowCompatibility("petstore_api", "1.4.0", "staging")

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Contains(body, `href="/checks/`+latestID+`"`)
	this.NotContains(body, `href="/checks/`+olderID+`"`)
	this.Contains(body, `<div class="mdet-grid n1">`)
}

func (this *IntegrationSuite) TestShowCompatibility_CountsTheSameBreaksAsTheCheckPage() {
	_, breakingID, _ := this.tableForShowCompatibility()
	cookie := this.signInAsAdmin()

	_, _, table := this.getPage("/compatibility?participant=petstore_api", cookie)
	_, _, check := this.getPage("/checks/"+breakingID, cookie)

	tableCount := showCompatibilityVerdictText.FindStringSubmatch(table)
	checkCount := showCompatibilityBreakCount.FindStringSubmatch(check)
	this.Require().NotNil(tableCount)
	this.Require().NotNil(checkCount)
	this.Equal(checkCount[1], tableCount[1])
}

func (this *IntegrationSuite) TestShowCompatibility_PagesFiveVersionsAtATime() {
	this.publishManyForShowCompatibility(7)
	this.deployForShowCompatibility("petstore_api", "1", "production")
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage("/compatibility?participant=petstore_api", cookie)
	this.Require().Equal(http.StatusOK, status, first)
	this.Equal([]string{"7", "6", "5", "4", "3"}, this.rowVersionsForShowCompatibility(first))
	this.Contains(first, `<span class="muted">· 7 versions</span>`)
	this.Contains(first, `href="/compatibility?page=2&amp;participant=petstore_api" rel="next"`)

	status, _, second := this.getPage("/compatibility?participant=petstore_api&page=2", cookie)
	this.Require().Equal(http.StatusOK, status, second)
	this.Equal([]string{"2", "1"}, this.rowVersionsForShowCompatibility(second))
	this.Contains(second, `href="/compatibility?page=1&amp;participant=petstore_api" rel="prev"`)

	for _, query := range []string{"page=3", "page=0", "page=01", "page=+1", "page=", "page=abc"} {
		status, _, _ := this.getPage("/compatibility?participant=petstore_api&"+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

func (this *IntegrationSuite) TestShowCompatibility_FooterIsNavigationOnly() {
	this.publishManyForShowCompatibility(2)

	_, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Contains(body, `<div class="sheet-foot"><nav class="pager" aria-label="Pages">`)
	this.Contains(body, `<span class="off">Previous</span>`)
	this.Contains(body, `<span aria-current="page">1</span>`)
	this.Contains(body, `<span class="off">Next</span>`)
	this.NotContains(body, `showing `)
}

func (this *IntegrationSuite) TestShowCompatibility_FiltersKeepTheParticipant() {
	this.createForShowCompatibility([]string{"production"}, []string{"petstore_web", "petstore_api", "petstore_reviews"})

	status, _, body := this.getPage("/compatibility?participant=petstore_api", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<form method="get" action="/compatibility" class="filters compact">`)
	this.Contains(body, `<option value="petstore_api" selected>petstore_api</option><option value="petstore_reviews">petstore_reviews</option><option value="petstore_web">petstore_web</option>`)
	this.NotContains(body, `name="versions"`)
	this.Contains(body, `<button type="submit" class="btn primary">Show</button>`)
	this.Contains(body, `<p class="filters-note">Newest first, 5 versions per page.</p>`)
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

func (this *IntegrationSuite) TestShowCompatibility_CheckWithoutReportShowsNoCount() {
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

	this.Regexp(`<a class="res bad" href="/checks/[0-9]+"><svg[^>]*><path[^>]*/></svg>no</a>`, body)
	this.Contains(body, `<td><a class="name" href="/participants/petstore_web">petstore_web</a> <span class="ver">2.2.0</span></td>`)
	this.Contains(body, `not compatible</span></td>`)
	this.NotContains(body, `break</span>`)
	this.NotContains(body, `breaks</span></td>`)
}

func (this *IntegrationSuite) TestShowCompatibility_DetailGridFollowsTheEnvironmentCount() {
	this.createForShowCompatibility([]string{"production", "qa", "staging"}, []string{"petstore_api"})
	this.publishForShowCompatibility("petstore_api", "1.4.0", showCompatibilityAPIContract)
	cookie := this.signInAsAdmin()

	_, _, body := this.getPage("/compatibility", cookie)
	this.Contains(body, `<div class="mdet-grid n3">`)
	this.Contains(body, `<td colspan="4">`)
	this.Equal(3, strings.Count(body, `<td class="mc none"`))

	this.createForShowCompatibility([]string{"dev", "eu", "us", "sandbox"}, nil)
	_, _, body = this.getPage("/compatibility", cookie)
	this.Contains(body, `<div class="mdet-grid n6">`)
	this.Equal(7, strings.Count(body, `<div class="mdet">`))
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

func (this *IntegrationSuite) TestShowCompatibility_ReadUserSeesThePageWithoutWriteActions() {
	this.tableForShowCompatibility()
	this.createActiveUser(showCompatibilityReaderEmail, model.PermissionRead, showCompatibilityReaderPassword)

	status, _, body := this.getPage("/compatibility?participant=petstore_api",
		this.signInAsUser(showCompatibilityReaderEmail, showCompatibilityReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `no · 2 breaks</a>`)
	this.Equal(1, strings.Count(body, `method="post"`))
	this.Contains(body, `<form method="post" action="/sign-out">`)
}

func (this *IntegrationSuite) TestShowCompatibility_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage("/compatibility", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fcompatibility", header.Get("Location"))
}
