package model

import "github.com/guregu/null"

type PersistedResource struct {
	Resource
	ParticipantID      int64
	ContractID         int64
	ConsumerHash       null.String
	ProviderHash       string
	Schemas            Schemas
	ParticipantVersion null.String
	Removed            bool
}

func (this *PersistedResource) PrimaryHash() string {
	if this.IsProvider() {
		return this.ProviderHash
	}

	return this.ConsumerHash.String
}
