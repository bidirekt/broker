package setup_admin

const (
	EmailInvalidMessage    string = "invalid email"
	PasswordInvalidMessage string = "the password must have 15 to 128 characters"

	emailFormField    = "email"
	passwordFormField = "password"

	signInPath = "/sign-in"
	usersPath  = "/users"

	setupTemplate = "setup.html"
)

type SetupPage struct {
	Email   string
	Message string
}
