package rename_environment

const (
	EnvironmentRenamedMessage       string = "environment renamed"
	EnvironmentInvalidInputMessage  string = "environment invalid input"
	EnvironmentNotFoundMessage      string = "environment not found"
	EnvironmentAlreadyExistsMessage string = "environment already exists"
)

type RenameEnvironmentRequestBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type RenameEnvironmentResponseBody struct {
	Message string `json:"message"`
}
