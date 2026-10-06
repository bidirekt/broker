package model

import (
	"sort"

	"github.com/guregu/null"
)

type PersistedResource struct {
	ParticipantID      int64               `json:"-"`
	ParticipantName    string              `json:"-"`
	ContractID         int64               `json:"-"`
	Direction          Direction           `json:"direction"`
	Interaction        Interaction         `json:"interaction"`
	ConsumedProvider   null.String         `json:"consumedProvider"`
	ConsumerHash       null.String         `json:"-"`
	ProviderHash       string              `json:"-"`
	Endpoint           string              `json:"endpoint"`
	Method             string              `json:"method"`
	ResponseStatusCode null.String         `json:"responseStatusCode"`
	Properties         map[string]Property `json:"-"`
	ParticipantVersion null.String         `json:"version"`
	DeployedVersions   map[string]string   `json:"-"`
	Removed            bool                `json:"-"`
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
