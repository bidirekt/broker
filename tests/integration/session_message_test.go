package integration_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const (
	sessionMessageText   = "participant created"
	sessionMessageNotice = `<div class="flash" role="status">`
	sessionMessagePage   = "/profile"
)

func (this *IntegrationSuite) sessionMessageSet(sessionCookie, message string) {
	idHash := sha256.Sum256([]byte(sessionCookie))
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET message = $2 WHERE id_hash = $1`, idHash[:], message)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) sessionMessageStored(sessionCookie string) (message *string) {
	idHash := sha256.Sum256([]byte(sessionCookie))
	err := this.Pool.QueryRow(context.Background(), `SELECT message FROM sessions WHERE id_hash = $1`, idHash[:]).Scan(&message)
	this.Require().NoError(err)

	return message
}

func (this *IntegrationSuite) TestSessionMessage_ShowsOnceOnTheNextGetPageAndIsGoneOnReload() {
	cookie := this.signInAsAdmin()
	this.sessionMessageSet(cookie, sessionMessageText)

	status, _, page := this.getPage(sessionMessagePage, cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(page, sessionMessageNotice)
	this.Contains(page, "<span>"+sessionMessageText+"</span>")
	this.Nil(this.sessionMessageStored(cookie))

	status, _, page = this.getPage(sessionMessagePage, cookie)
	this.Equal(http.StatusOK, status)
	this.NotContains(page, sessionMessageNotice)
	this.NotContains(page, sessionMessageText)
}

func (this *IntegrationSuite) TestSessionMessage_APageRenderedByAPostNeitherShowsNorTakesIt() {
	cookie := this.signInAsAdmin()
	status, _, page := this.getPage(sessionMessagePage, cookie)
	this.Require().Equal(http.StatusOK, status)
	csrfToken := this.csrfTokenFrom(page)
	this.sessionMessageSet(cookie, sessionMessageText)

	status, _, page = this.postForm(sessionMessagePage, cookie, csrfToken, url.Values{
		"current_password": {"not-the-current-password"},
		"new_password":     {"a-new-password-long-enough"},
	})
	this.Equal(http.StatusBadRequest, status)
	this.NotContains(page, sessionMessageNotice)
	this.NotContains(page, sessionMessageText)

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.Equal(sessionMessageText, *stored)
}

func (this *IntegrationSuite) TestSessionMessage_AHeadRequestDoesNotTakeIt() {
	cookie := this.signInAsAdmin()
	this.sessionMessageSet(cookie, sessionMessageText)

	status, _, _ := this.sendPageRequest(httptest.NewRequest(http.MethodHead, sessionMessagePage, nil), cookie)
	this.Equal(http.StatusOK, status)

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.Equal(sessionMessageText, *stored)

	_, _, page := this.getPage(sessionMessagePage, cookie)
	this.Contains(page, sessionMessageNotice)
}

func (this *IntegrationSuite) TestSessionMessage_ARequestThatDoesNotAskForAPageDoesNotTakeIt() {
	cookie := this.signInAsAdmin()
	this.sessionMessageSet(cookie, sessionMessageText)

	req := httptest.NewRequest(http.MethodGet, sessionMessagePage, nil)
	req.AddCookie(&http.Cookie{Name: suiteSessionCookieName, Value: cookie})
	req.Header.Set("Accept", "application/json")
	resp, err := this.Components.Server.Test(req)
	this.Require().NoError(err)
	_ = resp.Body.Close()

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.Equal(sessionMessageText, *stored)
}
