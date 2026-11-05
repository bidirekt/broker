package model

import "github.com/guregu/null"

type ParticipantListing struct {
	ID            int64
	Name          string
	LatestVersion null.String
	PublishedAt   null.Time
	Deployments   []ParticipantListingDeployment
}

type ParticipantListingDeployment struct {
	EnvironmentID   int64
	EnvironmentName string
	Version         string
}
