package model

import "github.com/guregu/null"

type EnvironmentListing struct {
	ID                        int64
	Name                      string
	DeployedParticipantCount  int
	LastDeploymentParticipant null.String
	LastDeploymentVersion     null.String
	LastDeployedAt            null.Time
}
