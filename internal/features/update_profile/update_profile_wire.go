package update_profile

import "github.com/bidirekt/broker/internal/model"

const (
	PasswordChangedMessage        string = "password changed"
	CurrentPasswordWrongMessage   string = "the current password is wrong"
	PasswordInvalidMessage        string = "the password must have 15 to 128 characters"
	TooManyProfileAttemptsMessage string = "too many attempts, try again later"

	currentPasswordFormField = "current_password"
	newPasswordFormField     = "new_password"

	profilePath = "/profile"

	updateProfileTemplate = "update_profile.html"
)

var permissionDescriptions = map[model.Permission]string{
	model.PermissionRead:  "Sees every data screen and their own profile; runs can-i-deploy and validate.",
	model.PermissionWrite: "Everything read does, plus publishing contracts, recording deployments, and creating participants and environments, and renaming environments.",
	model.PermissionAdmin: "Everything write can do, plus managing users. The only admin of this broker.",
}

type UpdateProfilePage struct {
	Email                 string
	Permission            model.Permission
	PermissionDescription string
	Field                 string
	Message               string
}
