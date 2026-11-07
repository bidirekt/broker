package integration_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	sessionMessageText       = "participant created"
	sessionMessageNotice     = `<div class="flash" role="status">`
	sessionMessagePage       = "/profile"
	sessionMessageStoredJSON = `[{"kind":"success","text":"participant created"}]`
)

func (this *IntegrationSuite) sessionMessageAdd(sessionCookie, text string) {
	idHash := sha256.Sum256([]byte(sessionCookie))
	repository.NewSessionRepository(this.Pool).AddMessage(context.Background(), idHash[:], model.NewSessionMessage(model.MessageSuccess, text))
}

func (this *IntegrationSuite) sessionMessageStored(sessionCookie string) (messages *string) {
	idHash := sha256.Sum256([]byte(sessionCookie))
	err := this.Pool.QueryRow(context.Background(), `SELECT messages::text FROM sessions WHERE id_hash = $1`, idHash[:]).Scan(&messages)
	this.Require().NoError(err)

	return messages
}

func (this *IntegrationSuite) TestSessionMessage_ShowsOnceOnTheNextGetPageAndIsGoneOnReload() {
	cookie := this.signInAsAdmin()
	this.sessionMessageAdd(cookie, sessionMessageText)

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

func (this *IntegrationSuite) TestSessionMessage_TwoMessagesAddedBeforeTheNextGetPageAreBothShownInOrderAndGoneOnReload() {
	cookie := this.signInAsAdmin()
	this.sessionMessageAdd(cookie, "participant created")
	this.sessionMessageAdd(cookie, "environment created")

	status, _, page := this.getPage(sessionMessagePage, cookie)
	this.Equal(http.StatusOK, status)
	this.Equal(2, strings.Count(page, sessionMessageNotice))
	first := strings.Index(page, "<span>participant created</span>")
	second := strings.Index(page, "<span>environment created</span>")
	this.Require().NotEqual(-1, first)
	this.Require().NotEqual(-1, second)
	this.Less(first, second)
	this.Nil(this.sessionMessageStored(cookie))

	status, _, page = this.getPage(sessionMessagePage, cookie)
	this.Equal(http.StatusOK, status)
	this.NotContains(page, sessionMessageNotice)
	this.NotContains(page, "participant created")
	this.NotContains(page, "environment created")
}

func (this *IntegrationSuite) TestSessionMessage_APageRenderedByAPostNeitherShowsNorTakesIt() {
	cookie := this.signInAsAdmin()
	status, _, page := this.getPage(sessionMessagePage, cookie)
	this.Require().Equal(http.StatusOK, status)
	csrfToken := this.csrfTokenFrom(page)
	this.sessionMessageAdd(cookie, sessionMessageText)

	status, _, page = this.postForm(sessionMessagePage, cookie, csrfToken, url.Values{
		"current_password": {"not-the-current-password"},
		"new_password":     {"a-new-password-long-enough"},
	})
	this.Equal(http.StatusBadRequest, status)
	this.NotContains(page, sessionMessageNotice)
	this.NotContains(page, sessionMessageText)

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.JSONEq(sessionMessageStoredJSON, *stored)
}

func (this *IntegrationSuite) TestSessionMessage_AHeadRequestDoesNotTakeIt() {
	cookie := this.signInAsAdmin()
	this.sessionMessageAdd(cookie, sessionMessageText)

	status, _, _ := this.sendPageRequest(httptest.NewRequest(http.MethodHead, sessionMessagePage, nil), cookie)
	this.Equal(http.StatusOK, status)

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.JSONEq(sessionMessageStoredJSON, *stored)

	_, _, page := this.getPage(sessionMessagePage, cookie)
	this.Contains(page, sessionMessageNotice)
}

func (this *IntegrationSuite) TestSessionMessage_ARequestThatDoesNotAskForAPageDoesNotTakeIt() {
	cookie := this.signInAsAdmin()
	this.sessionMessageAdd(cookie, sessionMessageText)

	req := httptest.NewRequest(http.MethodGet, sessionMessagePage, nil)
	req.AddCookie(&http.Cookie{Name: suiteSessionCookieName, Value: cookie})
	req.Header.Set("Accept", "application/json")
	resp, err := this.Components.Server.Test(req)
	this.Require().NoError(err)
	_ = resp.Body.Close()

	stored := this.sessionMessageStored(cookie)
	this.Require().NotNil(stored)
	this.JSONEq(sessionMessageStoredJSON, *stored)
}
