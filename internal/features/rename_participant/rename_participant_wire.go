package rename_participant

const (
	ParticipantRenamed               string = "participant renamed"
	ParticipantInvalidInput          string = "participant invalid input"
	ParticipantNameNotSnakeCase      string = "participant name must be snake_case"
	ParticipantAlreadyExists         string = "participant already exists"
	ParticipantNotFound              string = "participant not found"
	ParticipantHasPublishedContracts string = "participant has published contracts, create a new participant and switch every consumer's consumes to it"
)

type RenameParticipantRequestBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type RenameParticipantResponseBody struct {
	Message string `json:"message"`
}
