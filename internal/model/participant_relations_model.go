package model

import "github.com/guregu/null"

type RelationsHeader struct {
	Participant     *Participant
	LatestVersion   null.String
	Deployed        []Deployment
	VersionCount    int
	DeploymentCount int
	CheckCount      int
}

type ConsumedProvider struct {
	Name       string
	Registered bool
	Published  bool
	Deployed   []Deployment
}

type EnvironmentConsumers struct {
	EnvironmentID   int64
	EnvironmentName string
	Consumers       []DeployedConsumer
}

type DeployedConsumer struct {
	Name    string
	Version string
}
