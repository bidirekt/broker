package list_participants

const ParticipantsListedMessage string = "participants listed successfully"

type ListParticipantsResponseBody struct {
	Message      string   `json:"message"`
	Participants []string `json:"participants"`
}

type ListParticipantsErrorResponseBody struct {
	Message string `json:"message"`
}
