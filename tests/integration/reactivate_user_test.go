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
	reactivateUserEmail     = "ana@x.com"
	reactivateUserPassword  = "reactivate-user-correct-horse"
	reactivateUserLinkToken = "reactivate-user-pending-token"
	reactivateUserNotFound  = "user not found"
)

func (this *IntegrationSuite) reactivateUserPost(userID int64, action string) (status int, header http.Header) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	status, header, _ = this.postForm(fmt.Sprintf("/users/%d/%s", userID, action), cookie, this.csrfTokenFrom(users), url.Values{})

	return status, header
}

func (this *IntegrationSuite) reactivateUserState(userID int64) model.UserState {
	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)

	return user.State()
}

func (this *IntegrationSuite) reactivateUserUpdatedAt(userID int64) time.Time {
	var updatedAt time.Time
	err := this.Pool.QueryRow(context.Background(), `SELECT updated_at FROM users WHERE id = $1`, userID).Scan(&updatedAt)
	this.Require().NoError(err)

	return updatedAt
}

func (this *IntegrationSuite) TestReactivateUser_TheUserSignsInAgain() {
	userID := this.createActiveUser(reactivateUserEmail, model.PermissionWrite, reactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)

	status, header := this.reactivateUserPost(userID, "reactivate")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))
	this.Equal(model.UserActive, this.reactivateUserState(userID))
	this.NotEmpty(this.signInAsUser(reactivateUserEmail, reactivateUserPassword))
}

func (this *IntegrationSuite) TestReactivateUser_AnInvitedUserComesBackInvitedWithNoLink() {
	userID := this.createActiveUser(reactivateUserEmail, model.PermissionRead, reactivateUserPassword)
	this.setUserState(userID, model.UserInvited)
	tokenHash := sha256.Sum256([]byte(reactivateUserLinkToken))
	_, found, deactivated := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	this.Require().True(found)
	this.Require().False(deactivated)
	status, _ := this.reactivateUserPost(userID, "deactivate")
	this.Require().Equal(http.StatusSeeOther, status)

	status, header := this.reactivateUserPost(userID, "reactivate")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))
	this.Equal(model.UserInvited, this.reactivateUserState(userID))
	this.Equal(0, this.countRows("one_time_links"))
	_, _, usable := repository.NewOneTimeLinkRepository(this.Pool).FindUsable(context.Background(), tokenHash[:])
	this.False(usable, "the link from before the deactivation works again")
}

func (this *IntegrationSuite) TestReactivateUser_AnActiveOrInvitedUserGoesBackToUsersUnchanged() {
	activeID := this.createActiveUser("ana@x.com", model.PermissionWrite, reactivateUserPassword)
	invitedID := this.createActiveUser("bob@x.com", model.PermissionWrite, reactivateUserPassword)
	this.setUserState(invitedID, model.UserInvited)

	for userID, state := range map[int64]model.UserState{activeID: model.UserActive, invitedID: model.UserInvited} {
		updatedAt := this.reactivateUserUpdatedAt(userID)

		status, header := this.reactivateUserPost(userID, "reactivate")

		this.Equal(http.StatusSeeOther, status, state)
		this.Equal("/users", header.Get("Location"), state)
		this.Equal(state, this.reactivateUserState(userID))
		this.Equal(updatedAt, this.reactivateUserUpdatedAt(userID), state)
	}
}

func (this *IntegrationSuite) TestReactivateUser_AnswersUserNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser(reactivateUserEmail, model.PermissionWrite, reactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	for _, id := range []string{fmt.Sprint(userID + 1), "abc", "0" + fmt.Sprint(userID), "+" + fmt.Sprint(userID)} {
		status, _, body := this.postForm("/users/"+id+"/reactivate", cookie, this.csrfTokenFrom(users), url.Values{})

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, reactivateUserNotFound, id)
	}

	this.Equal(model.UserDeactivated, this.reactivateUserState(userID))
}

func (this *IntegrationSuite) TestReactivateUser_ButtonIsShownOnlyForADeactivatedUser() {
	invitedID := this.createActiveUser("ana@x.com", model.PermissionRead, reactivateUserPassword)
	this.setUserState(invitedID, model.UserInvited)
	activeID := this.createActiveUser("bob@x.com", model.PermissionRead, reactivateUserPassword)
	deactivatedID := this.createActiveUser("zoe@x.com", model.PermissionRead, reactivateUserPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/reactivate"><button type="submit">Reactivate</button></form>`, deactivatedID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/reactivate"`, invitedID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/reactivate"`, activeID))
}
