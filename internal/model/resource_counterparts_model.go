package model

import "github.com/guregu/null"

type ResourceCounterparts struct {
	Providers           map[string]PersistedResource   // provider hash → the provider deployed in the environment
	Consumers           map[string][]PersistedResource // provider hash → the consumers of that hash in the environment
	ProviderHints       map[string]any                 // provider hash not found, where the deployed provider declares something around it → what it declares, in the resource protocol's own type
	RegisteredProviders map[string]RegisteredProvider  // consumed provider name that is a registered participant → that participant, with its version deployed in the environment
}

type RegisteredProvider struct {
	ParticipantID   int64
	DeployedVersion null.String
}
