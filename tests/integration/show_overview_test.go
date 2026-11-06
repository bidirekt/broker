package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	showOverviewReaderEmail    = "reader@x.com"
	showOverviewReaderPassword = "reader-correct-horse-battery"
)

const showOverviewAPIContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

const showOverviewWebCompatibleContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

const showOverviewGhostConsumerContract = `consumes:
  ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
`

func (this *IntegrationSuite) mustPostForShowOverview(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForShowOverview(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowOverview("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowOverview("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) publishForShowOverview(participant, version, content string) {
	this.mustPostForShowOverview("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForShowOverview(participant, version, environment string) {
	this.mustPostForShowOverview("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForShowOverview(participant, version, environment string) (checkID string) {
	this.mustPostForShowOverview("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)

	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&id))

	return strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) environmentIDForShowOverview(environment string) string {
	var environmentID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, environment).Scan(&environmentID))

	return strconv.FormatInt(environmentID, 10)
}

func (this *IntegrationSuite) sectionOfShowOverview(body, heading, nextHeading string) string {
	start := strings.Index(body, `<h2 class="h2">`+heading+`</h2>`)
	this.Require().NotEqual(-1, start, "no section %q", heading)
	section := body[start:]
	if nextHeading == "" {
		return section
	}

	end := strings.Index(section, `<h2 class="h2">`+nextHeading+`</h2>`)
	this.Require().NotEqual(-1, end, "no section %q", nextHeading)

	return section[:end]
}

func (this *IntegrationSuite) TestShowOverview_ShowsTheNumbersTheLastNoChecksWhatIsDeployedAndTheActivity() {
	this.createForShowOverview([]string{"production", "staging", "qa"}, []string{"petstore_api", "petstore_web", "petstore_admin"})
	this.publishForShowOverview("petstore_api", "1.0.0", showOverviewAPIContract)
	this.deployForShowOverview("petstore_api", "1.0.0", "production")
	this.deployForShowOverview("petstore_api", "1.0.0", "qa")
	this.publishForShowOverview("petstore_web", "2.0.0", showOverviewWebCompatibleContract)
	this.publishForShowOverview("petstore_web", "2.1.0", showOverviewGhostConsumerContract)

	stagingNo := this.checkForShowOverview("petstore_web", "2.1.0", "staging")
	this.deployForShowOverview("petstore_web", "2.1.0", "staging")
	productionYes := this.checkForShowOverview("petstore_web", "2.0.0", "production")
	productionNo := this.checkForShowOverview("petstore_web", "2.1.0", "production")
	qaNo := this.checkForShowOverview("petstore_web", "2.1.0", "qa")
	qaYes := this.checkForShowOverview("petstore_web", "2.0.0", "qa")
	lastMonth := this.checkForShowOverview("petstore_web", "2.0.0", "production")
	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_checks SET created_at = now() - interval '40 days' WHERE id = $1`, lastMonth)
	this.Require().NoError(err)

	status, header, body := this.getPage("/", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1>Overview</h1>`)
	this.Contains(body, `<p class="sub">What is deployed, what the last checks said, and what happened last.</p>`)

	this.Contains(body, `<div><b>3</b><span>participants · 1 never published</span></div>`)
	this.Contains(body, `<div><b>3</b><span>environments</span></div>`)
	this.Contains(body, `<div><b>3</b><span>versions published</span></div>`)
	this.Contains(body, `<div><b>5</b><span>checks this month</span></div>`)

	production := this.environmentIDForShowOverview("production")
	staging := this.environmentIDForShowOverview("staging")
	qa := this.environmentIDForShowOverview("qa")

	saidNo := this.sectionOfShowOverview(body, "Last check said no", "Deployed now")
	this.Equal(2, strings.Count(saidNo, `<div class="attn">`))
	productionRow := `<div class="what"><a class="name" href="/participants/petstore_web">petstore_web</a> <a class="ver" href="/participants/petstore_web/versions/2.1.0">2.1.0</a><br><b>cannot be deployed</b> to <a class="name" href="/environments/` + production + `">production</a></div>
      <a class="badge no" href="/checks/` + productionNo + `">`
	stagingRow := `<div class="what"><a class="name" href="/participants/petstore_web">petstore_web</a> <a class="ver" href="/participants/petstore_web/versions/2.1.0">2.1.0</a><br><b>cannot be deployed</b> to <a class="name" href="/environments/` + staging + `">staging</a> <span class="muted">· deployed there anyway</span></div>
      <a class="badge no" href="/checks/` + stagingNo + `">`
	this.Contains(saidNo, productionRow)
	this.Contains(saidNo, stagingRow)
	this.Less(strings.Index(saidNo, productionRow), strings.Index(saidNo, stagingRow))
	this.Contains(saidNo, `1 break · check #`+productionNo+`</a>`)
	this.NotContains(saidNo, `href="/checks/`+qaNo+`"`)
	this.NotContains(saidNo, `href="/environments/`+qa+`"`)
	this.NotContains(saidNo, `badge more`)

	deployed := this.sectionOfShowOverview(body, "Deployed now", "Recent activity")
	this.Contains(deployed, `<div class="scroll"><table class="t stack dep">`)
	this.Contains(deployed, `<th scope="col">Participant</th>
        <th scope="col"><a class="badge env" href="/environments/`+production+`">production</a></th><th scope="col"><a class="badge env" href="/environments/`+qa+`">qa</a></th><th scope="col"><a class="badge env" href="/environments/`+staging+`">staging</a></th>`)
	webRow := `<td data-l="Participant"><a class="name" href="/participants/petstore_web">petstore_web</a></td>
        <td data-l="production"><span class="muted">not deployed</span></td><td data-l="qa"><span class="muted">not deployed</span></td><td data-l="staging"><div class="v2"><a class="ver" href="/participants/petstore_web/versions/2.1.0">2.1.0</a><span class="time">`
	apiRow := `<td data-l="Participant"><a class="name" href="/participants/petstore_api">petstore_api</a></td>
        <td data-l="production"><div class="v2"><a class="ver" href="/participants/petstore_api/versions/1.0.0">1.0.0</a><span class="time">`
	apiNotInStaging := `<td data-l="staging"><span class="muted">not deployed</span></td>
      </tr>`
	this.Contains(deployed, webRow)
	this.Contains(deployed, apiRow)
	this.Contains(deployed, apiNotInStaging)
	this.Less(strings.Index(deployed, webRow), strings.Index(deployed, apiRow))
	this.NotContains(deployed, `petstore_admin`)
	this.Contains(deployed, `<span>2 of 3 participants are deployed · 1 never published</span>`)
	this.NotContains(deployed, `badge more`)

	activity := this.sectionOfShowOverview(body, "Recent activity", "")
	this.Equal(9, strings.Count(activity, "<li>"))
	order := []string{
		`<a class="badge yes" href="/checks/` + qaYes + `">`,
		`<a class="badge no" href="/checks/` + qaNo + `">`,
		`<a class="badge no" href="/checks/` + productionNo + `">`,
		`<a class="badge yes" href="/checks/` + productionYes + `">`,
		`<a class="name" href="/environments/` + staging + `">staging</a></div>
      <span class="ev"><span class="badges"><span class="badge">deployed</span>`,
		`<a class="badge no" href="/checks/` + stagingNo + `">`,
		`<a class="name" href="/environments/` + qa + `">qa</a></div>
      <span class="ev"><span class="badges"><span class="badge">deployed</span>`,
		`<a class="name" href="/environments/` + production + `">production</a></div>
      <span class="ev"><span class="badges"><span class="badge">deployed</span>`,
		`<a class="badge yes" href="/checks/` + lastMonth + `">`,
	}
	for index := 1; index < len(order); index++ {
		this.Require().Contains(activity, order[index])
		this.Less(strings.Index(activity, order[index-1]), strings.Index(activity, order[index]), order[index])
	}
	this.Contains(activity, `<a class="badge no" href="/checks/`+productionNo+`"><svg`)
	this.Contains(activity, `can-i-deploy no · 1 break</a>`)
	this.Contains(activity, `can-i-deploy yes</a>`)
	this.Contains(activity, `<div class="what"><a class="name" href="/participants/petstore_web">petstore_web</a> <a class="ver" href="/participants/petstore_web/versions/2.1.0">2.1.0</a> <span class="arrow" aria-hidden="true">→</span><span class="sr">to</span> <a class="name" href="/environments/`+qa+`">qa</a></div>`)
}

func (this *IntegrationSuite) TestShowOverview_PanelsShowTenRowsAndLinkTheRest() {
	this.createForShowOverview([]string{"production"}, nil)
	for letter := 'a'; letter <= 'k'; letter++ {
		name := "consumer_" + string(letter)
		this.createForShowOverview(nil, []string{name})
		this.publishForShowOverview(name, "1.0.0", showOverviewGhostConsumerContract)
		this.checkForShowOverview(name, "1.0.0", "production")
		this.deployForShowOverview(name, "1.0.0", "production")
	}

	_, _, body := this.getPage("/", this.signInAsAdmin())

	saidNo := this.sectionOfShowOverview(body, "Last check said no", "Deployed now")
	this.Equal(10, strings.Count(saidNo, `<div class="attn">`))
	this.Contains(saidNo, `<div class="sheet-foot"><a class="badge more" href="/environments">+1 more</a></div>`)
	this.Less(strings.Index(saidNo, `>consumer_k</a>`), strings.Index(saidNo, `>consumer_j</a>`))
	this.NotContains(saidNo, `>consumer_a</a>`)

	deployed := this.sectionOfShowOverview(body, "Deployed now", "Recent activity")
	this.Equal(10, strings.Count(deployed, `<td data-l="Participant">`))
	this.Contains(deployed, `<a class="badge more" href="/participants">+1 more</a>`)
	this.Contains(deployed, `<span>11 of 11 participants are deployed · 0 never published</span>`)
	this.Less(strings.Index(deployed, `>consumer_k</a>`), strings.Index(deployed, `>consumer_j</a>`))
	this.NotContains(deployed, `>consumer_a</a>`)

	activity := this.sectionOfShowOverview(body, "Recent activity", "")
	this.Equal(10, strings.Count(activity, "<li>"))
	this.NotContains(activity, "badge more")
}

func (this *IntegrationSuite) TestShowOverview_EmptyPanelsInABrokerWithData() {
	this.createForShowOverview([]string{"production"}, []string{"petstore_api"})

	status, _, body := this.getPage("/", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<p class="pad muted">No check said no.</p>`)
	this.Contains(body, `<p class="pad muted">Nothing deployed yet.</p>`)
	this.Contains(body, `<p class="pad muted">No activity yet.</p>`)
	this.Contains(body, `<div><b>1</b><span>participant · 1 never published</span></div>`)
	this.Contains(body, `<div><b>1</b><span>environment</span></div>`)
	this.Contains(body, `<div><b>0</b><span>versions published</span></div>`)
	this.Contains(body, `<div><b>0</b><span>checks this month</span></div>`)
	this.NotContains(body, `class="steps"`)
}

func (this *IntegrationSuite) TestShowOverview_EmptyBrokerShowsTheCliStepsInOrderAndTheCreateButtons() {
	status, header, body := this.getPage("/", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<p class="sub">This broker has no data yet.</p>`)
	this.Contains(body, `<div><b>0</b><span>participants · 0 never published</span></div>`)
	this.Contains(body, `<div><b>0</b><span>environments</span></div>`)

	steps := []string{
		`<code class="cmd">bidirekt create-participant</code>`,
		`<code class="cmd">bidirekt create-environment</code>`,
		`<code class="cmd">bidirekt publish</code>`,
		`<code class="cmd">bidirekt can-i-deploy</code>`,
		`<li><p>Deploy the version with your own pipeline.</p></li>`,
		`<code class="cmd">bidirekt record-deployment</code>`,
	}
	for index := 1; index < len(steps); index++ {
		this.Require().Contains(body, steps[index])
		this.Less(strings.Index(body, steps[index-1]), strings.Index(body, steps[index]), steps[index])
	}

	this.Contains(body, `<a class="btn" href="/participants/new-participant">Create participant</a>`)
	this.Contains(body, `<a class="btn" href="/environments/new-environment">Create environment</a>`)
	this.NotContains(body, `Last check said no`)
	this.NotContains(body, `Recent activity`)
}

func (this *IntegrationSuite) TestShowOverview_BrokerWithoutEnvironmentsIsEmptyAndReadSeesNoCreateButtons() {
	this.createForShowOverview(nil, []string{"petstore_api"})
	this.publishForShowOverview("petstore_api", "1.0.0", showOverviewAPIContract)
	this.createActiveUser(showOverviewReaderEmail, model.PermissionRead, showOverviewReaderPassword)

	status, _, body := this.getPage("/", this.signInAsUser(showOverviewReaderEmail, showOverviewReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<p class="sub">This broker has no data yet.</p>`)
	this.Contains(body, `<div><b>1</b><span>participant · 0 never published</span></div>`)
	this.Contains(body, `<div><b>1</b><span>version published</span></div>`)
	this.Contains(body, `<code class="cmd">bidirekt create-environment</code>`)
	this.NotContains(body, `href="/participants/new-participant"`)
	this.NotContains(body, `href="/environments/new-environment"`)
	this.NotContains(body, `Prefer the UI for setup?`)
}

func (this *IntegrationSuite) TestShowOverview_BrokerWithoutParticipantsIsEmpty() {
	this.createForShowOverview([]string{"production"}, nil)

	_, _, body := this.getPage("/", this.signInAsAdmin())

	this.Contains(body, `<p class="sub">This broker has no data yet.</p>`)
	this.Contains(body, `<div><b>1</b><span>environment</span></div>`)
	this.Contains(body, `class="steps"`)
}

func (this *IntegrationSuite) TestShowOverview_ReadUserSeesThePanelsWithoutWriteActions() {
	this.createForShowOverview([]string{"production"}, []string{"petstore_api"})
	this.publishForShowOverview("petstore_api", "1.0.0", showOverviewAPIContract)
	this.deployForShowOverview("petstore_api", "1.0.0", "production")
	this.createActiveUser(showOverviewReaderEmail, model.PermissionRead, showOverviewReaderPassword)

	status, _, body := this.getPage("/", this.signInAsUser(showOverviewReaderEmail, showOverviewReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h2 class="h2">Deployed now</h2>`)
	this.Contains(body, `<a class="ver" href="/participants/petstore_api/versions/1.0.0">1.0.0</a>`)
	this.NotContains(body, `new-participant`)
	this.NotContains(body, `new-environment`)
	this.NotContains(body, `/rename"`)
}

func (this *IntegrationSuite) TestShowOverview_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage("/", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestShowOverview_IndexesExist() {
	indexes := []struct{ table, definition string }{
		{"compatibility_checks", "%USING btree (participant_id, environment_id, created_at DESC, id DESC)"},
		{"compatibility_checks", "%USING btree (created_at DESC)"},
		{"deployments", "%USING btree (deployed_at DESC)"},
	}
	for _, index := range indexes {
		var count int
		this.Require().NoError(this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_indexes WHERE tablename = $1 AND indexdef LIKE $2`, index.table, index.definition,
		).Scan(&count))
		this.Equal(1, count, index.definition)
	}
}
