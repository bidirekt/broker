package model

import "github.com/guregu/null"

type ResourceCounterparts struct {
	Providers          map[string]PersistedResource   // provider hash → the provider deployed in the environment
	Consumers          map[string][]PersistedResource // provider hash → the consumers of that hash in the environment
	ProviderStatuses   map[string][]string            // provider hash not found, on an endpoint and method the deployed provider declares → the statuses it declares there
	UnmatchedProviders map[string]RegisteredProvider  // provider hash not found, consumed name a registered participant → that participant
}

type RegisteredProvider struct {
	ParticipantID   int64
	DeployedVersion null.String
}
