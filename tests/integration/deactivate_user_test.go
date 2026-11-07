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
	deactivateUserEmail        = "ana@x.com"
	deactivateUserPassword     = "deactivate-user-correct-horse"
	deactivateUserLinkToken    = "deactivate-user-pending-token"
	deactivateUserNotFoundPage = "<h1>user not found</h1>"
	deactivateUserErrorStatus  = `<div class="code-n">404</div>`
	deactivateUserNotice       = `<div class="flash" role="status">`
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

func deactivateUserInBackground(send func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		send()
	}()

	return panicked
}

func (this *IntegrationSuite) deactivateUserAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) deactivateUserSendForm(path, sessionCookie, csrfToken string, form url.Values) (status int) {
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

func (this *IntegrationSuite) deactivateUserCountRows(table string, userID int64) int {
	var count int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table+` WHERE user_id = $1`, userID).Scan(&count)
	this.Require().NoError(err)

	return count
}

func (this *IntegrationSuite) deactivateUserSeedLink(userID int64) {
	tokenHash := sha256.Sum256([]byte(deactivateUserLinkToken))
	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(), &model.OneTimeLink{
		TokenHash: tokenHash[:],
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	this.Require().True(found)
	this.Require().False(deactivated)
}

func (this *IntegrationSuite) deactivateUserRacing(userID int64, race func(adminCookie, csrfToken string) (status int)) (raceStatus int) {
	ctx := context.Background()
	this.deactivateUserSeedLink(userID)
	adminCookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", adminCookie)
	csrfToken := this.csrfTokenFrom(users)

	linkOnHold, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = linkOnHold.Rollback(ctx) }()
	_, err = linkOnHold.Exec(ctx, `SELECT 1 FROM one_time_links WHERE user_id = $1 FOR UPDATE`, userID)
	this.Require().NoError(err)

	var deactivateStatus int
	deactivateFinished := deactivateUserInBackground(func() {
		deactivateStatus = this.deactivateUserSendForm(fmt.Sprintf("/users/%d/deactivate", userID), adminCookie, csrfToken, url.Values{})
	})
	this.deactivateUserAwaitLockWaiters(1)
	raceFinished := deactivateUserInBackground(func() {
		raceStatus = race(adminCookie, csrfToken)
	})
	this.deactivateUserAwaitLockWaiters(2)

	this.Require().NoError(linkOnHold.Rollback(ctx))
	this.Require().Nil(<-deactivateFinished)
	this.Require().Nil(<-raceFinished)
	this.Require().Equal(http.StatusSeeOther, deactivateStatus)

	return raceStatus
}

func (this *IntegrationSuite) TestDeactivateUser_EndsTheSessionsTheSignInAndThePendingLink() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)
	userCookie := this.signInAsUser(deactivateUserEmail, deactivateUserPassword)
	this.deactivateUserSeedLink(userID)

	status, header, _ := this.deactivateUser(fmt.Sprint(userID))

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))

	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserDeactivated, user.State())

	status, header, _ = this.getPage("/profile", userCookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	status, _, _ = this.postForm("/sign-in", "", "", url.Values{"email": {deactivateUserEmail}, "password": {deactivateUserPassword}})
	this.Equal(http.StatusUnauthorized, status)

	status, _, body := this.getPage("/set-password?token="+deactivateUserLinkToken, "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, deactivateUserLinkNotValid)
}

func (this *IntegrationSuite) TestDeactivateUser_ASignInThatReadTheUserBeforeTheDeactivationLeavesNoSession() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)

	signInStatus := this.deactivateUserRacing(userID, func(string, string) int {
		return this.deactivateUserSendForm("/sign-in", "", "", url.Values{"email": {deactivateUserEmail}, "password": {deactivateUserPassword}})
	})

	this.Equal(http.StatusUnauthorized, signInStatus)
	this.Equal(0, this.deactivateUserCountRows("sessions", userID))
}

func (this *IntegrationSuite) TestDeactivateUser_AUserDeactivatedWhileALinkWaitsForTheUserGetsNoLink() {
	userID := this.createActiveUser(deactivateUserEmail, model.PermissionWrite, deactivateUserPassword)

	linkStatus := this.deactivateUserRacing(userID, func(adminCookie, csrfToken string) int {
		return this.deactivateUserSendForm(fmt.Sprintf("/users/%d/link", userID), adminCookie, csrfToken, url.Values{})
	})

	this.Equal(http.StatusConflict, linkStatus)
	this.Equal(0, this.deactivateUserCountRows("one_time_links", userID))
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
		this.Contains(body, deactivateUserNotFoundPage, id)
		this.Contains(body, deactivateUserErrorStatus, id)
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
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/deactivate"><input type="hidden" name="page" value="1"><button type="submit" class="btn sm danger">Deactivate</button></form>`, invitedID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/deactivate"><input type="hidden" name="page" value="1"><button type="submit" class="btn sm danger">Deactivate</button></form>`, activeID))
	this.NotContains(body, fmt.Sprintf(`action="/users/%d/deactivate"`, deactivatedID))
}

func (this *IntegrationSuite) deactivateUserPostAs(cookie string, userID int64, form url.Values) (status int, header http.Header) {
	_, _, users := this.getPage("/users", cookie)
	status, header, _ = this.postForm(fmt.Sprintf("/users/%d/deactivate", userID), cookie, this.csrfTokenFrom(users), form)

	return status, header
}

func (this *IntegrationSuite) TestDeactivateUser_ConfirmsWithASessionMessageOnTheUsersPage() {
	userID := this.createActiveUser("Ana@X.com", model.PermissionWrite, deactivateUserPassword)
	this.setUserState(userID, model.UserActive)
	cookie := this.signInAsAdmin()

	status, _ := this.deactivateUserPostAs(cookie, userID, url.Values{})
	this.Require().Equal(http.StatusSeeOther, status)

	_, _, users := this.getPage("/users", cookie)
	this.Contains(users, deactivateUserNotice)
	this.Contains(users, "<span>user deactivated: Ana@X.com</span>")

	_, _, reloaded := this.getPage("/users", cookie)
	this.NotContains(reloaded, deactivateUserNotice)
}

func (this *IntegrationSuite) TestDeactivateUser_SetsNoMessageWhenTheStateDidNotChange() {
	for _, state := range []model.UserState{
		model.UserDeactivated,
	} {
		userID := this.createActiveUser(string(state)+"@x.com", model.PermissionWrite, deactivateUserPassword)
		this.setUserState(userID, state)
		cookie := this.signInAsAdmin()

		status, _ := this.deactivateUserPostAs(cookie, userID, url.Values{})
		this.Require().Equal(http.StatusSeeOther, status, state)

		_, _, users := this.getPage("/users", cookie)
		this.NotContains(users, deactivateUserNotice, state)
	}
}

func (this *IntegrationSuite) TestDeactivateUser_ReturnsToThePageItCameFrom() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, deactivateUserPassword)
	this.setUserState(userID, model.UserActive)

	status, header := this.deactivateUserPostAs(this.signInAsAdmin(), userID, url.Values{"page": {"3"}})

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/users?page=3", header.Get("Location"))
}

func (this *IntegrationSuite) TestDeactivateUser_AnswersNotFoundForANonCanonicalPageAndChangesNothing() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, deactivateUserPassword)
	this.setUserState(userID, model.UserActive)
	cookie := this.signInAsAdmin()

	for _, page := range []string{"0", "01", "+1", "", "abc"} {
		status, _ := this.deactivateUserPostAs(cookie, userID, url.Values{"page": {page}})

		this.Equal(http.StatusNotFound, status, page)
	}

	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserActive, user.State())
}

func (this *IntegrationSuite) TestDeactivateUser_AnswersNotFoundForAPageWhoseOffsetWouldOverflowAndChangesNothing() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, deactivateUserPassword)

	status, _ := this.deactivateUserPostAs(this.signInAsAdmin(), userID, url.Values{"page": {"400000000000000000"}})

	this.Equal(http.StatusNotFound, status)
	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)
	this.Equal(model.UserActive, user.State())
}
