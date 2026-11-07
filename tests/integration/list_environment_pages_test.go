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
	listEnvironmentPagesReaderEmail    = "reader@x.com"
	listEnvironmentPagesReaderPassword = "reader-correct-horse-battery"
)

const listEnvironmentPagesContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

var listEnvironmentPagesRowName = regexp.MustCompile(`<a class="name" href="/environments/[0-9]+">([a-z0-9_]+)</a>`)

func (this *IntegrationSuite) mustPostForListEnvironmentPages(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createEnvironmentsForListEnvironmentPages(names ...string) {
	for _, name := range names {
		this.mustPostForListEnvironmentPages("/api/environments", `{"environment":"`+name+`"}`)
	}
}

func (this *IntegrationSuite) publishForListEnvironmentPages(participant, version string) {
	this.mustPostForListEnvironmentPages("/api/participants", `{"participant":"`+participant+`"}`)
	this.mustPostForListEnvironmentPages("/api/contracts",
		this.publishBody(participant, version, contractFragment{"contract.yaml", listEnvironmentPagesContract}))
}

func (this *IntegrationSuite) deployForListEnvironmentPages(participant, version, environment string) {
	this.mustPostForListEnvironmentPages("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) environmentIDForListEnvironmentPages(name string) string {
	var id int64
	err := this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id)
	this.Require().NoError(err)
	return strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) insertEnvironmentsForListEnvironmentPages(count int) {
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO environments (name) SELECT 'env_' || lpad(n::text, 2, '0') FROM generate_series(1, $1) n`, count)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) TestListEnvironmentPages_ShowsDeployedCountAndLastDeployment() {
	this.createEnvironmentsForListEnvironmentPages("staging", "production", "qa")
	this.publishForListEnvironmentPages("petstore_api", "1.4.0")
	this.publishForListEnvironmentPages("petstore_web", "2.3.0")
	this.deployForListEnvironmentPages("petstore_api", "1.4.0", "production")
	this.deployForListEnvironmentPages("petstore_api", "1.4.0", "staging")
	this.deployForListEnvironmentPages("petstore_web", "2.3.0", "staging")
	this.mustPostForListEnvironmentPages("/api/contracts",
		this.publishBody("petstore_api", "1.5.0", contractFragment{"contract.yaml", listEnvironmentPagesContract}))
	this.deployForListEnvironmentPages("petstore_api", "1.5.0", "staging")

	status, header, body := this.getPage("/environments", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1>Environments</h1>`)
	this.Contains(body, `<a href="/environments" aria-current="page">Environments</a>`)
	this.Contains(body, `<a href="/participants">Participants</a>`)

	rows := strings.Split(body, "<tr>")[2:]
	this.Require().Len(rows, 3)

	this.Contains(rows[0], `<a class="name" href="/environments/`+this.environmentIDForListEnvironmentPages("production")+`">production</a>`)
	this.Contains(rows[0], `<td data-l="Participants deployed">1</td>`)
	this.Contains(rows[0], `<a class="name" href="/participants/petstore_api">petstore_api</a> <a class="ver" href="/participants/petstore_api/versions/1.4.0">1.4.0</a>`)
	this.Contains(rows[0], `just now</time>`)

	this.Contains(rows[1], `<a class="name" href="/environments/`+this.environmentIDForListEnvironmentPages("qa")+`">qa</a>`)
	this.Contains(rows[1], `<td data-l="Participants deployed">0</td>`)
	this.Contains(rows[1], `<td data-l="Last deployment"><span class="muted">not deployed</span></td>`)
	this.NotContains(rows[1], `<time`)

	this.Contains(rows[2], `<a class="name" href="/environments/`+this.environmentIDForListEnvironmentPages("staging")+`">staging</a>`)
	this.Contains(rows[2], `<td data-l="Participants deployed">2</td>`)
	this.Contains(rows[2], `<a class="name" href="/participants/petstore_api">petstore_api</a> <a class="ver" href="/participants/petstore_api/versions/1.5.0">1.5.0</a>`)
	this.NotContains(rows[2], `petstore_web`)

	this.Contains(body, `showing 1–3 of 3`)
	this.NotContains(body, `bidirekt create-environment`)
}

func (this *IntegrationSuite) TestListEnvironmentPages_SortedByteWise() {
	this.createEnvironmentsForListEnvironmentPages("stagingeu", "staging_us", "staging2")

	_, _, body := this.getPage("/environments", this.signInAsAdmin())

	var names []string
	for _, match := range listEnvironmentPagesRowName.FindAllStringSubmatch(body, -1) {
		names = append(names, match[1])
	}
	this.Equal([]string{"staging2", "staging_us", "stagingeu"}, names)
}

func (this *IntegrationSuite) TestListEnvironmentPages_EmptyListShowsTheCliCommand() {
	status, _, body := this.getPage("/environments", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `No environments yet`)
	this.Contains(body, `<code class="cmd">bidirekt create-environment</code>`)
	this.NotContains(body, `<table`)
}

func (this *IntegrationSuite) TestListEnvironmentPages_PaginatesTwentyFiveAPage() {
	this.insertEnvironmentsForListEnvironmentPages(26)
	cookie := this.signInAsAdmin()

	_, _, first := this.getPage("/environments", cookie)
	this.Len(listEnvironmentPagesRowName.FindAllString(first, -1), 25)
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `<a href="/environments?page=2" rel="next">Next</a>`)

	status, _, second := this.getPage("/environments?page=2", cookie)
	this.Require().Equal(http.StatusOK, status)
	var names []string
	for _, match := range listEnvironmentPagesRowName.FindAllStringSubmatch(second, -1) {
		names = append(names, match[1])
	}
	this.Equal([]string{"env_26"}, names)
	this.Contains(second, `showing 26–26 of 26`)
	this.Contains(second, `<a href="/environments?page=1" rel="prev">Previous</a>`)
}

func (this *IntegrationSuite) TestListEnvironmentPages_InvalidOrPastTheLastPageIsNotFound() {
	this.insertEnvironmentsForListEnvironmentPages(26)
	cookie := this.signInAsAdmin()

	for _, path := range []string{"/environments?page=3", "/environments?page=0", "/environments?page=01", "/environments?page=%2B1", "/environments?page=", "/environments?page=abc"} {
		status, header, body := this.getPage(path, cookie)
		this.Equal(http.StatusNotFound, status, path)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), path)
		this.Contains(body, "Not Found", path)
	}
}

func (this *IntegrationSuite) TestListEnvironmentPages_EmptyListHasOnlyTheFirstPage() {
	status, _, _ := this.getPage("/environments?page=2", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestListEnvironmentPages_ReadUserSeesThePageWithoutWriteActions() {
	this.createEnvironmentsForListEnvironmentPages("staging")
	this.createActiveUser(listEnvironmentPagesReaderEmail, model.PermissionRead, listEnvironmentPagesReaderPassword)

	status, _, body := this.getPage("/environments", this.signInAsUser(listEnvironmentPagesReaderEmail, listEnvironmentPagesReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `">staging</a>`)
	this.NotContains(body, `href="/environments/new-environment"`)
	this.NotContains(body, `method="post" action="/environments`)
	this.NotContains(body, `href="/users"`)
}

func (this *IntegrationSuite) TestListEnvironmentPages_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage("/environments", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestListEnvironmentPages_DeploymentsAreIndexedByEnvironmentNewestFirst() {
	var count int
	err := this.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'deployments' AND indexdef LIKE '%(environment_id, deployed_at DESC)'`,
	).Scan(&count)

	this.Require().NoError(err)
	this.Equal(1, count)
}

func (this *IntegrationSuite) TestListEnvironmentPages_LastDeploymentLinksArePathEscaped() {
	this.createEnvironmentsForListEnvironmentPages("production")
	this.publishForListEnvironmentPages("orders_web", "1.0.0+build")
	this.deployForListEnvironmentPages("orders_web", "1.0.0+build", "production")

	status, _, body := this.getPage("/environments", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<a class="name" href="/participants/orders_web">orders_web</a> <a class="ver" href="/participants/orders_web/versions/1.0.0&#43;build">1.0.0&#43;build</a>`)
}
