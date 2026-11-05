package rename_participant

const (
	ParticipantRenamedMessage               string = "participant renamed"
	ParticipantInvalidInputMessage          string = "participant invalid input"
	ParticipantNameNotSnakeCaseMessage      string = "participant name must be snake_case"
	ParticipantAlreadyExistsMessage         string = "participant already exists"
	ParticipantNotFoundMessage              string = "participant not found"
	ParticipantHasPublishedContractsMessage string = "participant has published contracts, create a new participant and switch every consumer's consumes to it"

	participantNameParam = "name"
	nameFormField        = "name"

	renameParticipantTemplate = "rename_participant.html"
)

type RenameParticipantPage struct {
	Name    string
	NewName string
	Alert   string
	Field   string
	Message string
}

func newRenameParticipantPage(name, newName string, published bool) RenameParticipantPage {
	page := RenameParticipantPage{Name: name, NewName: newName}
	if published {
		page.Alert = ParticipantHasPublishedContractsMessage
	}

	return page
}

type RenameParticipantRequestBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type RenameParticipantResponseBody struct {
	Message string `json:"message"`
}
