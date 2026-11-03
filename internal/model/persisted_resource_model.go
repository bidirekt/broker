package model

import (
	"sort"

	"github.com/guregu/null"
)

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
	DeployedVersions   map[string]string
	Removed            bool
}

func (this *PersistedResource) IsConsumer() bool {
	return this.Direction == Consumes
}

func (this *PersistedResource) IsProvider() bool {
	return this.Direction == Provides
}

func (this *PersistedResource) DeployedVersionIn(environment string) (string, bool) {
	version, ok := this.DeployedVersions[environment]
	return version, ok
}

func (this *PersistedResource) PrimaryHash() string {
	if this.Direction == Provides {
		return this.ProviderHash
	}

	return this.ConsumerHash.String
}

func (this *PersistedResource) DeployedEnvironments() []string {
	environments := make([]string, 0, len(this.DeployedVersions))

	for environment := range this.DeployedVersions {
		environments = append(environments, environment)
	}

	sort.Strings(environments)

	return environments
}
