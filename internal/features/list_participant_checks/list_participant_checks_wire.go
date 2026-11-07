package list_participant_checks

import (
	"net/url"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	nameParam = "name"
	pageParam = "page"

	listParticipantChecksTemplate = "list_participant_checks.html"

	counterpartsPerRow = 3
)

type ListParticipantChecksPage struct {
	Header         *repository.ParticipantChecksHeader
	ParticipantURL string
	LatestURL      string
	Checks         []CheckRow
	Pager          pages.Pager
}

type CheckRow struct {
	ListedCheck
	Shown []breakreport.Counterpart
	More  int
}

func newListParticipantChecksPage(output ListParticipantChecksOutput, pager pages.Pager) ListParticipantChecksPage {
	participantName := output.Header.Participant.Name
	page := ListParticipantChecksPage{
		Header:         output.Header,
		ParticipantURL: participantURL(participantName),
		LatestURL:      versionURL(participantName, output.Header.LatestVersion.String),
		Pager:          pager,
	}
	for _, listed := range output.Checks {
		page.Checks = append(page.Checks, toCheckRow(listed))
	}

	return page
}

func toCheckRow(listed ListedCheck) CheckRow {
	var breaking, compatible []breakreport.Counterpart
	for _, counterpart := range listed.Report.Counterparts {
		if counterpart.Breaks > 0 {
			breaking = append(breaking, counterpart)
		} else {
			compatible = append(compatible, counterpart)
		}
	}

	ordered := append(breaking, compatible...)
	shown := ordered[:min(len(ordered), counterpartsPerRow)]

	return CheckRow{ListedCheck: listed, Shown: shown, More: len(ordered) - len(shown)}
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
