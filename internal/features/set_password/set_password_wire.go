package set_password

import "time"

const (
	PasswordInvalidMessage string = "the password must have 8 to 128 characters"
	LinkInvalidMessage     string = "this link is no longer valid, ask your admin for a new one"

	tokenField        = "token"
	passwordFormField = "password"

	signInPath = "/sign-in"

	setPasswordTemplate = "set_password.html"
	linkInvalidTemplate = "set_password_link_invalid.html"
)

type SetPasswordPage struct {
	Token     string
	Email     string
	ExpiresAt time.Time
	Field     string
	Message   string
}

type LinkInvalidPage struct {
	Message string
}
