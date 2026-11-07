package model

import "github.com/guregu/null"

type PersistedResource struct {
	ParticipantID      int64
	ParticipantName    string
	ContractID         int64
	Direction          Direction
	Interaction        Interaction
	ConsumedProvider   null.String
	ConsumerHash       null.String
	ProviderHash       string
	Endpoint           string
	Method             string
	ResponseStatusCode null.String
	Bodies             []ResourceBody
	Schemas            Schemas
	ParticipantVersion null.String
	Removed            bool
}

func (this *PersistedResource) IsConsumer() bool {
	return this.Direction == Consumes
}

func (this *PersistedResource) IsProvider() bool {
	return this.Direction == Provides
}

func (this *PersistedResource) PrimaryHash() string {
	if this.Direction == Provides {
		return this.ProviderHash
	}

	return this.ConsumerHash.String
}
