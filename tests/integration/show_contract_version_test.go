package integration_test

import (
	"context"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	contractVersionReaderEmail    = "version-reader@x.com"
	contractVersionReaderPassword = "version-reader-correct-horse"

	contractVersionConsumesSource = "/home/runner/work/version_web/contracts/consumes.yaml"
	contractVersionSchemasSource  = "contracts/schemas.yaml"
)

const contractVersionProviderContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
`

const contractVersionConsumesFile = `# reads <pets> & "owners"
consumes:
  version_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet

      /owners:
        get:
          responses:
            200: ~
`

const contractVersionSchemasFile = `schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
      status: { type: string }`

var (
	contractVersionCodeBlock = regexp.MustCompile(`(?s)<pre class="code">(.*?)</pre>`)
	contractVersionCodeLine  = regexp.MustCompile(`(?s)<span class="l">(.*?)</span>`)
	contractVersionBreaks    = regexp.MustCompile(`<div><b>(\d+)</b><span>breaks?</span></div>`)
	contractVersionCheckRow  = regexp.MustCompile(`<td data-l="Check"><a href="/checks/\d+">`)
)

func (this *IntegrationSuite) mustPostForContractVersion(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) deployForContractVersion(participant, version, environment string) {
	this.mustPostForContractVersion("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForContractVersion(participant, version, environment string) (checkID int64) {
	this.mustPostForContractVersion("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)

	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return checkID
}

func (this *IntegrationSuite) publishForContractVersion() {
	for _, environment := range []string{"production", "staging"} {
		this.mustPostForContractVersion("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range []string{"version_api", "version_web"} {
		this.mustPostForContractVersion("/api/participants", `{"participant":"`+participant+`"}`)
	}
	this.mustPostForContractVersion("/api/contracts", this.publishBody("version_api", "1.0.0",
		contractFragment{"contract.yaml", contractVersionProviderContract}))
	this.deployForContractVersion("version_api", "1.0.0", "production")
	this.mustPostForContractVersion("/api/contracts", this.publishBody("version_web", "release/2.0",
		contractFragment{contractVersionConsumesSource, contractVersionConsumesFile},
		contractFragment{contractVersionSchemasSource, contractVersionSchemasFile},
	))
}

const contractVersionPath = "/participants/version_web/versions/release%2F2.0"

func (this *IntegrationSuite) TestShowContractVersion_ShowsTheFilesExactlyAsPublished() {
	this.publishForContractVersion()

	status, header, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1><span class="name">version_web</span> <span class="ver">release/2.0</span></h1>`)
	this.Contains(body, `<p class="sub">Published <time class="time" datetime="`)
	this.Regexp(`<p class="sub">Published <time class="time" datetime="[^"]+Z" title="\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC">\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC · just now</time></p>`, body)
	this.Contains(body, `Exactly as published · 2 files`)

	this.Contains(body, `<span class="path">`+contractVersionConsumesSource+`</span><span class="muted">13 lines · select the text to copy</span>`)
	this.Contains(body, `<span class="path">`+contractVersionSchemasSource+`</span><span class="muted">6 lines · select the text to copy</span>`)
	this.Less(strings.Index(body, `<span class="path">`+contractVersionConsumesSource), strings.Index(body, `<span class="path">`+contractVersionSchemasSource))

	blocks := contractVersionCodeBlock.FindAllStringSubmatch(body, -1)
	this.Require().Len(blocks, 2)
	for index, expected := range []string{contractVersionConsumesFile, contractVersionSchemasFile} {
		var content strings.Builder
		lines := contractVersionCodeLine.FindAllStringSubmatch(blocks[index][1], -1)
		for _, line := range lines {
			content.WriteString(html.UnescapeString(line[1]))
		}
		this.Equal(expected, content.String(), "file %d", index)
		this.Equal(strings.Count(blocks[index][1], `<span class="l">`), len(lines))
	}
	this.Contains(body, `<span class="l"># reads &lt;pets&gt; &amp; &#34;owners&#34;`+"\n"+`</span>`)
	this.Contains(body, `<span class="l">`+"\n"+`</span>`)
	this.NotContains(body, `class="k"`)
	this.NotContains(body, `class="s"`)
}

func (this *IntegrationSuite) TestShowContractVersion_BreadcrumbsLinkToTheParticipantsAndTheParticipant() {
	this.publishForContractVersion()

	status, _, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<nav class="crumbs" aria-label="Breadcrumbs"><a href="/participants">participants</a><span aria-hidden="true">/</span><a href="/participants/version_web">version_web</a><span aria-hidden="true">/</span><span aria-current="page">release/2.0</span></nav>`)
}

func (this *IntegrationSuite) environmentPathForContractVersion(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) TestShowContractVersion_ShowsDeployedNowAndTheDeploymentHistoryNewestFirst() {
	this.publishForContractVersion()
	this.mustPostForContractVersion("/api/contracts", this.publishBody("version_web", "3.0.0",
		contractFragment{contractVersionConsumesSource, contractVersionConsumesFile},
		contractFragment{contractVersionSchemasSource, contractVersionSchemasFile},
	))
	this.deployForContractVersion("version_web", "release/2.0", "staging")
	this.deployForContractVersion("version_web", "release/2.0", "production")
	this.deployForContractVersion("version_web", "3.0.0", "production")
	this.deployForContractVersion("version_web", "release/2.0", "production")
	production := `<a class="badge env" href="` + this.environmentPathForContractVersion("production") + `">production</a>`
	staging := `<a class="badge env" href="` + this.environmentPathForContractVersion("staging") + `">staging</a>`

	_, _, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Regexp(`(?s)<h2 class="h2">Deployed now</h2>.*?<li>`+production+`<span class="muted">since <time class="time" datetime="[^"]+" title="[^"]+ UTC">[^<]+ UTC · just now</time></span></li>\s*<li>`+staging, body)

	history := body[strings.Index(body, `Deployment history`):strings.Index(body, `All checks`)]
	this.Equal(3, strings.Count(history, `<tr>`)-1)
	this.Equal(1, strings.Count(history, `rollback</span>`))
	rows := strings.Split(history, `<tr>`)[2:]
	this.Contains(rows[0], production+`<span class="badge rb">`)
	this.Contains(rows[1], production+`</span>`)
	this.Contains(rows[2], staging+`</span>`)
	this.Contains(rows[0], ` UTC · just now</time>`)
}

func (this *IntegrationSuite) TestShowContractVersion_SaysWhenNotDeployedNorChecked() {
	this.publishForContractVersion()

	_, _, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Equal(2, strings.Count(body, `<p class="pad muted">not deployed</p>`))
	this.Equal(2, strings.Count(body, `<p class="pad muted">not checked yet</p>`))
	this.NotContains(body, `class="pager"`)
}

func (this *IntegrationSuite) breaksOnCheckPageForContractVersion(checkID int64, cookie string) (breaks string) {
	_, _, checkPage := this.getPage("/checks/"+strconv.FormatInt(checkID, 10), cookie)
	match := contractVersionBreaks.FindStringSubmatch(checkPage)
	this.Require().Len(match, 2, checkPage)

	return match[1] + " " + map[bool]string{true: "break", false: "breaks"}[match[1] == "1"]
}

func (this *IntegrationSuite) TestShowContractVersion_ShowsTheLatestCheckPerEnvironmentWithTheCheckPageBreakCount() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_web", "release/2.0", "production")
	latestProduction := this.checkForContractVersion("version_web", "release/2.0", "production")
	latestStaging := this.checkForContractVersion("version_web", "release/2.0", "staging")
	cookie := this.signInAsAdmin()
	productionBreaks := this.breaksOnCheckPageForContractVersion(latestProduction, cookie)
	stagingBreaks := this.breaksOnCheckPageForContractVersion(latestStaging, cookie)
	production := strconv.FormatInt(latestProduction, 10)
	staging := strconv.FormatInt(latestStaging, 10)
	productionBadge := `<a class="badge env" href="` + this.environmentPathForContractVersion("production") + `">production</a>`
	stagingBadge := `<a class="badge env" href="` + this.environmentPathForContractVersion("staging") + `">staging</a>`

	status, _, body := this.getPage(contractVersionPath, cookie)
	this.Require().Equal(http.StatusOK, status, body)

	latest := body[strings.Index(body, `Latest check per environment`):strings.Index(body, `<h2 class="h2">Contract</h2>`)]
	this.Equal(2, strings.Count(latest, `<li>`))
	this.Regexp(`<li>`+productionBadge+`<span class="badge no">.*?no · `+productionBreaks+`</span><a href="/checks/`+production+`">check #`+production+`</a><span class="muted"><time class="time" datetime="[^"]+" title="[^"]+ UTC">[^<]+ UTC · just now</time></span></li>`, latest)
	this.Regexp(`<li>`+stagingBadge+`<span class="badge no">.*?no · `+stagingBreaks+`</span><a href="/checks/`+staging+`">check #`+staging+`</a>`, latest)
	this.Less(strings.Index(latest, `production`), strings.Index(latest, `staging`))

	all := body[strings.Index(body, `All checks`):]
	this.Len(contractVersionCheckRow.FindAllString(all, -1), 3)
	this.Less(strings.Index(all, `#`+staging+`</a>`), strings.Index(all, `#`+production+`</a>`))
	this.Contains(all, `<span><span class="role">PROVIDER</span><span class="badge cpb no">version_api <span class="v">1.0.0</span> · `+productionBreaks+`</span></span>`)
	this.Contains(all, `<span><span class="role">PROVIDER</span><span class="badge cpb no">version_api <span class="v">no version</span> · `+stagingBreaks+`</span></span>`)
	this.Contains(all, `<td data-l="Answer"><span class="badge no">`)
	this.Contains(all, `<td data-l="Environment">`+productionBadge+`</td>`)
	this.Contains(all, `<td data-l="Environment">`+stagingBadge+`</td>`)
}

func (this *IntegrationSuite) TestShowContractVersion_CheckWithoutAStoredReportShowsNoCount() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_web", "release/2.0", "production")
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL`)
	this.Require().NoError(err)

	_, _, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Contains(body, `<span class="badge no">`)
	this.NotContains(body, `no · `)
	this.NotContains(body, `class="role"`)
	this.Contains(body, `<span class="badge cpb">version_api <span class="v">1.0.0</span></span>`)
}

func (this *IntegrationSuite) TestShowContractVersion_CounterpartWithNoVersionAndNoBreaksIsQuiet() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_web", "release/2.0", "staging")
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL`)
	this.Require().NoError(err)

	_, _, body := this.getPage(contractVersionPath, this.signInAsAdmin())

	this.Contains(body, `<span><span class="badge cpb quiet">version_api <span class="v">no version</span></span></span>`)
}

const contractVersionFourProvidersContract = `consumes:
  version_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
  version_auth:
    rest:
      /pets:
        get:
          responses:
            200: Pet
  version_ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
  version_zombie:
    rest:
      /zombies:
        get:
          responses:
            200: ~
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
`

func (this *IntegrationSuite) TestShowContractVersion_ComparedWithShowsThreeCounterpartsBreakingFirstThenMore() {
	this.publishForContractVersion()
	this.mustPostForContractVersion("/api/participants", `{"participant":"version_auth"}`)
	this.mustPostForContractVersion("/api/contracts", this.publishBody("version_auth", "1.0.0",
		contractFragment{"contract.yaml", contractVersionProviderContract}))
	this.deployForContractVersion("version_auth", "1.0.0", "production")
	this.mustPostForContractVersion("/api/contracts", this.publishBody("version_web", "4.0.0",
		contractFragment{"contract.yaml", contractVersionFourProvidersContract}))
	checkID := strconv.FormatInt(this.checkForContractVersion("version_web", "4.0.0", "production"), 10)

	status, _, body := this.getPage("/participants/version_web/versions/4.0.0", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	all := body[strings.Index(body, `All checks`):]
	this.Equal(3, strings.Count(all, `class="badge cpb`))
	this.Contains(all, `<span><span class="role">PROVIDER</span><span class="badge cpb no">version_ghost <span class="v">no version</span> · 1 break</span></span>`+
		`<span><span class="role">PROVIDER</span><span class="badge cpb no">version_zombie <span class="v">no version</span> · 1 break</span></span>`+
		`<span><span class="role">PROVIDER</span><span class="badge cpb">version_api <span class="v">1.0.0</span></span></span>`)
	this.NotContains(all, `version_auth`)
	this.Contains(all, `<a class="badge more" href="/checks/`+checkID+`">+1 more</a>`)
}

const contractVersionProviderPath = "/participants/version_api/versions/1.0.0"

func (this *IntegrationSuite) TestShowContractVersion_StoredCheckWithNoCounterpartsSaysNothingDeployedSharesAnEndpoint() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_api", "1.0.0", "production")

	_, _, body := this.getPage(contractVersionProviderPath, this.signInAsAdmin())

	this.Contains(body, `<span class="muted">nothing deployed shares an endpoint</span>`)
}

func (this *IntegrationSuite) TestShowContractVersion_CheckWithoutAStoredReportOrCounterpartsLeavesComparedWithEmpty() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_api", "1.0.0", "production")
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET report = NULL`)
	this.Require().NoError(err)

	_, _, body := this.getPage(contractVersionProviderPath, this.signInAsAdmin())

	this.Regexp(`<td data-l="Compared with">\s*<div class="badges">\s*</div>\s*</td>`, body)
	this.NotContains(body, `nothing deployed shares an endpoint`)
}

func (this *IntegrationSuite) TestShowContractVersion_PaginatesTheChecks() {
	this.publishForContractVersion()
	for range 26 {
		this.checkForContractVersion("version_web", "release/2.0", "production")
	}
	cookie := this.signInAsAdmin()

	_, _, first := this.getPage(contractVersionPath, cookie)
	this.Len(contractVersionCheckRow.FindAllString(first, -1), 25)
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `<a href="`+contractVersionPath+`?checks_page=2" rel="next">Next</a>`)
	this.Contains(first, `<a href="/checks/26">#26</a>`)

	status, _, second := this.getPage(contractVersionPath+"?checks_page=2", cookie)
	this.Require().Equal(http.StatusOK, status)
	this.Len(contractVersionCheckRow.FindAllString(second, -1), 1)
	this.Contains(second, `showing 26–26 of 26`)
	this.Contains(second, `<a href="/checks/1">#1</a>`)

	for _, query := range []string{"?checks_page=3", "?checks_page=0", "?checks_page=01", "?checks_page=+1", "?checks_page=", "?checks_page=x"} {
		status, _, _ := this.getPage(contractVersionPath+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

const contractVersionDeploymentRow = `<td data-l="Deployed">`

func (this *IntegrationSuite) insertDeploymentsForContractVersion(count int) {
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO deployments (participant_id, version, environment_id, rollback, deployed_at)
		SELECT p.id, 'release/2.0', e.id, false, '2026-01-01T00:00:00Z'::timestamptz + n * interval '1 minute'
		FROM participants p CROSS JOIN environments e CROSS JOIN generate_series(1, $1) n
		WHERE p.name = 'version_web' AND e.name = 'production'`, count)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) insertChecksForContractVersion(count int) {
	_, err := this.Pool.Exec(context.Background(), `
		INSERT INTO compatibility_checks (participant_id, contract_id, version, environment_id, deployable, created_at)
		SELECT cv.participant_id, cv.contract_id, cv.version, e.id, true, '2026-01-01T00:00:00Z'::timestamptz + n * interval '1 minute'
		FROM contract_versions cv JOIN participants p ON p.id = cv.participant_id CROSS JOIN environments e CROSS JOIN generate_series(1, $1) n
		WHERE p.name = 'version_web' AND cv.version = 'release/2.0' AND e.name = 'production'`, count)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) TestShowContractVersion_PaginatesTheDeploymentHistory() {
	this.publishForContractVersion()
	this.insertDeploymentsForContractVersion(26)
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage(contractVersionPath, cookie)
	this.Require().Equal(http.StatusOK, status, first)
	this.Equal(25, strings.Count(first, contractVersionDeploymentRow))
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `<a href="`+contractVersionPath+`?deployments_page=2" rel="next">Next</a>`)
	this.Contains(first, `datetime="2026-01-01T00:26:00Z"`)
	this.NotContains(first, `datetime="2026-01-01T00:01:00Z"`)

	status, _, second := this.getPage(contractVersionPath+"?deployments_page=2", cookie)
	this.Require().Equal(http.StatusOK, status, second)
	this.Equal(1, strings.Count(second, contractVersionDeploymentRow))
	this.Contains(second, `showing 26–26 of 26`)
	this.Contains(second, `datetime="2026-01-01T00:01:00Z"`)
	this.Contains(second, `<a href="`+contractVersionPath+`?deployments_page=1" rel="prev">Previous</a>`)

	for _, query := range []string{"?deployments_page=3", "?deployments_page=0", "?deployments_page=01", "?deployments_page=+1", "?deployments_page=", "?deployments_page=x"} {
		status, _, _ := this.getPage(contractVersionPath+query, cookie)
		this.Equal(http.StatusNotFound, status, query)
	}
}

func (this *IntegrationSuite) TestShowContractVersion_PaginatesDeploymentsAndChecksIndependently() {
	this.publishForContractVersion()
	this.insertDeploymentsForContractVersion(26)
	this.insertChecksForContractVersion(26)
	cookie := this.signInAsAdmin()

	status, _, deploymentsSecond := this.getPage(contractVersionPath+"?deployments_page=2", cookie)
	this.Require().Equal(http.StatusOK, status, deploymentsSecond)
	this.Equal(1, strings.Count(deploymentsSecond, contractVersionDeploymentRow))
	this.Len(contractVersionCheckRow.FindAllString(deploymentsSecond, -1), 25)
	this.Contains(deploymentsSecond, `<a href="`+contractVersionPath+`?checks_page=2&amp;deployments_page=2" rel="next">Next</a>`)

	status, _, bothSecond := this.getPage(contractVersionPath+"?checks_page=2&deployments_page=2", cookie)
	this.Require().Equal(http.StatusOK, status, bothSecond)
	this.Equal(1, strings.Count(bothSecond, contractVersionDeploymentRow))
	this.Len(contractVersionCheckRow.FindAllString(bothSecond, -1), 1)
	this.Contains(bothSecond, `<a href="`+contractVersionPath+`?checks_page=2&amp;deployments_page=1" rel="prev">Previous</a>`)
	this.Contains(bothSecond, `<a href="`+contractVersionPath+`?checks_page=1&amp;deployments_page=2" rel="prev">Previous</a>`)
}

func (this *IntegrationSuite) TestShowContractVersion_UnknownOrUnreadableVersionIsNotFound() {
	this.publishForContractVersion()
	cookie := this.signInAsAdmin()

	for _, path := range []string{
		"/participants/version_web/versions/9.9.9",
		"/participants/version_web/versions/release",
		"/participants/version_api/versions/release%2F2.0",
		"/participants/ghost/versions/1.0.0",
		"/participants/version_web/versions/%00",
		"/participants/version_web/versions/release%2F2.0%00",
		"/participants/version_web/versions/%FF",
	} {
		status, header, body := this.getPage(path, cookie)
		this.Equal(http.StatusNotFound, status, path)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), path)
		this.Contains(body, "Not Found", path)
	}
}

func (this *IntegrationSuite) TestShowContractVersion_ReadUserSeesThePageWithoutWriteActions() {
	this.publishForContractVersion()
	this.checkForContractVersion("version_web", "release/2.0", "production")
	this.createActiveUser(contractVersionReaderEmail, model.PermissionRead, contractVersionReaderPassword)

	status, _, body := this.getPage(contractVersionPath, this.signInAsUser(contractVersionReaderEmail, contractVersionReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="path">`+contractVersionSchemasSource+`</span>`)
	this.Equal(1, strings.Count(body, `method="post"`))
	this.Contains(body, `<form method="post" action="/sign-out">`)
}

func (this *IntegrationSuite) TestShowContractVersion_SignedOutIsRedirectedToSignIn() {
	this.publishForContractVersion()

	status, header, _ := this.getPage(contractVersionPath, "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fparticipants%2Fversion_web%2Fversions%2Frelease%252F2.0", header.Get("Location"))
}
