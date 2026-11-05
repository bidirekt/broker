package model

import (
	"time"

	"github.com/guregu/null"
)

type ParticipantVersionsHeader struct {
	Participant     Participant
	LatestVersion   null.String
	DeployedNow     []EnvironmentVersion
	VersionCount    int
	DeploymentCount int
	CheckCount      int
	RelationCount   int
}

type EnvironmentVersion struct {
	Environment Environment
	Version     string
}

type PublishedVersion struct {
	Version     string
	PublishedAt time.Time
	DeployedIn  []Environment
}
