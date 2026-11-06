package integration_test

import (
	"context"
	"net/http"
	"net/url"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	renameParticipantUiReaderEmail     = "rename-reader@x.com"
	renameParticipantUiReaderPassword  = "rename-reader-correct-horse"
	renameParticipantUiNotSnakeCase    = "participant name must be snake_case"
	renameParticipantUiAlreadyExists   = "participant already exists"
	renameParticipantUiRenamed         = "participant renamed"
	renameParticipantUiHasPublished    = "participant has published contracts, create a new participant and switch every consumer&#39;s consumes to it"
	renameParticipantUiNoAccess        = "<h1>you don&#39;t have access</h1>"
	renameParticipantUiNotFound        = "<h1>Not Found</h1>"
	renameParticipantUiDisabledSubmit  = `<button type="submit" class="btn primary" aria-disabled="true" disabled>Rename</button>`
	renameParticipantUiEnabledSubmit   = `<button type="submit" class="btn primary">Rename</button>`
	renameParticipantUiNameFieldError  = `<p class="err" id="name-err"><svg`
	renameParticipantUiNameInvalidMark = `aria-invalid="true" aria-describedby="name-err"`
	renameParticipantUiMessageIcon     = `<svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M8 1.8l6.5 11.7h-13z"/><path d="M8 6.5v3M8 11.6v.01"/></svg>`
	renameParticipantUiWholeFormAlert  = `<div class="alert" role="alert">` + renameParticipantUiMessageIcon
	renameParticipantUiDisabledRename  = `<a class="btn" aria-disabled="true">Rename</a>`
	renameParticipantUiDisabledNote    = `<p class="disabled-note">participant has published contracts, create a new participant and switch every consumer's consumes to it</p>`
)

const renameParticipantUiContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

var renameParticipantUiHeaderTabs = []string{"", "/deployments", "/checks", "/relations"}

func (this *IntegrationSuite) renameParticipantUiCreate(names ...string) {
	for _, name := range names {
		status, response := this.post("/api/participants", `{"participant":"`+name+`"}`)
		this.Require().Equal(http.StatusOK, status, response)
	}
}

func (this *IntegrationSuite) renameParticipantUiPublish(name string) {
	status, response := this.post("/api/contracts", this.publishBody(name, "1.0.0", contractFragment{"contract.yaml", renameParticipantUiContract}))
	this.Require().Equal(http.StatusOK, status, response)
}

func (this *IntegrationSuite) renameParticipantUiWriter() (sessionCookie, csrfToken string) {
	sessionCookie = this.signInAsUser(suiteWriterEmail, suiteWriterPassword)
	_, _, page := this.getPage("/participants", sessionCookie)

	return sessionCookie, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) renameParticipantUiReader() (sessionCookie, csrfToken string) {
	this.createActiveUser(renameParticipantUiReaderEmail, model.PermissionRead, renameParticipantUiReaderPassword)
	sessionCookie = this.signInAsUser(renameParticipantUiReaderEmail, renameParticipantUiReaderPassword)
	_, _, page := this.getPage("/participants", sessionCookie)

	return sessionCookie, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) renameParticipantUiPost(oldName, newName string) (status int, header http.Header, body string) {
	sessionCookie, csrfToken := this.renameParticipantUiWriter()

	return this.postForm("/participants/"+oldName+"/rename", sessionCookie, csrfToken, url.Values{"name": {newName}})
}

func (this *IntegrationSuite) renameParticipantUiExists(name string) bool {
	_, found := repository.NewParticipantRepository(this.Pool).FindByName(context.Background(), name)

	return found
}

func (this *IntegrationSuite) TestRenameParticipantUi_FormIsPrefilledWithTheCurrentNameAndWarnsAboutTheOldName() {
	this.renameParticipantUiCreate("petstore_admin")
	sessionCookie, _ := this.renameParticipantUiWriter()

	status, header, body := this.getPage("/participants/petstore_admin/rename", sessionCookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<nav class="crumbs" aria-label="Breadcrumbs"><a href="/participants">participants</a><span aria-hidden="true">/</span><a href="/participants/petstore_admin">petstore_admin</a><span aria-hidden="true">/</span><span aria-current="page">rename</span></nav>`)
	this.Contains(body, "<h1>Rename petstore_admin</h1>")
	this.Contains(body, `<form class="sheet formcard" method="post" action="/participants/petstore_admin/rename">`)
	this.Contains(body, `name="name" type="text" autocomplete="off" value="petstore_admin" required autofocus>`)
	this.Contains(body, `<div class="warn" role="note">`+renameParticipantUiMessageIcon+`<span>Another participant's <code>consumes</code> and the CI that use <span class="mono">petstore_admin</span> stop finding it under the old name.</span></div>`)
	this.Contains(body, renameParticipantUiEnabledSubmit+`<a class="btn" href="/participants/petstore_admin">Cancel</a>`)
	this.NotContains(body, renameParticipantUiWholeFormAlert)
	this.NotContains(body, `aria-invalid`)
}

func (this *IntegrationSuite) TestRenameParticipantUi_RenamesAndRedirectsToTheNewNameWithTheSessionMessage() {
	this.renameParticipantUiCreate("petstore_admin")
	sessionCookie, csrfToken := this.renameParticipantUiWriter()

	status, header, body := this.postForm("/participants/petstore_admin/rename", sessionCookie, csrfToken, url.Values{"name": {"petstore_backoffice"}})

	this.Require().Equal(http.StatusSeeOther, status, body)
	this.Equal("/participants/petstore_backoffice", header.Get("Location"))
	this.True(this.renameParticipantUiExists("petstore_backoffice"))
	this.False(this.renameParticipantUiExists("petstore_admin"))

	_, _, landed := this.getPage("/participants/petstore_backoffice", sessionCookie)
	this.Contains(landed, `<div class="flash" role="status">`)
	this.Contains(landed, "<span>"+renameParticipantUiRenamed+"</span>")

	_, _, reloaded := this.getPage("/participants/petstore_backoffice", sessionCookie)
	this.NotContains(reloaded, renameParticipantUiRenamed)
}

func (this *IntegrationSuite) TestRenameParticipantUi_TheSameNameIsASuccess() {
	this.renameParticipantUiCreate("petstore_admin")

	status, header, body := this.renameParticipantUiPost("petstore_admin", "petstore_admin")

	this.Require().Equal(http.StatusSeeOther, status, body)
	this.Equal("/participants/petstore_admin", header.Get("Location"))
	this.True(this.renameParticipantUiExists("petstore_admin"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_ANameNotInSnakeCaseIsAFieldErrorKeepingTheValue() {
	this.renameParticipantUiCreate("petstore_admin")

	status, header, body := this.renameParticipantUiPost("petstore_admin", "Petstore-Admin")

	this.Require().Equal(http.StatusBadRequest, status, body)
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `value="Petstore-Admin" required autofocus `+renameParticipantUiNameInvalidMark)
	this.Contains(body, renameParticipantUiNameFieldError)
	this.Contains(body, "<span>"+renameParticipantUiNotSnakeCase+"</span>")
	this.NotContains(body, renameParticipantUiWholeFormAlert)
	this.Contains(body, renameParticipantUiEnabledSubmit)
	this.True(this.renameParticipantUiExists("petstore_admin"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_ATakenNameIsAFieldConflictKeepingTheValue() {
	this.renameParticipantUiCreate("petstore_admin", "petstore_web")

	status, _, body := this.renameParticipantUiPost("petstore_admin", "petstore_web")

	this.Require().Equal(http.StatusConflict, status, body)
	this.Contains(body, `value="petstore_web" required autofocus `+renameParticipantUiNameInvalidMark)
	this.Contains(body, renameParticipantUiNameFieldError)
	this.Contains(body, "<span>"+renameParticipantUiAlreadyExists+"</span>")
	this.NotContains(body, renameParticipantUiWholeFormAlert)
	this.True(this.renameParticipantUiExists("petstore_admin"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_APublishedParticipantIsAWholeFormConflictKeepingTheValue() {
	this.renameParticipantUiCreate("petstore_web")
	this.renameParticipantUiPublish("petstore_web")

	status, _, body := this.renameParticipantUiPost("petstore_web", "petstore_frontend")

	this.Require().Equal(http.StatusConflict, status, body)
	this.Contains(body, renameParticipantUiWholeFormAlert)
	this.Contains(body, "<span>"+renameParticipantUiHasPublished+"</span>")
	this.Contains(body, `value="petstore_frontend" required autofocus>`)
	this.Contains(body, renameParticipantUiDisabledSubmit)
	this.NotContains(body, `aria-invalid`)
	this.True(this.renameParticipantUiExists("petstore_web"))
	this.False(this.renameParticipantUiExists("petstore_frontend"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_TheFormOfAPublishedParticipantShowsTheAlertWithTheSubmitDisabled() {
	this.renameParticipantUiCreate("petstore_web")
	this.renameParticipantUiPublish("petstore_web")
	sessionCookie, _ := this.renameParticipantUiWriter()

	status, _, body := this.getPage("/participants/petstore_web/rename", sessionCookie)

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, renameParticipantUiWholeFormAlert)
	this.Contains(body, "<span>"+renameParticipantUiHasPublished+"</span>")
	this.Contains(body, `value="petstore_web" required autofocus>`)
	this.Contains(body, renameParticipantUiDisabledSubmit)
}

func (this *IntegrationSuite) TestRenameParticipantUi_AnUnknownParticipantIsNotFound() {
	sessionCookie, csrfToken := this.renameParticipantUiWriter()

	getStatus, _, getBody := this.getPage("/participants/ghost/rename", sessionCookie)
	postStatus, _, postBody := this.postForm("/participants/ghost/rename", sessionCookie, csrfToken, url.Values{"name": {"spirit"}})

	this.Equal(http.StatusNotFound, getStatus)
	this.Contains(getBody, renameParticipantUiNotFound)
	this.Equal(http.StatusNotFound, postStatus)
	this.Contains(postBody, renameParticipantUiNotFound)
	this.False(this.renameParticipantUiExists("spirit"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_AReadUserIsForbiddenOnGetAndPost() {
	this.renameParticipantUiCreate("petstore_admin")
	sessionCookie, csrfToken := this.renameParticipantUiReader()

	getStatus, _, getBody := this.getPage("/participants/petstore_admin/rename", sessionCookie)
	postStatus, _, postBody := this.postForm("/participants/petstore_admin/rename", sessionCookie, csrfToken, url.Values{"name": {"petstore_backoffice"}})

	this.Equal(http.StatusForbidden, getStatus)
	this.Contains(getBody, renameParticipantUiNoAccess)
	this.Equal(http.StatusForbidden, postStatus)
	this.Contains(postBody, renameParticipantUiNoAccess)
	this.True(this.renameParticipantUiExists("petstore_admin"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_SignedOutIsRedirectedToSignIn() {
	this.renameParticipantUiCreate("petstore_admin")

	status, header, _ := this.getPage("/participants/petstore_admin/rename", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestRenameParticipantUi_EveryParticipantTabOffersRenameToAWriterBeforeTheFirstPublish() {
	this.renameParticipantUiCreate("petstore_admin")
	sessionCookie, _ := this.renameParticipantUiWriter()

	for _, tab := range renameParticipantUiHeaderTabs {
		status, _, body := this.getPage("/participants/petstore_admin"+tab, sessionCookie)

		this.Require().Equal(http.StatusOK, status, tab)
		this.Contains(body, `<div class="acts"><a class="btn" href="/participants/petstore_admin/rename">Rename</a></div>`, tab)
		this.NotContains(body, renameParticipantUiDisabledRename, tab)
		this.NotContains(body, `class="disabled-note"`, tab)
	}
}

func (this *IntegrationSuite) TestRenameParticipantUi_EveryParticipantTabDisablesRenameWithTheReasonOncePublished() {
	this.renameParticipantUiCreate("petstore_web")
	this.renameParticipantUiPublish("petstore_web")
	sessionCookie, _ := this.renameParticipantUiWriter()

	for _, tab := range renameParticipantUiHeaderTabs {
		status, _, body := this.getPage("/participants/petstore_web"+tab, sessionCookie)

		this.Require().Equal(http.StatusOK, status, tab)
		this.Contains(body, `<div class="acts">`+renameParticipantUiDisabledRename+`</div>`, tab)
		this.Contains(body, renameParticipantUiDisabledNote, tab)
		this.NotContains(body, `/participants/petstore_web/rename`, tab)
	}
}

func (this *IntegrationSuite) TestRenameParticipantUi_EveryParticipantTabHidesRenameFromAReadUser() {
	this.renameParticipantUiCreate("petstore_admin")
	sessionCookie, _ := this.renameParticipantUiReader()

	for _, tab := range renameParticipantUiHeaderTabs {
		status, _, body := this.getPage("/participants/petstore_admin"+tab, sessionCookie)

		this.Require().Equal(http.StatusOK, status, tab)
		this.NotContains(body, "Rename", tab)
	}
}
