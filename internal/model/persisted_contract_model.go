package model

type PersistedContract struct {
	ID              int64
	ParticipantID   int64
	ParticipantName string
	Version         string
	ContractContent string
	Resources       map[string]PersistedResource
	Schemas         Schemas
}
