package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
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

func reactivateUserInBackground(send func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		send()
	}()

	return panicked
}

func (this *IntegrationSuite) reactivateUserAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) reactivateUserSendForm(path, sessionCookie, csrfToken string, form url.Values) (status int) {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", suitePublicURL)
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: suiteSessionCookieName, Value: sessionCookie})
	}

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		panic(err)
	}
	_ = resp.Body.Close()

	return resp.StatusCode
}

func (this *IntegrationSuite) reactivateUserCountRows(table string, userID int64) int {
	var count int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table+` WHERE user_id = $1`, userID).Scan(&count)
	this.Require().NoError(err)

	return count
}

func (this *IntegrationSuite) reactivateUserDeactivateRacing(userID int64, race func(adminCookie, csrfToken string) (status int)) (raceStatus int) {
	ctx := context.Background()
	tokenHash := sha256.Sum256([]byte(reactivateUserLinkToken))
	_, found, deactivated := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(ctx, &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	this.Require().True(found)
	this.Require().False(deactivated)
	adminCookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", adminCookie)
	csrfToken := this.csrfTokenFrom(users)

	linkOnHold, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = linkOnHold.Rollback(ctx) }()
	_, err = linkOnHold.Exec(ctx, `SELECT 1 FROM one_time_links WHERE user_id = $1 FOR UPDATE`, userID)
	this.Require().NoError(err)

	var deactivateStatus int
	deactivateFinished := reactivateUserInBackground(func() {
		deactivateStatus = this.reactivateUserSendForm(fmt.Sprintf("/users/%d/deactivate", userID), adminCookie, csrfToken, url.Values{})
	})
	this.reactivateUserAwaitLockWaiters(1)
	raceFinished := reactivateUserInBackground(func() {
		raceStatus = race(adminCookie, csrfToken)
	})
	this.reactivateUserAwaitLockWaiters(2)

	this.Require().NoError(linkOnHold.Rollback(ctx))
	this.Require().Nil(<-deactivateFinished)
	this.Require().Nil(<-raceFinished)
	this.Require().Equal(http.StatusSeeOther, deactivateStatus)

	return raceStatus
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

func (this *IntegrationSuite) TestReactivateUser_ASignInThatReadTheUserBeforeTheDeactivationHasNoSessionAfterReactivation() {
	userID := this.createActiveUser(reactivateUserEmail, model.PermissionWrite, reactivateUserPassword)

	signInStatus := this.reactivateUserDeactivateRacing(userID, func(string, string) int {
		return this.reactivateUserSendForm("/sign-in", "", "", url.Values{"identifier": {reactivateUserEmail}, "password": {reactivateUserPassword}})
	})
	this.Require().Equal(http.StatusUnauthorized, signInStatus)

	status, _ := this.reactivateUserPost(userID, "reactivate")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal(model.UserActive, this.reactivateUserState(userID))
	this.Equal(0, this.reactivateUserCountRows("sessions", userID))
}

func (this *IntegrationSuite) TestReactivateUser_ALinkRequestedWhileTheUserWasBeingDeactivatedIsNotThereAfterReactivation() {
	userID := this.createActiveUser(reactivateUserEmail, model.PermissionWrite, reactivateUserPassword)

	linkStatus := this.reactivateUserDeactivateRacing(userID, func(adminCookie, csrfToken string) int {
		return this.reactivateUserSendForm(fmt.Sprintf("/users/%d/link", userID), adminCookie, csrfToken, url.Values{})
	})
	this.Require().Equal(http.StatusConflict, linkStatus)

	status, _ := this.reactivateUserPost(userID, "reactivate")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal(model.UserActive, this.reactivateUserState(userID))
	this.Equal(0, this.reactivateUserCountRows("one_time_links", userID))
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
