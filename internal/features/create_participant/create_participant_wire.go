package create_participant

const (
	ParticipantCreatedMessage          string = "participant created"
	ParticipantInvalidInputMessage     string = "participant invalid input"
	ParticipantNameNotSnakeCaseMessage string = "participant name must be snake_case"
	ParticipantAlreadyExistsMessage    string = "participant already exists"
)

type CreateParticipantRequestBody struct {
	Participant string `json:"participant"`
}

type CreateParticipantResponseBody struct {
	Message string `json:"message"`
}
