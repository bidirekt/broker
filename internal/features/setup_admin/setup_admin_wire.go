package setup_admin

const (
	EmailInvalidMessage    string = "invalid email"
	PasswordInvalidMessage string = "the password must have 8 to 128 characters"

	emailFormField    = "email"
	passwordFormField = "password"

	signInPath = "/sign-in"
	homePath   = "/"

	setupTemplate = "setup.html"
)

type SetupPage struct {
	Email   string
	Field   string
	Message string
}
