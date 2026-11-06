package show_contract_version

import (
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	participantNameParam = "name"
	versionParam         = "version"
	deploymentsPageParam = "deployments_page"
	checksPageParam      = "checks_page"

	showContractVersionTemplate = "show_contract_version.html"
)

type ShowContractVersionPage struct {
	ContractVersion    *model.ContractVersion
	ParticipantURL     string
	Files              []ShownFile
	CurrentDeployments []model.Deployment
	LatestChecks       []ReportedCheck
	Deployments        []model.Deployment
	DeploymentsPager   pages.Pager
	Checks             []ReportedCheck
	ChecksPager        pages.Pager
}

type ShownFile struct {
	Source string
	Lines  []string
}

func newShowContractVersionPage(output ShowContractVersionOutput, deploymentsPager, checksPager pages.Pager) ShowContractVersionPage {
	files := make([]ShownFile, 0, len(output.Files))
	for _, file := range output.Files {
		files = append(files, ShownFile{Source: file.Source, Lines: toLines(file.Content)})
	}

	return ShowContractVersionPage{
		ContractVersion:    output.ContractVersion,
		ParticipantURL:     participantURL(output.ContractVersion.ParticipantName),
		Files:              files,
		CurrentDeployments: output.CurrentDeployments,
		LatestChecks:       output.LatestChecks,
		Deployments:        output.Deployments,
		DeploymentsPager:   deploymentsPager,
		Checks:             output.Checks,
		ChecksPager:        checksPager,
	}
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
