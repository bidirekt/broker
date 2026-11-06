package list_environment_pages

import (
	"net/url"
	"time"

	"github.com/bidirekt/broker/internal/components/pages"
)

const (
	pageParam = "page"

	listEnvironmentPagesTemplate = "list_environment_pages.html"
)

type ListEnvironmentPagesPage struct {
	Environments []EnvironmentRow
	Pager        pages.Pager
}

type EnvironmentRow struct {
	ID                       int64
	Name                     string
	DeployedParticipantCount int
	LastDeployment           *LastDeploymentRow
}

type LastDeploymentRow struct {
	Participant    string
	ParticipantURL string
	Version        string
	VersionURL     string
	DeployedAt     time.Time
}

func newListEnvironmentPagesPage(output ListEnvironmentPagesOutput, pager pages.Pager) ListEnvironmentPagesPage {
	page := ListEnvironmentPagesPage{Environments: make([]EnvironmentRow, 0, len(output.Environments)), Pager: pager}
	for _, environment := range output.Environments {
		row := EnvironmentRow{ID: environment.ID, Name: environment.Name, DeployedParticipantCount: environment.DeployedParticipantCount}
		if last := environment.LastDeployment; last != nil {
			row.LastDeployment = &LastDeploymentRow{
				Participant:    last.Participant.Name,
				ParticipantURL: participantURL(last.Participant.Name),
				Version:        last.Version,
				VersionURL:     versionURL(last.Participant.Name, last.Version),
				DeployedAt:     last.DeployedAt,
			}
		}
		page.Environments = append(page.Environments, row)
	}

	return page
}

func participantURL(participantName string) string {
	return "/participants/" + url.PathEscape(participantName)
}

func versionURL(participantName, version string) string {
	if version == "" || version == "." || version == ".." {
		return ""
	}

	return participantURL(participantName) + "/versions/" + url.PathEscape(version)
}
