package sign_in

const (
	InvalidCredentialsMessage string = "invalid email, username or password"

	identifierFormField = "identifier"
	passwordFormField   = "password"

	homePath      = "/"
	adminHomePath = "/users"
	userHomePath  = "/profile"

	signInTemplate = "sign_in.html"
)

type SignInPage struct {
	Identifier string
	Message    string
}
