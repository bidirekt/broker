package sign_in

const (
	PageInvalidCredentialsMessage string = "invalid email, username or password"
	ApiInvalidCredentialsMessage  string = "invalid credentials"
	SignInInvalidInputMessage     string = "sign in invalid input"
	SignedInMessage               string = "signed in"
	TooManySignInAttemptsMessage  string = "too many attempts, try again later"
	ForbiddenMessage              string = "forbidden"

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

type SignInRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignInResponseBody struct {
	Message string `json:"message"`
}

type SignedInResponseBody struct {
	Message   string `json:"message"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}
