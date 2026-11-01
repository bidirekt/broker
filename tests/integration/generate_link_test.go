package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	generateLinkPassword        = "generate-link-correct-horse"
	generateLinkPreviousToken   = "generate-link-previous-token"
	generateLinkUserNotFound    = "user not found"
	generateLinkUserDeactivated = "deactivated users cannot get a link"
)

var generateLinkInPage = regexp.MustCompile(`http://localhost:8080/set-password\?token=([A-Za-z0-9_-]{43})`)

func (this *IntegrationSuite) generateLinkFor(userID string) (status int, body string) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	status, _, body = this.postForm("/users/"+userID+"/link", cookie, this.csrfTokenFrom(users), url.Values{})

	return status, body
}

func (this *IntegrationSuite) generateLinkPreviousOpenLink(userID int64, purpose model.LinkPurpose) []byte {
	tokenHash := sha256.Sum256([]byte(generateLinkPreviousToken))
	repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(time.Hour),
	})

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
	this.Equal(1, this.countRows("one_time_links"))

	_, _, list := this.getPage("/users", this.signInAsAdmin())
	this.NotContains(list, match[1])
}

func (this *IntegrationSuite) TestGenerateLink_InvitedUserGetsANewInviteLinkReplacingThePrevious() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(userID, model.UserInvited)
	previousHash := this.generateLinkPreviousOpenLink(userID, model.LinkPurposeInvite)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "ana@x.com")
	this.generateLinkAssertReplaced(userID, previousHash, body, model.LinkPurposeInvite)
}

func (this *IntegrationSuite) TestGenerateLink_ActiveUserGetsAResetLinkReplacingThePrevious() {
	userID := this.createActiveUser("bob@x.com", model.PermissionWrite, generateLinkPassword)
	previousHash := this.generateLinkPreviousOpenLink(userID, model.LinkPurposeReset)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "bob@x.com")
	this.generateLinkAssertReplaced(userID, previousHash, body, model.LinkPurposeReset)
}

func (this *IntegrationSuite) TestGenerateLink_RefusesADeactivatedUser() {
	userID := this.createActiveUser("zoe@x.com", model.PermissionRead, generateLinkPassword)
	this.setUserState(userID, model.UserDeactivated)

	status, body := this.generateLinkFor(fmt.Sprint(userID))

	this.Equal(http.StatusConflict, status)
	this.Contains(body, generateLinkUserDeactivated)
	this.NotRegexp(generateLinkInPage, body)
	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestGenerateLink_AnswersUserNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, generateLinkPassword)

	for _, id := range []string{fmt.Sprint(userID + 1), "abc", "0" + fmt.Sprint(userID), "+" + fmt.Sprint(userID)} {
		status, body := this.generateLinkFor(id)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, generateLinkUserNotFound, id)
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
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/link"><button type="submit">New invite link</button></form>`, invitedID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/link"><button type="submit">New reset link</button></form>`, activeID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/link"`, deactivatedID))
}
