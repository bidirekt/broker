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
	reactivateUserEmail        = "ana@x.com"
	reactivateUserPassword     = "reactivate-user-correct-horse"
	reactivateUserLinkToken    = "reactivate-user-pending-token"
	reactivateUserNotFoundPage = "<h1>user not found</h1>"
	reactivateUserErrorStatus  = `<div class="code-n">404</div>`
	reactivateUserNotice       = `<div class="flash" role="status">`
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
	req.Header.Set("X-Forwarded-For", this.clientIP)
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
	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(ctx, &model.OneTimeLink{
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
	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
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
		return this.reactivateUserSendForm("/sign-in", "", "", url.Values{"email": {reactivateUserEmail}, "password": {reactivateUserPassword}})
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
		this.Contains(body, reactivateUserNotFoundPage, id)
		this.Contains(body, reactivateUserErrorStatus, id)
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
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/reactivate"><input type="hidden" name="page" value="1"><button type="submit" class="btn sm">Reactivate</button></form>`, deactivatedID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/reactivate"`, invitedID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/reactivate"`, activeID))
}

func (this *IntegrationSuite) reactivateUserPostAs(cookie string, userID int64, form url.Values) (status int, header http.Header) {
	_, _, users := this.getPage("/users", cookie)
	status, header, _ = this.postForm(fmt.Sprintf("/users/%d/reactivate", userID), cookie, this.csrfTokenFrom(users), form)

	return status, header
}

func (this *IntegrationSuite) TestReactivateUser_ConfirmsWithASessionMessageOnTheUsersPage() {
	userID := this.createActiveUser("Ana@X.com", model.PermissionWrite, reactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)
	cookie := this.signInAsAdmin()

	status, _ := this.reactivateUserPostAs(cookie, userID, url.Values{})
	this.Require().Equal(http.StatusSeeOther, status)

	_, _, users := this.getPage("/users", cookie)
	this.Contains(users, reactivateUserNotice)
	this.Contains(users, "<span>user reactivated: Ana@X.com</span>")

	_, _, reloaded := this.getPage("/users", cookie)
	this.NotContains(reloaded, reactivateUserNotice)
}

func (this *IntegrationSuite) TestReactivateUser_SetsNoMessageWhenTheStateDidNotChange() {
	for _, state := range []model.UserState{
		model.UserActive,
		model.UserInvited,
	} {
		userID := this.createActiveUser(string(state)+"@x.com", model.PermissionWrite, reactivateUserPassword)
		this.setUserState(userID, state)
		cookie := this.signInAsAdmin()

		status, _ := this.reactivateUserPostAs(cookie, userID, url.Values{})
		this.Require().Equal(http.StatusSeeOther, status, state)

		_, _, users := this.getPage("/users", cookie)
		this.NotContains(users, reactivateUserNotice, state)
	}
}

func (this *IntegrationSuite) TestReactivateUser_ReturnsToThePageItCameFrom() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, reactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)

	status, header := this.reactivateUserPostAs(this.signInAsAdmin(), userID, url.Values{"page": {"3"}})

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/users?page=3", header.Get("Location"))
}

func (this *IntegrationSuite) TestReactivateUser_AnswersNotFoundForANonCanonicalPageAndChangesNothing() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, reactivateUserPassword)
	this.setUserState(userID, model.UserDeactivated)
	cookie := this.signInAsAdmin()

	for _, page := range []string{"0", "01", "+1", "", "abc"} {
		status, _ := this.reactivateUserPostAs(cookie, userID, url.Values{"page": {page}})

		this.Equal(http.StatusNotFound, status, page)
	}

	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserDeactivated, user.State())
}
