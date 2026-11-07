package show_contract_version

import (
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	participantNameParam = "name"
	versionParam         = "version"
	deploymentsPageParam = "deployments_page"
	checksPageParam      = "checks_page"

	showContractVersionTemplate = "show_contract_version.html"

	counterpartsPerRow = 3
)

type ShowContractVersionPage struct {
	ContractVersion    *model.ContractVersion
	ParticipantURL     string
	Files              []ShownFile
	CurrentDeployments []model.Deployment
	LatestChecks       []ReportedCheck
	Deployments        []model.Deployment
	DeploymentsPager   pages.Pager
	Checks             []CheckRow
	ChecksPager        pages.Pager
}

type ShownFile struct {
	Source string
	Lines  []string
}

type CheckRow struct {
	ReportedCheck
	Shown []breakreport.Counterpart
	More  int
}

func newShowContractVersionPage(output ShowContractVersionOutput, deploymentsPager, checksPager pages.Pager) ShowContractVersionPage {
	files := make([]ShownFile, 0, len(output.Files))
	for _, file := range output.Files {
		files = append(files, ShownFile{Source: file.Source, Lines: toLines(file.Content)})
	}

	checks := make([]CheckRow, 0, len(output.Checks))
	for _, reported := range output.Checks {
		checks = append(checks, toCheckRow(reported))
	}

	return ShowContractVersionPage{
		ContractVersion:    output.ContractVersion,
		ParticipantURL:     participantURL(output.ContractVersion.ParticipantName),
		Files:              files,
		CurrentDeployments: output.CurrentDeployments,
		LatestChecks:       output.LatestChecks,
		Deployments:        output.Deployments,
		DeploymentsPager:   deploymentsPager,
		Checks:             checks,
		ChecksPager:        checksPager,
	}
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
	shown := ordered[:min(len(ordered), counterpartsPerRow)]

	return CheckRow{ReportedCheck: reported, Shown: shown, More: len(ordered) - len(shown)}
}

func participantURL(participantName string) string {
	return "/participants/" + url.PathEscape(participantName)
}

func toLines(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
