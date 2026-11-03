package invite_user

const (
	EmailInvalidMessage      string = "invalid email"
	PermissionInvalidMessage string = "invalid permission"
	EmailAlreadyInUseMessage string = "email already in use"

	emailFormField      = "email"
	permissionFormField = "permission"

	setPasswordPath = "/set-password?token="

	inviteUserTemplate = "invite_user.html"
)

type InviteUserPage struct {
	Email      string
	Permission string
	Message    string
	Link       string
}
