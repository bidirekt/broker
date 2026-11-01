package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	deactivateUserEmail        = "ana@x.com"
	deactivateUserPassword     = "deactivate-user-correct-horse"
	deactivateUserLinkToken    = "deactivate-user-pending-token"
	deactivateUserNotFound     = "user not found"
	deactivateUserLinkNotValid = "this link is no longer valid, ask your admin for a new one"
)

func (this *IntegrationSuite) deactivateUser(userID string) (status int, header http.Header, body string) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	return this.postForm("/users/"+userID+"/deactivate", cookie, this.csrfTokenFrom(users), url.Values{})
}

func (this *IntegrationSuite) deactivateUserUpdatedAt(userID int64) time.Time {
	var updatedAt time.Time
	err := this.Pool.QueryRow(context.Background(), `SELECT updated_at FROM users WHERE id = $1`, userID).Scan(&updatedAt)
	this.Require().NoError(err)

	return updatedAt
}

func (this *IntegrationSuite) TestDeactivateUser_EndsTheSessionsTheSignInAndThePendingLink() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)
	userCookie := this.signInAsUser(deactivateUserEmail, deactivateUserPassword)
	tokenHash := sha256.Sum256([]byte(deactivateUserLinkToken))
	_, found, deactivated := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	this.Require().True(found)
	this.Require().False(deactivated)

	status, header, _ := this.deactivateUser(fmt.Sprint(userID))

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))

	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserDeactivated, user.State())

	status, header, _ = this.getPage("/profile", userCookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	status, _, _ = this.postForm("/sign-in", "", "", url.Values{"identifier": {deactivateUserEmail}, "password": {deactivateUserPassword}})
	this.Equal(http.StatusUnauthorized, status)

	status, _, body := this.getPage("/set-password?token="+deactivateUserLinkToken, "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, deactivateUserLinkNotValid)
}

func (this *IntegrationSuite) TestDeactivateUser_AnAlreadyDeactivatedUserGoesBackToUsersUnchanged() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)
	updatedAt := this.deactivateUserUpdatedAt(userID)

	status, header, _ := this.deactivateUser(fmt.Sprint(userID))

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))
	this.Equal(updatedAt, this.deactivateUserUpdatedAt(userID))
}

func (this *IntegrationSuite) TestDeactivateUser_AnswersUserNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)

	for _, id := range []string{fmt.Sprint(userID + 1), "abc", "0" + fmt.Sprint(userID), "+" + fmt.Sprint(userID)} {
		status, _, body := this.deactivateUser(id)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, deactivateUserNotFound, id)
	}

	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserActive, user.State())
}

func (this *IntegrationSuite) TestDeactivateUser_ButtonIsShownForEveryUserNotDeactivated() {
	invitedID := this.createActiveUser("ana@x.com", model.PermissionRead, deactivateUserPassword)
	this.setUserState(invitedID, model.UserInvited)
	activeID := this.createActiveUser("bob@x.com", model.PermissionRead, deactivateUserPassword)
	deactivatedID := this.createActiveUser("zoe@x.com", model.PermissionRead, deactivateUserPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/deactivate"><button type="submit">Deactivate</button></form>`, invitedID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/deactivate"><button type="submit">Deactivate</button></form>`, activeID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/deactivate"`, deactivatedID))
}
