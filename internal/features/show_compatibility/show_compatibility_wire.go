package show_compatibility

import (
	"cmp"
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

	maxDetailColumns = 6
)

type ShowCompatibilityPage struct {
	Participants   []ParticipantOption
	Participant    *model.Participant
	ParticipantURL string
	Environments   []model.Environment
	Columns        int
	DetailGrid     string
	VersionsCount  int
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
	DeployedIn  []model.Environment
	Open        bool
	Cells       []Cell
}

type Cell struct {
	Environment model.Environment
	Check       *CheckCell
}

type CheckCell struct {
	ReportedCheck
	Counterparts   []CounterpartRow
	VersionMissing bool
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
		Participant:   output.Participant,
		Environments:  output.Environments,
		Columns:       len(output.Environments) + 1,
		DetailGrid:    fmt.Sprintf("n%d", min(len(output.Environments), maxDetailColumns)),
		VersionsCount: output.VersionsTotal,
		Pager:         pager,
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

	for i, version := range output.Versions {
		row := VersionRow{
			Version:     version.Version,
			VersionURL:  versionURL(output.Participant.Name, version.Version),
			PublishedAt: version.PublishedAt,
			DeployedIn:  version.DeployedIn,
			Open:        i == 0,
		}
		for _, environment := range output.Environments {
			cell := Cell{Environment: environment}
			if reported, checked := checksByCell[cellKey{version.Version, environment.ID}]; checked {
				cell.Check = toCheckCell(reported)
			}
			row.Cells = append(row.Cells, cell)
		}
		page.Rows = append(page.Rows, row)
	}

	return page
}

func toCheckCell(reported ReportedCheck) *CheckCell {
	counterparts := slices.Clone(reported.Report.Counterparts)
	slices.SortFunc(counterparts, func(a, b breakreport.Counterpart) int {
		return cmp.Or(compareBreakingFirst(a.Deployable, b.Deployable), strings.Compare(a.Name, b.Name))
	})

	cell := &CheckCell{ReportedCheck: reported}
	for _, counterpart := range counterparts {
		row := CounterpartRow{Counterpart: counterpart}
		if counterpart.Registered {
			row.ParticipantURL = participantURL(counterpart.Name)
		}
		cell.VersionMissing = cell.VersionMissing || counterpart.Version == ""
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

func participantURL(participantName string) string {
	return "/participants/" + url.PathEscape(participantName)
}

func versionURL(participantName, version string) string {
	if version == "" || version == "." || version == ".." {
		return ""
	}

	return participantURL(participantName) + "/versions/" + url.PathEscape(version)
}
