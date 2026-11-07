package model

import (
	"time"

	"github.com/guregu/null"
)

type ParticipantVersionsHeader struct {
	Participant     Participant
	LatestVersion   null.String
	DeployedNow     []Deployment
	VersionCount    int
	DeploymentCount int
	CheckCount      int
	RelationCount   int
}

type PublishedVersion struct {
	Version     string
	PublishedAt time.Time
	DeployedIn  []Environment
}
