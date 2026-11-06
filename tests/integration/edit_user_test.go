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
	editUserUserNotFoundPage  = "<h1>user not found</h1>"
	editUserErrorStatus       = `<div class="code-n">404</div>`
	editUserNotFoundPage      = "<h1>Not Found</h1>"
	editUserReadDescription   = "Sees every data screen and their own profile; runs can-i-deploy and validate."
	editUserWriteDescription  = "Everything read does, plus publishing contracts, recording deployments, and creating participants and environments, and renaming environments."
)

func (this *IntegrationSuite) editUserPost(userID, email, permission string) (status int, header http.Header, body string) {
	return this.editUserPostForm(userID, url.Values{"email": {email}, "permission": {permission}})
}

func (this *IntegrationSuite) editUserPostForm(userID string, form url.Values) (status int, header http.Header, body string) {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)

	return this.postForm("/users/"+userID+"/edit", cookie, this.csrfTokenFrom(users), form)
}

func editUserFieldError(field string) string {
	return `<p class="err" id="` + field + `-err"><svg`
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
	this.Contains(body, "<h1>Edit ana@x.com</h1>")
	this.Contains(body, `value="ana@x.com" required autofocus>`)
	this.Contains(body, `<input type="radio" name="permission" value="read" required>`)
	this.Contains(body, `<input type="radio" name="permission" value="write" required checked>`)
	this.Contains(body, editUserReadDescription)
	this.Contains(body, editUserWriteDescription)
	this.Contains(body, fmt.Sprintf(`<form class="sheet formcard" method="post" action="/users/%d/edit">`, userID))
	this.Contains(body, `<button type="submit" class="btn primary">Save</button><a class="btn" href="/users">Cancel</a>`)
	this.NotContains(body, `name="page"`)
	this.NotContains(body, `aria-invalid`)
}

func (this *IntegrationSuite) TestEditUser_ShowsALineWithTheUserState() {
	activeID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	invitedID := this.createActiveUser("bob@x.com", model.PermissionRead, editUserPassword)
	this.setUserState(invitedID, model.UserInvited)
	deactivatedID := this.createActiveUser("zoe@x.com", model.PermissionRead, editUserPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)
	cookie := this.signInAsAdmin()

	for userID, line := range map[int64]string{
		activeID:      "Active since they set their password.",
		invitedID:     "Invited, has not set a password yet.",
		deactivatedID: "Deactivated.",
	} {
		status, _, body := this.getPage(fmt.Sprintf("/users/%d/edit", userID), cookie)

		this.Equal(http.StatusOK, status, line)
		this.Contains(body, `<p class="sub">`+line+`</p>`, line)
	}
}

func (this *IntegrationSuite) TestEditUser_ARefusedFormKeepsTheStoredEmailInTheHeadingAndTheState() {
	this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	bobID := this.createActiveUser("bob@x.com", model.PermissionRead, editUserPassword)
	this.setUserState(bobID, model.UserInvited)

	for _, email := range []string{"ana@x.com", "bob.x.com"} {
		_, _, body := this.editUserPost(fmt.Sprint(bobID), email, "write")

		this.Contains(body, "<h1>Edit bob@x.com</h1>", email)
		this.Contains(body, `<span aria-current="page">bob@x.com</span>`, email)
		this.Contains(body, `<p class="sub">Invited, has not set a password yet.</p>`, email)
		this.Contains(body, `value="`+email+`"`, email)
	}
}

func (this *IntegrationSuite) TestEditUser_ChangesEmailAndPermissionKeepingTheSessions() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	userCookie := this.signInAsUser("ana@x.com", editUserPassword)

	status, header, _ := this.editUserPost(fmt.Sprint(userID), "anna@y.com", "write")

	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))

	_, _, users := this.getPage("/users", this.signInAsAdmin())
	this.Contains(users, "<td data-l=\"Email\"><span class=\"mono\">anna@y.com</span></td>\n        <td data-l=\"Permission\"><span class=\"badge perm\">write</span></td>")
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
	this.Contains(body, `aria-invalid="true" aria-describedby="email-err">`)
	this.Contains(body, editUserFieldError("email"))
	this.Contains(body, `</svg><span>`+editUserEmailAlreadyInUse+`</span></p>`)
	this.Contains(body, `value="write" required checked>`)
	bob := this.editUserFind(bobID)
	this.Equal("bob@x.com", bob.Email)
	this.Equal(model.PermissionRead, bob.Permission)
}

func (this *IntegrationSuite) TestEditUser_RefusesAnInvalidEmailOrPermission() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	cases := []struct {
		email      string
		permission string
		field      string
		message    string
	}{
		{"ana.x.com", "write", "email", editUserInvalidEmail},
		{"ana @x.com", "write", "email", editUserInvalidEmail},
		{"anna@y.com", "admin", "permission", editUserInvalidPermission},
		{"anna@y.com", "", "permission", editUserInvalidPermission},
	}

	for _, c := range cases {
		status, _, body := this.editUserPost(fmt.Sprint(userID), c.email, c.permission)

		this.Equal(http.StatusBadRequest, status, c.email+" "+c.permission)
		this.Contains(body, `aria-describedby="`+c.field+`-err">`, c.email+" "+c.permission)
		this.Contains(body, editUserFieldError(c.field), c.email+" "+c.permission)
		this.Contains(body, `</svg><span>`+c.message+`</span></p>`, c.email+" "+c.permission)
		if c.field == "permission" {
			this.NotContains(body, " checked", c.email+" "+c.permission)
			this.Contains(body, `<input type="radio" name="permission" value="read" required autofocus>`, c.email+" "+c.permission)
			this.Contains(body, `value="`+c.email+`" required>`, c.email+" "+c.permission)
		}
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
		this.Contains(getBody, editUserUserNotFoundPage, "GET "+id)
		this.Contains(getBody, editUserErrorStatus, "GET "+id)

		postStatus, _, postBody := this.editUserPost(id, "anna@y.com", "write")

		this.Equal(http.StatusNotFound, postStatus, "POST "+id)
		this.Contains(postBody, editUserUserNotFoundPage, "POST "+id)
		this.Contains(postBody, editUserErrorStatus, "POST "+id)
	}

	this.Equal("ana@x.com", this.editUserFind(userID).Email)
}

func (this *IntegrationSuite) TestEditUser_ListOffersAnEditLinkPerUser() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	_, _, users := this.getPage("/users", this.signInAsAdmin())

	this.Contains(users, fmt.Sprintf(`<a class="btn sm" href="/users/%d/edit?page=1">Edit</a>`, userID))
}

func (this *IntegrationSuite) TestEditUser_KeepsThePageItCameFromAndReturnsToIt() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)

	status, _, form := this.getPage(fmt.Sprintf("/users/%d/edit?page=2", userID), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(form, `<input type="hidden" name="page" value="2">`)
	this.Contains(form, `<a class="btn" href="/users?page=2">Cancel</a>`)

	status, _, refused := this.editUserPostForm(fmt.Sprint(userID), url.Values{"email": {"ana.x.com"}, "permission": {"read"}, "page": {"2"}})
	this.Equal(http.StatusBadRequest, status)
	this.Contains(refused, `<input type="hidden" name="page" value="2">`)

	status, header, _ := this.editUserPostForm(fmt.Sprint(userID), url.Values{"email": {"anna@x.com"}, "permission": {"read"}, "page": {"2"}})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/users?page=2", header.Get("Location"))
}

func (this *IntegrationSuite) TestEditUser_AnswersNotFoundForANonCanonicalPage() {
	userID := this.createActiveUser("ana@x.com", model.PermissionRead, editUserPassword)
	cookie := this.signInAsAdmin()

	for _, page := range []string{"0", "01", "+1", "", "abc"} {
		status, _, body := this.getPage(fmt.Sprintf("/users/%d/edit?page=%s", userID, url.QueryEscape(page)), cookie)
		this.Equal(http.StatusNotFound, status, "GET "+page)
		this.Contains(body, editUserNotFoundPage, "GET "+page)

		status, _, body = this.editUserPostForm(fmt.Sprint(userID), url.Values{"email": {"anna@x.com"}, "permission": {"write"}, "page": {page}})
		this.Equal(http.StatusNotFound, status, "POST "+page)
		this.Contains(body, editUserNotFoundPage, "POST "+page)
	}

	this.Equal("ana@x.com", this.editUserFind(userID).Email)
}
