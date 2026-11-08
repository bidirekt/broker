package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bidirekt/broker/internal/model"
)

const (
	createEnvironmentUiPath           = "/environments/new-environment"
	createEnvironmentUiReaderEmail    = "reader@x.com"
	createEnvironmentUiReaderPassword = "reader-correct-horse-battery"
	createEnvironmentUiForbidden      = "<h1>you don&#39;t have access</h1>"
)

func (this *IntegrationSuite) createEnvironmentUiSubmit(sessionCookie, name string) (status int, header http.Header, body string) {
	_, _, form := this.getPage("/environments", sessionCookie)

	return this.postForm(createEnvironmentUiPath, sessionCookie, this.csrfTokenFrom(form), url.Values{"name": {name}})
}

func (this *IntegrationSuite) createEnvironmentUiCount(name string) int {
	var count int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM environments WHERE name = $1`, name).Scan(&count)
	this.Require().NoError(err)

	return count
}

func (this *IntegrationSuite) createEnvironmentUiID(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id)
	this.Require().NoError(err)

	return id
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_ShowsTheFormBeforeTheEnvironmentRoute() {
	status, header, body := this.getPage(createEnvironmentUiPath, this.signInAsUser(suiteWriterEmail, suiteWriterPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1>Create environment</h1>`)
	this.Contains(body, `<form class="sheet formcard" method="post" action="/environments/new-environment">`)
	this.Contains(body, `<input class="input" id="name" name="name" type="text" autocomplete="off" value="" required autofocus>`)
	this.Contains(body, `<button type="submit" class="btn primary">Create environment</button>`)
	this.Contains(body, `<code class="cmd">bidirekt create-environment</code>`)
	this.Contains(body, `<a href="/environments" aria-current="page">Environments</a>`)
	this.NotContains(body, `class="err"`)
	this.NotContains(body, "Not Found")
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_CreatesAndRedirectsToTheEnvironmentWithTheMessage() {
	cookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)

	status, header, _ := this.createEnvironmentUiSubmit(cookie, "qa")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Require().Equal(1, this.createEnvironmentUiCount("qa"))
	environmentPath := "/environments/" + strconv.FormatInt(this.createEnvironmentUiID("qa"), 10)
	this.Equal(environmentPath, header.Get("Location"))

	status, _, body := this.getPage(environmentPath, cookie)
	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h1 class="name">qa</h1>`)
	this.Contains(body, `<div class="flash" role="status">`)
	this.Contains(body, `<span>environment created</span>`)

	_, _, reloaded := this.getPage(environmentPath, cookie)
	this.NotContains(reloaded, `environment created`)
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_FieldErrorsKeepTheTypedValue() {
	status, response := this.post("/api/environments", `{"environment":"staging"}`)
	this.Require().Equal(http.StatusOK, status, response)
	cookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)

	cases := []struct {
		name    string
		status  int
		message string
	}{
		{"staging", http.StatusConflict, "environment already exists"},
		{"", http.StatusBadRequest, "environment invalid input"},
		{" qa", http.StatusBadRequest, "environment invalid input"},
		{"qa ", http.StatusBadRequest, "environment invalid input"},
	}
	for _, testCase := range cases {
		status, header, body := this.createEnvironmentUiSubmit(cookie, testCase.name)

		this.Equal(testCase.status, status, testCase.name)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), testCase.name)
		this.Equal("no-store", header.Get("Cache-Control"), testCase.name)
		this.Contains(body, `value="`+testCase.name+`" required autofocus aria-invalid="true" aria-describedby="name-err">`, testCase.name)
		this.Contains(body, `<p class="err" id="name-err">`, testCase.name)
		this.Contains(body, `<span>`+testCase.message+`</span>`, testCase.name)
	}

	this.Equal(1, this.countRows("environments"))
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_ReadUserIsForbidden() {
	this.createActiveUser(createEnvironmentUiReaderEmail, model.PermissionRead, createEnvironmentUiReaderPassword)
	cookie := this.signInAsUser(createEnvironmentUiReaderEmail, createEnvironmentUiReaderPassword)

	status, header, body := this.getPage(createEnvironmentUiPath, cookie)
	this.Equal(http.StatusForbidden, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, createEnvironmentUiForbidden)
	this.NotContains(body, `<h1>Create environment</h1>`)

	status, _, body = this.createEnvironmentUiSubmit(cookie, "qa")
	this.Equal(http.StatusForbidden, status)
	this.Contains(body, createEnvironmentUiForbidden)
	this.Equal(0, this.createEnvironmentUiCount("qa"))
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage(createEnvironmentUiPath, "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fenvironments%2Fnew-environment", header.Get("Location"))
}

func (this *IntegrationSuite) TestCreateEnvironmentUi_EnvironmentsListOffersCreateOnlyToWriters() {
	status, _, writerList := this.getPage("/environments", this.signInAsUser(suiteWriterEmail, suiteWriterPassword))
	this.Require().Equal(http.StatusOK, status)
	this.Contains(writerList, `<div class="acts"><a class="btn primary" href="/environments/new-environment">Create environment</a></div>`)
	this.Contains(writerList, `<code>production</code>, here, or from CI:`)

	this.createActiveUser(createEnvironmentUiReaderEmail, model.PermissionRead, createEnvironmentUiReaderPassword)
	_, _, readerList := this.getPage("/environments", this.signInAsUser(createEnvironmentUiReaderEmail, createEnvironmentUiReaderPassword))
	this.NotContains(readerList, `href="/environments/new-environment"`)
	this.Contains(readerList, `<code>production</code>, from CI:`)
}
