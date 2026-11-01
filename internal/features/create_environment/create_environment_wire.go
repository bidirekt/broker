package create_environment

const (
	EnvironmentCreatedMessage       string = "environment created"
	EnvironmentInvalidInputMessage  string = "environment invalid input"
	EnvironmentAlreadyExistsMessage string = "environment already exists"
)

type CreateEnvironmentRequestBody struct {
	Environment string `json:"environment"`
}

type CreateEnvironmentResponseBody struct {
	Message string `json:"message"`
}
