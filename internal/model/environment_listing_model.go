package model

type EnvironmentListing struct {
	ID                       int64
	Name                     string
	DeployedParticipantCount int
	LastDeployment           *Deployment
}
