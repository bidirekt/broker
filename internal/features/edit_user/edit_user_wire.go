package edit_user

const (
	EmailInvalidMessage      string = "invalid email"
	PermissionInvalidMessage string = "invalid permission"
	EmailAlreadyInUseMessage string = "email already in use"
	UserNotFoundMessage      string = "user not found"

	userIDParam = "id"

	emailFormField      = "email"
	permissionFormField = "permission"

	usersPath = "/users"

	editUserTemplate = "edit_user.html"
)

type EditUserPage struct {
	UserID     int64
	Email      string
	Permission string
	Message    string
}
