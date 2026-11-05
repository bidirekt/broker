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
	this.Contains(body, `<p class="sub">Published <time datetime="`)
	this.Regexp(`<p class="sub">Published <time datetime="[^"]+Z">\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC · just now</time></p>`, body)
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

	this.Regexp(`(?s)<h2 class="h2">Deployed now</h2>.*?<li>`+production+`<span class="muted">since <time datetime="[^"]+">[^<]+ UTC · just now</time></span></li>\s*<li>`+staging, body)

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
	this.Regexp(`<li>`+productionBadge+`<span class="badge no">.*?no · `+productionBreaks+`</span><a href="/checks/`+production+`">check #`+production+`</a><span class="muted"><time datetime="[^"]+">[^<]+ UTC · just now</time></span></li>`, latest)
	this.Regexp(`<li>`+stagingBadge+`<span class="badge no">.*?no · `+stagingBreaks+`</span><a href="/checks/`+staging+`">check #`+staging+`</a>`, latest)
	this.Less(strings.Index(latest, `production`), strings.Index(latest, `staging`))

	all := body[strings.Index(body, `All checks`):]
	this.Len(contractVersionCheckRow.FindAllString(all, -1), 3)
	this.Less(strings.Index(all, `#`+staging+`</a>`), strings.Index(all, `#`+production+`</a>`))
	this.Contains(all, `<span class="badge cpb no">version_api <span class="v">1.0.0</span> · `+productionBreaks+`</span>`)
	this.Contains(all, `<span class="badge cpb no">version_api <span class="v">no version</span> · `+stagingBreaks+`</span>`)
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
	this.Contains(body, `<span class="badge cpb">version_api <span class="v">1.0.0</span></span>`)
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
	this.Equal("/sign-in", header.Get("Location"))
}
