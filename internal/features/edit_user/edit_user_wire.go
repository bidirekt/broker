package edit_user

import (
	"strconv"

	"github.com/bidirekt/broker/internal/model"
)

const (
	EmailInvalidMessage      string = "invalid email"
	PermissionInvalidMessage string = "invalid permission"
	EmailAlreadyInUseMessage string = "email already in use"
	UserNotFoundMessage      string = "user not found"

	userIDParam = "id"
	pageParam   = "page"

	emailFormField      = "email"
	permissionFormField = "permission"

	usersPath = "/users"

	editUserTemplate = "edit_user.html"
)

var stateLines = map[model.UserState]string{
	model.UserActive:      "Active since they set their password.",
	model.UserInvited:     "Invited, has not set a password yet.",
	model.UserDeactivated: "Deactivated.",
}

type EditUserPage struct {
	UserID      int64
	StoredEmail string
	StateLine   string
	Email       string
	Permission  string
	ReturnPage  int64
	UsersPath   string
	Field       string
	Message     string
}

func usersPathFor(page int64) string {
	if page == 0 {
		return usersPath
	}

	return usersPath + "?" + pageParam + "=" + strconv.FormatInt(page, 10)
}
