package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	generateLinkPassword         = "generate-link-correct-horse"
	generateLinkPreviousToken    = "generate-link-previous-token"
	generateLinkUserNotFoundPage = "<h1>user not found</h1>"
	generateLinkErrorStatus      = `<div class="code-n">404</div>`
	generateLinkUserDeactivated  = "deactivated users cannot get a link"
	generateLinkWarnIcon         = `<svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M8 1.8l6.5 11.7h-13z"/><path d="M8 6.5v3M8 11.6v.01"/></svg>`
	generateLinkPreviousWarning  = `<div class="warn" role="note">` + generateLinkWarnIcon + `<span>The previous link no longer works.</span></div>`
)

var generateLinkInPage = regexp.MustCompile(`http://localhost:8080/set-password\?token=([A-Za-z0-9_-]{43})`)

func (this *IntegrationSuite) generateLinkFor(userID string) (status int, body string) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	status, _, body = this.postForm("/users/"+userID+"/link", cookie, this.csrfTokenFrom(users), url.Values{})

	return status, body
}

func (this *IntegrationSuite) generateLinkPreviousOpenLink(userID int64) []byte {
	return this.generateLinkPreviousLink(userID, time.Now().Add(time.Hour))
}

func (this *IntegrationSuite) generateLinkPreviousLink(userID int64, expiresAt time.Time) []byte {
	tokenHash := sha256.Sum256([]byte(generateLinkPreviousToken))
	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		ExpiresAt: expiresAt,
	})
	this.Require().True(found)
	this.Require().False(deactivated)

	return tokenHash[:]
}

func (this *IntegrationSuite) generateLinkAssertReplaced(userID int64, previousHash []byte, body string, purpose model.LinkPurpose) {
	match := generateLinkInPage.FindStringSubmatch(body)
	this.Require().Len(match, 2, "page shows no link")

	links := repository.NewOneTimeLinkRepository(this.Pool)
	_, _, previousUsable := links.FindUsable(context.Background(), previousHash)
	this.False(previousUsable, "the previous link still works")

	newHash := sha256.Sum256([]byte(match[1]))
	link, user, usable := links.FindUsable(context.Background(), newHash[:])
	this.Require().True(usable, "the new link does not work")
	this.Equal(userID, user.ID)
	this.Equal(purpose, link.Purpose)
	this.WithinDuration(time.Now().Add(24*time.Hour), link.ExpiresAt, time.Minute)
	this.Contains(body, `<h1>New `+string(purpose)+` link</h1>`)
	this.Contains(body, `<p class="sub">for `+user.Email+`</p>`)
	this.Contains(body, `Send this `+string(purpose)+` link to `+user.Email+`. It works once, until <time class="time" datetime="`+link.ExpiresAt.UTC().Format(time.RFC3339Nano)+`" title="`+link.ExpiresAt.UTC().Format("2006-01-02 15:04")+` UTC">`+link.ExpiresAt.UTC().Format("2006-01-02 15:04")+` UTC</time>, and is not shown again.`)
	this.Contains(body, `<input class="copy" type="text" readonly value="`+match[0]+`"`)
	this.Contains(body, generateLinkPreviousWarning)
	this.Equal(1, this.countRows("one_time_links"))

	_, _, list := this.getPage("/users", this.signInAsAdmin())
	this.NotContains(list, match[1])
}

func (this *IntegrationSuite) TestGenerateLink_InvitedUserGetsANewInviteLinkReplacingThePrevious() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(userID, model.UserInvited)
	previousHash := this.generateLinkPreviousOpenLink(userID)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "ana@x.com")
	this.generateLinkAssertReplaced(userID, previousHash, body, model.LinkPurposeInvite)
}

func (this *IntegrationSuite) TestGenerateLink_ActiveUserGetsAResetLinkReplacingThePrevious() {
	userID := this.createActiveUser("bob@x.com", model.PermissionWrite, generateLinkPassword)
	previousHash := this.generateLinkPreviousOpenLink(userID)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "bob@x.com")
	this.generateLinkAssertReplaced(userID, previousHash, body, model.LinkPurposeReset)
}

func (this *IntegrationSuite) TestGenerateLink_DoesNotWarnAboutAPreviousLinkThatWasNotThereOrHadExpired() {
	withoutLinkID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)
	expiredLinkID := this.createActiveUser("bob@x.com", model.PermissionRead, generateLinkPassword)
	this.generateLinkPreviousLink(expiredLinkID, time.Now().Add(-time.Minute))

	for _, userID := range []int64{withoutLinkID, expiredLinkID} {
		status, body := this.generateLinkFor(fmt.Sprint(userID))

		this.Require().Equal(http.StatusOK, status)
		this.Regexp(generateLinkInPage, body)
		this.NotContains(body, "The previous link no longer works.")
	}
}

func (this *IntegrationSuite) TestGenerateLink_HtmxShowsTheLinkWithoutPushingTheFormURLOrBeingStored() {
	userID := this.createActiveUser("bob@x.com", model.PermissionWrite, generateLinkPassword)
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	req := httptest.NewRequest("POST", fmt.Sprintf("/users/%d/link", userID), strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", this.csrfTokenFrom(users))
	req.Header.Set("HX-Request", "true")

	status, header, body := this.sendPageRequest(req, cookie)

	this.Require().Equal(http.StatusOK, status)
	this.Regexp(generateLinkInPage, body)
	this.Equal("false", header.Get("HX-Push-Url"))
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
}

func (this *IntegrationSuite) TestGenerateLink_RefusesADeactivatedUser() {
	userID := this.createActiveUser("zoe@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(userID, model.UserDeactivated)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Equal(http.StatusConflict, status)
	this.Contains(body, "<h1>New link for zoe@x.com</h1>")
	this.Contains(body, `</svg><span>`+generateLinkUserDeactivated+`</span></div>`)
	this.NotRegexp(generateLinkInPage, body)
	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestGenerateLink_AnswersUserNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)

	for _, id := range []string{fmt.Sprint(userID + 1), "abc", "0" + fmt.Sprint(userID), "+" + fmt.Sprint(userID)} {
		status, body := this.generateLinkFor(id)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, generateLinkUserNotFoundPage, id)
		this.Contains(body, generateLinkErrorStatus, id)
	}

	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestGenerateLink_ButtonIsHiddenForADeactivatedUser() {
	invitedID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(invitedID, model.UserInvited)
	activeID := this.createActiveUser("bob@x.com", model.PermissionRead, generateLinkPassword)
	deactivatedID := this.createActiveUser("zoe@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/link"><button type="submit" class="btn sm">New invite link</button></form>`, invitedID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/link"><button type="submit" class="btn sm">New reset link</button></form>`, activeID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/link"`, deactivatedID))
}
