package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	renameEnvironmentUiReaderEmail     = "rename-environment-reader@x.com"
	renameEnvironmentUiReaderPassword  = "rename-environment-reader-correct-horse"
	renameEnvironmentUiInvalidInput    = "environment invalid input"
	renameEnvironmentUiAlreadyExists   = "environment already exists"
	renameEnvironmentUiRenamed         = "environment renamed"
	renameEnvironmentUiNoAccess        = "<h1>you don&#39;t have access</h1>"
	renameEnvironmentUiNotFound        = "<h1>Not Found</h1>"
	renameEnvironmentUiNameFieldError  = `<p class="err" id="name-err"><svg`
	renameEnvironmentUiNameInvalidMark = `aria-invalid="true" aria-describedby="name-err"`
	renameEnvironmentUiWarnIcon        = `<svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M8 1.8l6.5 11.7h-13z"/><path d="M8 6.5v3M8 11.6v.01"/></svg>`
)

func (this *IntegrationSuite) renameEnvironmentUiCreate(name string) (id int64) {
	status, response := this.post("/api/environments", `{"environment":"`+name+`"}`)
	this.Require().Equal(http.StatusOK, status, response)

	environment, found := repository.NewEnvironmentRepository(this.Pool).FindByName(context.Background(), name)
	this.Require().True(found)

	return environment.ID
}

func (this *IntegrationSuite) renameEnvironmentUiWriter() (sessionCookie, csrfToken string) {
	sessionCookie = this.signInAsUser(suiteWriterEmail, suiteWriterPassword)
	_, _, page := this.getPage("/environments", sessionCookie)

	return sessionCookie, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) renameEnvironmentUiReader() (sessionCookie, csrfToken string) {
	this.createActiveUser(renameEnvironmentUiReaderEmail, model.PermissionRead, renameEnvironmentUiReaderPassword)
	sessionCookie = this.signInAsUser(renameEnvironmentUiReaderEmail, renameEnvironmentUiReaderPassword)
	_, _, page := this.getPage("/environments", sessionCookie)

	return sessionCookie, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) renameEnvironmentUiPost(id int64, newName string) (status int, header http.Header, body string) {
	sessionCookie, csrfToken := this.renameEnvironmentUiWriter()

	return this.postForm(renameEnvironmentUiPath(id), sessionCookie, csrfToken, url.Values{"name": {newName}})
}

func (this *IntegrationSuite) renameEnvironmentUiNameOf(id int64) string {
	environment, found := repository.NewEnvironmentRepository(this.Pool).FindByID(context.Background(), id)
	this.Require().True(found)

	return environment.Name
}

func renameEnvironmentUiPath(id int64) string {
	return "/environments/" + strconv.FormatInt(id, 10) + "/rename"
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_FormIsPrefilledWithTheCurrentNameAndWarnsTheCI() {
	productionID := this.renameEnvironmentUiCreate("production")
	productionPath := "/environments/" + strconv.FormatInt(productionID, 10)
	sessionCookie, _ := this.renameEnvironmentUiWriter()

	status, header, body := this.getPage(renameEnvironmentUiPath(productionID), sessionCookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<a href="`+productionPath+`">production</a>`)
	this.Contains(body, "<h1>Rename production</h1>")
	this.Contains(body, `<form class="sheet formcard" method="post" action="`+productionPath+`/rename">`)
	this.Contains(body, `name="name" type="text" autocomplete="off" value="production" required autofocus>`)
	this.Contains(body, `<div class="warn" role="note">`+renameEnvironmentUiWarnIcon+`<span><b>Pipelines use this name.</b> Until they are updated, any pipeline still using <span class="mono">production</span> gets <code>environment not found</code>. Deployments and checks stay attached to the environment.</span></div>`)
	this.Contains(body, `<button type="submit" class="btn primary">Rename</button><a class="btn" href="`+productionPath+`">Cancel</a>`)
	this.NotContains(body, `aria-invalid`)
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_RenamesAndRedirectsToTheEnvironmentWithTheSessionMessage() {
	productionID := this.renameEnvironmentUiCreate("production")
	productionPath := "/environments/" + strconv.FormatInt(productionID, 10)
	sessionCookie, csrfToken := this.renameEnvironmentUiWriter()

	status, header, body := this.postForm(renameEnvironmentUiPath(productionID), sessionCookie, csrfToken, url.Values{"name": {"prod"}})

	this.Require().Equal(http.StatusSeeOther, status, body)
	this.Equal(productionPath, header.Get("Location"))
	this.Equal("prod", this.renameEnvironmentUiNameOf(productionID))
	this.Equal(1, this.countRows("environments"))

	_, _, landed := this.getPage(productionPath, sessionCookie)
	this.Contains(landed, `<div class="flash" role="status">`)
	this.Contains(landed, "<span>"+renameEnvironmentUiRenamed+"</span>")
	this.Contains(landed, `<h1 class="name">prod</h1>`)

	_, _, reloaded := this.getPage(productionPath, sessionCookie)
	this.NotContains(reloaded, renameEnvironmentUiRenamed)
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_TheSameNameIsASuccess() {
	productionID := this.renameEnvironmentUiCreate("production")

	status, header, body := this.renameEnvironmentUiPost(productionID, "production")

	this.Require().Equal(http.StatusSeeOther, status, body)
	this.Equal("/environments/"+strconv.FormatInt(productionID, 10), header.Get("Location"))
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_APaddedNameIsAFieldErrorKeepingTheValue() {
	productionID := this.renameEnvironmentUiCreate("production")

	status, header, body := this.renameEnvironmentUiPost(productionID, " prod")

	this.Require().Equal(http.StatusBadRequest, status, body)
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, "<h1>Rename production</h1>")
	this.Contains(body, `value=" prod" required autofocus `+renameEnvironmentUiNameInvalidMark)
	this.Contains(body, renameEnvironmentUiNameFieldError)
	this.Contains(body, "<span>"+renameEnvironmentUiInvalidInput+"</span>")
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_AnEmptyNameIsAFieldError() {
	productionID := this.renameEnvironmentUiCreate("production")

	status, _, body := this.renameEnvironmentUiPost(productionID, "")

	this.Require().Equal(http.StatusBadRequest, status, body)
	this.Contains(body, `value="" required autofocus `+renameEnvironmentUiNameInvalidMark)
	this.Contains(body, "<span>"+renameEnvironmentUiInvalidInput+"</span>")
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_ATakenNameIsAFieldConflictKeepingTheValue() {
	productionID := this.renameEnvironmentUiCreate("production")
	this.renameEnvironmentUiCreate("staging")

	status, header, body := this.renameEnvironmentUiPost(productionID, "staging")

	this.Require().Equal(http.StatusConflict, status, body)
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `value="staging" required autofocus `+renameEnvironmentUiNameInvalidMark)
	this.Contains(body, renameEnvironmentUiNameFieldError)
	this.Contains(body, "<span>"+renameEnvironmentUiAlreadyExists+"</span>")
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_AnUnknownIdIsNotFound() {
	sessionCookie, csrfToken := this.renameEnvironmentUiWriter()

	for _, path := range []string{"/environments/999999/rename", "/environments/01/rename", "/environments/0/rename", "/environments/abc/rename"} {
		getStatus, _, getBody := this.getPage(path, sessionCookie)
		postStatus, _, postBody := this.postForm(path, sessionCookie, csrfToken, url.Values{"name": {"prod"}})

		this.Equal(http.StatusNotFound, getStatus, path)
		this.Contains(getBody, renameEnvironmentUiNotFound, path)
		this.Equal(http.StatusNotFound, postStatus, path)
		this.Contains(postBody, renameEnvironmentUiNotFound, path)
	}
	this.Zero(this.countRows("environments"))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_RenameByIDOfAnIdGoneInBetweenIsNotFound() {
	productionID := this.renameEnvironmentUiCreate("production")
	_, err := this.Pool.Exec(context.Background(), `DELETE FROM environments WHERE id = $1`, productionID)
	this.Require().NoError(err)

	found, conflict := repository.NewEnvironmentRepository(this.Pool).RenameByID(context.Background(), productionID, "prod")

	this.False(found)
	this.False(conflict)
	this.Zero(this.countRows("environments"))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_RenameByIDOntoATakenNameIsAFoundConflict() {
	productionID := this.renameEnvironmentUiCreate("production")
	this.renameEnvironmentUiCreate("staging")

	found, conflict := repository.NewEnvironmentRepository(this.Pool).RenameByID(context.Background(), productionID, "staging")

	this.True(found)
	this.True(conflict)
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_AReadUserIsForbiddenOnGetAndPost() {
	productionID := this.renameEnvironmentUiCreate("production")
	sessionCookie, csrfToken := this.renameEnvironmentUiReader()

	getStatus, _, getBody := this.getPage(renameEnvironmentUiPath(productionID), sessionCookie)
	postStatus, _, postBody := this.postForm(renameEnvironmentUiPath(productionID), sessionCookie, csrfToken, url.Values{"name": {"prod"}})

	this.Equal(http.StatusForbidden, getStatus)
	this.Contains(getBody, renameEnvironmentUiNoAccess)
	this.Equal(http.StatusForbidden, postStatus)
	this.Contains(postBody, renameEnvironmentUiNoAccess)
	this.Equal("production", this.renameEnvironmentUiNameOf(productionID))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_SignedOutIsRedirectedToSignIn() {
	productionID := this.renameEnvironmentUiCreate("production")

	status, header, _ := this.getPage(renameEnvironmentUiPath(productionID), "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_TheEnvironmentPageOffersRenameToAWriter() {
	productionID := this.renameEnvironmentUiCreate("production")
	sessionCookie, _ := this.renameEnvironmentUiWriter()

	status, _, body := this.getPage("/environments/"+strconv.FormatInt(productionID, 10), sessionCookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<div class="acts"><a class="btn" href="`+renameEnvironmentUiPath(productionID)+`">Rename</a></div>`)
}

func (this *IntegrationSuite) TestRenameEnvironmentUi_TheEnvironmentPageHidesRenameFromAReadUser() {
	productionID := this.renameEnvironmentUiCreate("production")
	sessionCookie, _ := this.renameEnvironmentUiReader()

	status, _, body := this.getPage("/environments/"+strconv.FormatInt(productionID, 10), sessionCookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.NotContains(body, "Rename")
	this.NotContains(body, renameEnvironmentUiPath(productionID))
}
