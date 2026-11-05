package integration_test

import (
	"context"
	"net/http"
	"net/url"

	"github.com/bidirekt/broker/internal/model"
)

const (
	createParticipantUiPath           = "/participants/new-participant"
	createParticipantUiReaderEmail    = "reader@x.com"
	createParticipantUiReaderPassword = "reader-correct-horse-battery"
	createParticipantUiForbidden      = "<h1>you don&#39;t have access</h1>"
)

func (this *IntegrationSuite) createParticipantUiSubmit(sessionCookie, name string) (status int, header http.Header, body string) {
	_, _, form := this.getPage("/participants", sessionCookie)

	return this.postForm(createParticipantUiPath, sessionCookie, this.csrfTokenFrom(form), url.Values{"name": {name}})
}

func (this *IntegrationSuite) createParticipantUiCount(name string) int {
	var count int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM participants WHERE name = $1`, name).Scan(&count)
	this.Require().NoError(err)

	return count
}

func (this *IntegrationSuite) createParticipantUiCountAfterApiCreate(name string) int {
	status, response := this.post("/api/participants", `{"participant":"`+name+`"}`)
	this.Require().Equal(http.StatusOK, status, response)

	return this.createParticipantUiCount(name)
}

func (this *IntegrationSuite) TestCreateParticipantUi_ShowsTheFormBeforeTheParticipantRoute() {
	status, header, body := this.getPage(createParticipantUiPath, this.signInAsUser(suiteWriterEmail, suiteWriterPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1>Create participant</h1>`)
	this.Contains(body, `<form class="sheet formcard" method="post" action="/participants/new-participant">`)
	this.Contains(body, `<input class="input" id="name" name="name" type="text" autocomplete="off" value="" required autofocus>`)
	this.Contains(body, `<button type="submit" class="btn primary">Create participant</button>`)
	this.Contains(body, `<code class="cmd">bidirekt create-participant</code>`)
	this.Contains(body, `<a href="/participants" aria-current="page">Participants</a>`)
	this.NotContains(body, `class="err"`)
}

func (this *IntegrationSuite) TestCreateParticipantUi_CreatesAndRedirectsWithTheMessage() {
	cookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)

	status, header, _ := this.createParticipantUiSubmit(cookie, "petstore_admin")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/participants/petstore_admin", header.Get("Location"))
	this.Equal(1, this.createParticipantUiCount("petstore_admin"))

	status, _, body := this.getPage("/participants/petstore_admin", cookie)
	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<div class="flash" role="status">`)
	this.Contains(body, `<span>participant created</span>`)

	_, _, reloaded := this.getPage("/participants/petstore_admin", cookie)
	this.NotContains(reloaded, `participant created`)
}

func (this *IntegrationSuite) TestCreateParticipantUi_FieldErrorsKeepTheTypedValue() {
	this.Require().Equal(1, this.createParticipantUiCountAfterApiCreate("petstore_api"))
	cookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)

	cases := []struct {
		name    string
		status  int
		message string
	}{
		{"Petstore-API", http.StatusBadRequest, "participant name must be snake_case"},
		{"petstore_api", http.StatusConflict, "participant already exists"},
		{"", http.StatusBadRequest, "participant invalid input"},
	}
	for _, testCase := range cases {
		status, header, body := this.createParticipantUiSubmit(cookie, testCase.name)

		this.Equal(testCase.status, status, testCase.name)
		this.Equal("text/html; charset=utf-8", header.Get("Content-Type"), testCase.name)
		this.Equal("no-store", header.Get("Cache-Control"), testCase.name)
		this.Contains(body, `value="`+testCase.name+`" required autofocus aria-invalid="true" aria-describedby="name-err">`, testCase.name)
		this.Contains(body, `<p class="err" id="name-err">`, testCase.name)
		this.Contains(body, `<span>`+testCase.message+`</span>`, testCase.name)
	}

	this.Equal(1, this.countRows("participants"))
}

func (this *IntegrationSuite) TestCreateParticipantUi_ReadUserIsForbidden() {
	this.createActiveUser(createParticipantUiReaderEmail, model.PermissionRead, createParticipantUiReaderPassword)
	cookie := this.signInAsUser(createParticipantUiReaderEmail, createParticipantUiReaderPassword)

	status, header, body := this.getPage(createParticipantUiPath, cookie)
	this.Equal(http.StatusForbidden, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, createParticipantUiForbidden)
	this.NotContains(body, `<h1>Create participant</h1>`)

	status, _, body = this.createParticipantUiSubmit(cookie, "petstore_admin")
	this.Equal(http.StatusForbidden, status)
	this.Contains(body, createParticipantUiForbidden)
	this.Equal(0, this.createParticipantUiCount("petstore_admin"))
}

func (this *IntegrationSuite) TestCreateParticipantUi_SignedOutIsRedirectedToSignIn() {
	status, header, _ := this.getPage(createParticipantUiPath, "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestCreateParticipantUi_ParticipantsListOffersCreateOnlyToWriters() {
	status, _, writerList := this.getPage("/participants", this.signInAsUser(suiteWriterEmail, suiteWriterPassword))
	this.Require().Equal(http.StatusOK, status)
	this.Contains(writerList, `<div class="acts"><a class="btn primary" href="/participants/new-participant">Create participant</a></div>`)
	this.Contains(writerList, `Create one here, or from CI:`)

	this.createActiveUser(createParticipantUiReaderEmail, model.PermissionRead, createParticipantUiReaderPassword)
	_, _, readerList := this.getPage("/participants", this.signInAsUser(createParticipantUiReaderEmail, createParticipantUiReaderPassword))
	this.NotContains(readerList, `href="/participants/new-participant"`)
	this.Contains(readerList, `Create one from CI:`)
}
