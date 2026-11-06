package list_participant_deployments

import (
	"net/url"
	"time"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	participantNameParam = "name"
	environmentParam     = "environment"
	pageParam            = "page"

	listParticipantDeploymentsTemplate = "list_participant_deployments.html"
)

type ListParticipantDeploymentsPage struct {
	Header         *repository.ParticipantDeploymentsHeader
	ParticipantURL string
	LatestURL      string
	Environments   []*model.Environment
	Environment    *model.Environment
	Deployments    []DeploymentRow
	Pager          pages.Pager
}

type DeploymentRow struct {
	Environment *model.Environment
	Version     string
	VersionURL  string
	Rollback    bool
	DeployedAt  time.Time
}

func newListParticipantDeploymentsPage(output ListParticipantDeploymentsOutput, pager pages.Pager) ListParticipantDeploymentsPage {
	participantName := output.Header.Participant.Name
	page := ListParticipantDeploymentsPage{
		Header:         output.Header,
		ParticipantURL: participantURL(participantName),
		LatestURL:      versionURL(participantName, output.Header.LatestVersion.String),
		Environments:   output.Environments,
		Environment:    output.Environment,
		Deployments:    make([]DeploymentRow, 0, len(output.Deployments)),
		Pager:          pager,
	}
	for _, deployment := range output.Deployments {
		page.Deployments = append(page.Deployments, DeploymentRow{
			Environment: deployment.Environment,
			Version:     deployment.Version,
			VersionURL:  versionURL(participantName, deployment.Version),
			Rollback:    deployment.Rollback,
			DeployedAt:  deployment.DeployedAt,
		})
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
