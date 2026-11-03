package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	editUserPassword          = "edit-user-correct-horse"
	editUserEmailAlreadyInUse = "email already in use"
	editUserInvalidEmail      = "invalid email"
	editUserInvalidPermission = "invalid permission"
	editUserUserNotFound      = "user not found"
)

func (this *IntegrationSuite) editUserPost(userID, email, permission string) (status int, header http.Header, body string) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	return this.postForm("/users/"+userID+"/edit", cookie, this.csrfTokenFrom(users),
		url.Values{"email": {email}, "permission": {permission}})
}

func (this *IntegrationSuite) editUserFind(userID int64) *model.User {
	user, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), userID)
	this.Require().True(found)

	return user
}

func (this *IntegrationSuite) TestEditUser_FormIsPrefilledWithTheEmailAndPermission() {
	userID := this.createActiveUser("ana@x.com", model.PermissionWrite, editUserPassword)

	status, _, body := this.getPage(fmt.Sprintf("/users/%d/edit", userID), this.signInAsAdmin())

	this.Equal(http.StatusOK, status)
	this.Contains(body, `value="ana@x.com"`)
	this.Contains(body, `<option value="write" selected>`)
	this.Contains(body, fmt.Sprintf(`action="/users/%d/edit"`, userID))
}

func (this *IntegrationSuite) TestEditUser_ChangesEmailAndPermissionKeepingTheSessions() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	userCookie := this.signInAsUser("ana@x.com", editUserPassword)

	status, header, _ := this.editUserPost(fmt.Sprint(userID), "anna@y.com", "write")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))

	_, _, users := this.getPage("/users", this.signInAsAdmin())
	this.Contains(users, "<td>anna@y.com</td>")
	this.Contains(users, "<td>write</td>")
	this.NotContains(users, "ana@x.com")

	profileStatus, _, _ := this.getPage("/profile", userCookie)
	this.Equal(http.StatusOK, profileStatus, "the user's session was not kept")

	this.signInAsUser("anna@y.com", editUserPassword)
}

func (this *IntegrationSuite) TestEditUser_AllowsChangingOnlyTheCaseOfTheOwnEmail() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	status, _, _ := this.editUserPost(fmt.Sprint(userID), "Ana@X.com", "read")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("Ana@X.com", this.editUserFind(userID).Email)
}

func (this *IntegrationSuite) TestEditUser_RefusesAnEmailUsedByAnotherUserInAnyCase() {
	this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	bobID := this.createActiveUser("bob@x.com", model.PermissionRead, editUserPassword)

	status, _, body := this.editUserPost(fmt.Sprint(bobID), "ANA@x.com", "write")

	this.Equal(http.StatusConflict, status)
	this.Contains(body, editUserEmailAlreadyInUse)
	bob := this.editUserFind(bobID)
	this.Equal("bob@x.com", bob.Email)
	this.Equal(model.PermissionRead, bob.Permission)
}

func (this *IntegrationSuite) TestEditUser_RefusesAnInvalidEmailOrPermission() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	cases := []struct {
		email      string
		permission string
		message    string
	}{
		{"ana.x.com", "write", editUserInvalidEmail},
		{"ana @x.com", "write", editUserInvalidEmail},
		{"anna@y.com", "admin", editUserInvalidPermission},
		{"anna@y.com", "", editUserInvalidPermission},
	}

	for _, c := range cases {
		status, _, body := this.editUserPost(fmt.Sprint(userID), c.email, c.permission)

		this.Equal(http.StatusBadRequest, status, c.email+" "+c.permission)
		this.Contains(body, c.message, c.email+" "+c.permission)
	}

	user := this.editUserFind(userID)
	this.Equal("ana@x.com", user.Email)
	this.Equal(model.PermissionRead, user.Permission)
}

func (this *IntegrationSuite) TestEditUser_AnswersUserNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	adminCookie := this.signInAsAdmin()

	for _, id := range []string{fmt.Sprint(userID + 1), "abc", "0" + fmt.Sprint(userID), "+" + fmt.Sprint(userID)} {
		getStatus, _, getBody := this.getPage("/users/"+id+"/edit", adminCookie)

		this.Equal(http.StatusNotFound, getStatus, "GET "+id)
		this.Contains(getBody, editUserUserNotFound, "GET "+id)

		postStatus, _, postBody := this.editUserPost(id, "anna@y.com", "write")

		this.Equal(http.StatusNotFound, postStatus, "POST "+id)
		this.Contains(postBody, editUserUserNotFound, "POST "+id)
	}

	this.Equal("ana@x.com", this.editUserFind(userID).Email)
}

func (this *IntegrationSuite) TestEditUser_ListOffersAnEditLinkPerUser() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	_, _, users := this.getPage("/users", this.signInAsAdmin())

	this.Contains(users, fmt.Sprintf(`<a href="/users/%d/edit">Edit</a>`, userID))
}
