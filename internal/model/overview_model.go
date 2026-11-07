package model

type OverviewNumbers struct {
	Participants      int
	NeverPublished    int
	Environments      int
	VersionsPublished int
	ChecksThisMonth   int
}

type OverviewSaidNoCheck struct {
	CompatibilityCheck
	DeployedThereAnyway bool
}
