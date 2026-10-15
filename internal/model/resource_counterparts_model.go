package model

type ResourceCounterparts struct {
	Providers        map[string]PersistedResource   // provider hash → the provider deployed in the environment
	Consumers        map[string][]PersistedResource // provider hash → the consumers of that hash in the environment
	ProviderStatuses map[string][]string            // provider hash not found, on an endpoint and method the deployed provider declares → the statuses it declares there
}
