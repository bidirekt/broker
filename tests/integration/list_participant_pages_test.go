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
	listParticipantPagesReaderEmail    = "reader@x.com"
	listParticipantPagesReaderPassword = "reader-correct-horse-battery"
)

const listParticipantPagesContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

var listParticipantPagesRowName = regexp.MustCompile(`<a class="name" href="/participants/([a-z0-9_]+)">`)

func (this *IntegrationSuite) mustPostForListParticipantPages(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForListParticipantPages(participant, version string) {
	this.mustPostForListParticipantPages("/api/contracts",
		this.publishBody(participant, version, contractFragment{"contract.yaml", listParticipantPagesContract}))
}

func (this *IntegrationSuite) deployForListParticipantPages(participant, version, environment string) {
	this.mustPostForListParticipantPages("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) createParticipantsForListParticipantPages(names ...string) {
	for _, name := range names {
		this.mustPostForListParticipantPages("/api/participants", `{"participant":"`+name+`"}`)
	}
}

func (this *IntegrationSuite) insertParticipantsForListParticipantPages(count int) {
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO participants (name) SELECT 'service_' || lpad(n::text, 2, '0') FROM generate_series(1, $1) n`, count)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) environmentPathForListParticipantPages(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func (this *IntegrationSuite) TestListParticipantPages_ShowsLatestVersionAndWhereEachIsDeployed() {
	for _, environment := range []string{"staging", "production"} {
		this.mustPostForListParticipantPages("/api/environments", `{"environment":"`+environment+`"}`)
	}
	this.createParticipantsForListParticipantPages("petstore_web", "petstore_api", "petstore_admin", "petstore_reviews")
	this.publishForListParticipantPages("petstore_api", "1.4.0")
	this.deployForListParticipantPages("petstore_api", "1.4.0", "production")
	this.publishForListParticipantPages("petstore_api", "1.5.0")
	this.deployForListParticipantPages("petstore_api", "1.5.0", "staging")
	this.publishForListParticipantPages("petstore_web", "2.3.0")
	this.deployForListParticipantPages("petstore_web", "2.3.0", "staging")
	this.publishForListParticipantPages("petstore_reviews", "1.0.0")
	production := this.environmentPathForListParticipantPages("production")
	staging := this.environmentPathForListParticipantPages("staging")

	status, header, body := this.getPage("/participants", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1>Participants</h1>`)
	this.Contains(body, `<a href="/participants" aria-current="page">Participants</a>`)

	rows := strings.Split(body, "<tr>")[2:]
	this.Require().Len(rows, 4)

	this.Contains(rows[0], `<a class="name" href="/participants/petstore_admin">petstore_admin</a>`)
	this.Equal(3, strings.Count(rows[0], `<span class="muted">never published</span>`))
	this.NotContains(rows[0], `not deployed`)
	this.NotContains(rows[0], `<time`)

	this.Contains(rows[1], `<a class="name" href="/participants/petstore_api">petstore_api</a>`)
	this.Contains(rows[1], `<span class="ver">1.5.0</span>`)
	this.Contains(rows[1], `just now</time>`)
	this.Contains(rows[1], `<div class="badges"><a class="badge env" href="`+production+`">production <span class="v">1.4.0</span></a><a class="badge env" href="`+staging+`">staging <span class="v">1.5.0</span></a></div>`)

	this.Contains(rows[2], `<a class="name" href="/participants/petstore_reviews">petstore_reviews</a>`)
	this.Contains(rows[2], `<span class="ver">1.0.0</span>`)
	this.Contains(rows[2], `<span class="muted">not deployed</span>`)
	this.NotContains(rows[2], `never published`)

	this.Contains(rows[3], `<a class="name" href="/participants/petstore_web">petstore_web</a>`)
	this.Contains(rows[3], `<a class="badge env" href="`+staging+`">staging <span class="v">2.3.0</span></a>`)
	this.NotContains(rows[3], `production`)

	this.Contains(body, `showing 1–4 of 4`)
	this.NotContains(body, `class="badge env stg"`)
	this.NotContains(body, `bidirekt create-participant`)
}

func (this *IntegrationSuite) TestListParticipantPages_SortedByteWise() {
	this.createParticipantsForListParticipantPages("ordersapi", "orders_web", "orders2")

	_, _, body := this.getPage("/participants", this.signInAsAdmin())

	var names []string
	for _, match := range listParticipantPagesRowName.FindAllStringSubmatch(body, -1) {
		names = append(names, match[1])
	}
	this.Equal([]string{"orders2", "orders_web", "ordersapi"}, names)
}

func (this *IntegrationSuite) TestListParticipantPages_EmptyListShowsTheCliCommand() {
	status, _, body := this.getPage("/participants", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `No participants yet`)
	this.Contains(body, `<code class="cmd">bidirekt create-participant</code>`)
	this.NotContains(body, `<table`)
}

func (this *IntegrationSuite) TestListParticipantPages_PaginatesTwentyFiveAPage() {
	this.insertParticipantsForListParticipantPages(26)
	cookie := this.signInAsAdmin()

	_, _, first := this.getPage("/participants", cookie)
	this.Equal(25, len(listParticipantPagesRowName.FindAllString(first, -1)))
	this.Contains(first, `showing 1–25 of 26`)
	this.Contains(first, `<a href="/participants?page=2" rel="next">Next</a>`)

	status, _, second := this.getPage("/participants?page=2", cookie)
	this.Require().Equal(http.StatusOK, status)
	this.Equal([]string{`<a class="name" href="/participants/service_26">`}, listParticipantPagesRowName.FindAllString(second, -1))
	this.Contains(second, `showing 26–26 of 26`)
	this.Contains(second, `<a href="/participants?page=1" rel="prev">Previous</a>`)
}

func (this *IntegrationSuite) TestListParticipantPages_InvalidOrPastTheLastPageIsNotFound() {
	this.insertParticipantsForListParticipantPages(26)
	cookie := this.signInAsAdmin()

	for _, path := range []string{"/participants?page=3", "/participants?page=0", "/participants?page=01", "/participants?page=%2B1", "/participants?page=", "/participants?page=abc"} {
		status, header, body := this.getPage(path, cookie)
		this.Equal(http.StatusNotFound, status, path)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), path)
		this.Contains(body, "Not Found", path)
	}
}

func (this *IntegrationSuite) TestListParticipantPages_PageWhoseOffsetWouldOverflowIsNotFound() {
	status, header, body := this.getPage("/participants?page=400000000000000000", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "Not Found")
}

func (this *IntegrationSuite) TestListParticipantPages_EmptyListHasOnlyTheFirstPage() {
	status, _, _ := this.getPage("/participants?page=2", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestListParticipantPages_ReadUserSeesThePageWithoutWriteActions() {
	this.createParticipantsForListParticipantPages("petstore_api")
	this.createActiveUser(listParticipantPagesReaderEmail, model.PermissionRead, listParticipantPagesReaderPassword)

	status, _, body := this.getPage("/participants", this.signInAsUser(listParticipantPagesReaderEmail, listParticipantPagesReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<a class="name" href="/participants/petstore_api">petstore_api</a>`)
	this.NotContains(body, `href="/participants/new-participant"`)
	this.NotContains(body, `method="post" action="/participants`)
	this.NotContains(body, `href="/users"`)
}

func (this *IntegrationSuite) TestListParticipantPages_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage("/participants", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}
