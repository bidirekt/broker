package rename_participant

const (
	ParticipantRenamedMessage               string = "participant renamed"
	ParticipantInvalidInputMessage          string = "participant invalid input"
	ParticipantNameNotSnakeCaseMessage      string = "participant name must be snake_case"
	ParticipantAlreadyExistsMessage         string = "participant already exists"
	ParticipantNotFoundMessage              string = "participant not found"
	ParticipantHasPublishedContractsMessage string = "participant has published contracts, create a new participant and switch every consumer's consumes to it"
)

type RenameParticipantRequestBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type RenameParticipantResponseBody struct {
	Message string `json:"message"`
}
