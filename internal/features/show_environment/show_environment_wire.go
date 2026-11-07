package show_environment

import (
	"net/url"
	"time"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	environmentIDParam = "id"
	historyPageParam   = "history_page"
	checksPageParam    = "checks_page"

	showEnvironmentTemplate = "show_environment.html"

	counterpartsPerRow = 3
)

type ShowEnvironmentPage struct {
	Environment    *model.Environment
	Deployed       []DeploymentRow
	LastDeployedAt time.Time
	History        []DeploymentRow
	HistoryPager   pages.Pager
	Checks         []CheckRow
	ChecksPager    pages.Pager
}

type DeploymentRow struct {
	ParticipantName string
	ParticipantURL  string
	Version         string
	VersionURL      string
	Rollback        bool
	DeployedAt      time.Time
}

type CheckRow struct {
	ReportedCheck
	ParticipantURL string
	VersionURL     string
	Shown          []CounterpartBadge
	More           int
}

type CounterpartBadge struct {
	breakreport.Counterpart
	ParticipantURL string
}

func newShowEnvironmentPage(output ShowEnvironmentOutput, historyPager, checksPager pages.Pager) ShowEnvironmentPage {
	page := ShowEnvironmentPage{
		Environment:  output.Environment,
		Deployed:     toDeploymentRows(output.Deployed),
		History:      toDeploymentRows(output.Deployments),
		HistoryPager: historyPager,
		ChecksPager:  checksPager,
	}
	for _, deployment := range output.Deployed {
		if deployment.DeployedAt.After(page.LastDeployedAt) {
			page.LastDeployedAt = deployment.DeployedAt
		}
	}
	for _, reported := range output.Checks {
		page.Checks = append(page.Checks, toCheckRow(reported))
	}

	return page
}

func toDeploymentRows(deployments []model.Deployment) []DeploymentRow {
	rows := make([]DeploymentRow, 0, len(deployments))
	for _, deployment := range deployments {
		rows = append(rows, DeploymentRow{
			ParticipantName: deployment.Participant.Name,
			ParticipantURL:  participantURL(deployment.Participant.Name),
			Version:         deployment.Version,
			VersionURL:      versionURL(deployment.Participant.Name, deployment.Version),
			Rollback:        deployment.Rollback,
			DeployedAt:      deployment.DeployedAt,
		})
	}

	return rows
}

func toCheckRow(reported ReportedCheck) CheckRow {
	var breaking, compatible []breakreport.Counterpart
	for _, counterpart := range reported.Report.Counterparts {
		if counterpart.Breaks > 0 {
			breaking = append(breaking, counterpart)
		} else {
			compatible = append(compatible, counterpart)
		}
	}

	ordered := append(breaking, compatible...)
	shown := make([]CounterpartBadge, 0, counterpartsPerRow)
	for _, counterpart := range ordered[:min(len(ordered), counterpartsPerRow)] {
		badge := CounterpartBadge{Counterpart: counterpart}
		if counterpart.Registered {
			badge.ParticipantURL = participantURL(counterpart.Name)
		}
		shown = append(shown, badge)
	}

	return CheckRow{
		ReportedCheck:  reported,
		ParticipantURL: participantURL(reported.Check.ParticipantName),
		VersionURL:     versionURL(reported.Check.ParticipantName, reported.Check.Version),
		Shown:          shown,
		More:           len(ordered) - len(shown),
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
