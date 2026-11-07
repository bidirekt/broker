package create_environment

const (
	EnvironmentCreatedMessage       string = "environment created"
	EnvironmentInvalidInputMessage  string = "environment invalid input"
	EnvironmentAlreadyExistsMessage string = "environment already exists"

	nameFormField = "name"

	environmentPathPrefix = "/environments/"

	createEnvironmentTemplate = "create_environment.html"
)

type CreateEnvironmentRequestBody struct {
	Environment string `json:"environment"`
}

type CreateEnvironmentResponseBody struct {
	Message string `json:"message"`
}

type CreateEnvironmentPage struct {
	Name    string
	Field   string
	Message string
}
