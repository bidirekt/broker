package repository

type CreateParticipantOutcome int

const (
	ParticipantCreated CreateParticipantOutcome = iota
	ParticipantExists
	ParticipantNameReserved
)

// CreateParticipantResult carries the reserving protocol's Message when the Outcome is ParticipantNameReserved.
type CreateParticipantResult struct {
	Outcome CreateParticipantOutcome
	Message string
}
