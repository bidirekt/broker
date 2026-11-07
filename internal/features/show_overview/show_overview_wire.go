package show_overview

import (
	"net/url"
	"time"

	"github.com/bidirekt/broker/internal/model"
)

const showOverviewTemplate = "show_overview.html"

type ShowOverviewPage struct {
	Empty                     bool
	Numbers                   model.OverviewNumbers
	SaidNo                    []SaidNoRow
	SaidNoMore                int
	Environments              []model.Environment
	Deployed                  []DeployedRow
	DeployedMore              int
	DeployedParticipantsTotal int
	Activity                  []ActivityRow
}

type SaidNoRow struct {
	SaidNoCheck
	ParticipantURL string
	VersionURL     string
}

type DeployedRow struct {
	ParticipantName string
	ParticipantURL  string
	Cells           []DeployedCell
}

type DeployedCell struct {
	Environment model.Environment
	Deployed    bool
	Version     string
	VersionURL  string
	DeployedAt  time.Time
}

type ActivityRow struct {
	Deployment      *model.Deployment
	Check           *ReportedCheck
	ParticipantName string
	ParticipantURL  string
	Version         string
	VersionURL      string
	EnvironmentID   int64
	EnvironmentName string
	At              time.Time
}

func newShowOverviewPage(output ShowOverviewOutput) ShowOverviewPage {
	page := ShowOverviewPage{
		Empty:                     output.Status == BrokerEmpty,
		Numbers:                   output.Numbers,
		SaidNoMore:                output.SaidNoTotal - len(output.SaidNo),
		Environments:              output.Environments,
		DeployedMore:              output.DeployedParticipantsTotal - len(output.Deployed),
		DeployedParticipantsTotal: output.DeployedParticipantsTotal,
	}
	for _, check := range output.SaidNo {
		page.SaidNo = append(page.SaidNo, SaidNoRow{
			SaidNoCheck:    check,
			ParticipantURL: participantURL(check.Check.ParticipantName),
			VersionURL:     versionURL(check.Check.ParticipantName, check.Check.Version),
		})
	}
	for _, participant := range output.Deployed {
		page.Deployed = append(page.Deployed, toDeployedRow(participant, output.Environments))
	}
	for _, event := range output.Activity {
		page.Activity = append(page.Activity, toActivityRow(event))
	}

	return page
}

func toDeployedRow(participant DeployedParticipant, environments []model.Environment) DeployedRow {
	byEnvironment := make(map[int64]model.Deployment, len(participant.Deployments))
	for _, deployment := range participant.Deployments {
		byEnvironment[deployment.Environment.ID] = deployment
	}

	row := DeployedRow{
		ParticipantName: participant.Participant.Name,
		ParticipantURL:  participantURL(participant.Participant.Name),
	}
	for _, environment := range environments {
		cell := DeployedCell{Environment: environment}
		if deployment, deployed := byEnvironment[environment.ID]; deployed {
			cell.Deployed = true
			cell.Version = deployment.Version
			cell.VersionURL = versionURL(participant.Participant.Name, deployment.Version)
			cell.DeployedAt = deployment.DeployedAt
		}
		row.Cells = append(row.Cells, cell)
	}

	return row
}

func toActivityRow(event Event) ActivityRow {
	if event.Deployment != nil {
		deployment := event.Deployment
		return ActivityRow{
			Deployment:      deployment,
			ParticipantName: deployment.Participant.Name,
			ParticipantURL:  participantURL(deployment.Participant.Name),
			Version:         deployment.Version,
			VersionURL:      versionURL(deployment.Participant.Name, deployment.Version),
			EnvironmentID:   deployment.Environment.ID,
			EnvironmentName: deployment.Environment.Name,
			At:              deployment.DeployedAt,
		}
	}

	check := event.Check.Check
	return ActivityRow{
		Check:           event.Check,
		ParticipantName: check.ParticipantName,
		ParticipantURL:  participantURL(check.ParticipantName),
		Version:         check.Version,
		VersionURL:      versionURL(check.ParticipantName, check.Version),
		EnvironmentID:   check.EnvironmentID,
		EnvironmentName: check.EnvironmentName,
		At:              check.CreatedAt,
	}
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
