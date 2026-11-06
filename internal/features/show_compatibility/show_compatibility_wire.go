package show_compatibility

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	participantParam = "participant"
	pageParam        = "page"

	showCompatibilityTemplate = "show_compatibility.html"

	maxSideBySideEnvironments = 2
)

type ShowCompatibilityPage struct {
	Participants   []ParticipantOption
	Participant    *model.Participant
	ParticipantURL string
	Environments   []model.Environment
	Stacked        bool
	Rows           []VersionRow
	Pager          pages.Pager
}

type ParticipantOption struct {
	Name     string
	Selected bool
}

type VersionRow struct {
	Version     string
	VersionURL  string
	PublishedAt time.Time
	Cells       []Cell
}

type Cell struct {
	Environment model.Environment
	Deployed    bool
	Check       *CheckCell
}

type CheckCell struct {
	ReportedCheck
	Counterparts []CounterpartRow
	StaleNote    string
}

type CounterpartRow struct {
	breakreport.Counterpart
	ParticipantURL string
}

type cellKey struct {
	version       string
	environmentID int64
}

func newShowCompatibilityPage(output ShowCompatibilityOutput, pager pages.Pager) ShowCompatibilityPage {
	page := ShowCompatibilityPage{
		Participant:  output.Participant,
		Environments: output.Environments,
		Stacked:      len(output.Environments) > maxSideBySideEnvironments,
		Pager:        pager,
	}
	for _, name := range output.Participants {
		page.Participants = append(page.Participants, ParticipantOption{
			Name:     name,
			Selected: output.Participant != nil && name == output.Participant.Name,
		})
	}
	if output.Participant == nil {
		return page
	}
	page.ParticipantURL = participantURL(output.Participant.Name)

	checksByCell := make(map[cellKey]ReportedCheck, len(output.Checks))
	for _, reported := range output.Checks {
		checksByCell[cellKey{reported.Check.Version, reported.Check.EnvironmentID}] = reported
	}

	for _, version := range output.Versions {
		row := VersionRow{
			Version:     version.Version,
			VersionURL:  versionURL(output.Participant.Name, version.Version),
			PublishedAt: version.PublishedAt,
		}
		for _, environment := range output.Environments {
			cell := Cell{
				Environment: environment,
				Deployed: slices.ContainsFunc(version.DeployedIn, func(deployedIn model.Environment) bool {
					return deployedIn.ID == environment.ID
				}),
			}
			if reported, checked := checksByCell[cellKey{version.Version, environment.ID}]; checked {
				cell.Check = toCheckCell(reported, environment)
			}
			row.Cells = append(row.Cells, cell)
		}
		page.Rows = append(page.Rows, row)
	}

	return page
}

func toCheckCell(reported ReportedCheck, environment model.Environment) *CheckCell {
	counterparts := slices.Clone(reported.Report.Counterparts)
	slices.SortStableFunc(counterparts, func(a, b breakreport.Counterpart) int {
		return compareBreakingFirst(a.Deployable, b.Deployable)
	})

	cell := &CheckCell{ReportedCheck: reported, StaleNote: staleNote(reported.DeploymentsSince, environment)}
	for _, counterpart := range counterparts {
		row := CounterpartRow{Counterpart: counterpart}
		if counterpart.Registered {
			row.ParticipantURL = participantURL(counterpart.Name)
		}
		cell.Counterparts = append(cell.Counterparts, row)
	}

	return cell
}

func compareBreakingFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	}

	return 1
}

func staleNote(since model.DeploymentsSinceCheck, environment model.Environment) string {
	if since.Count == 0 {
		return ""
	}

	deployments := "deployments"
	if since.Count == 1 {
		deployments = "deployment"
	}

	return fmt.Sprintf("Since this check, %d %s to %s: %s. Ask again before deploying.",
		since.Count, deployments, environment.Name, strings.Join(since.Participants, ", "))
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
