package update_profile

const (
	PasswordChangedMessage      string = "password changed"
	CurrentPasswordWrongMessage string = "the current password is wrong"
	PasswordInvalidMessage      string = "the password must have 15 to 128 characters"

	currentPasswordFormField = "current_password"
	newPasswordFormField     = "new_password"

	profilePath = "/profile"

	updateProfileTemplate = "update_profile.html"
)

type UpdateProfilePage struct {
	Problem      string
	Confirmation string
}
