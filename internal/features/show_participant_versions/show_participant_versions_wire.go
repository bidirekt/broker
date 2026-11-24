package show_participant_versions

import (
	"net/url"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	participantNameParam = "name"
	pageParam            = "page"

	showParticipantVersionsTemplate = "show_participant_versions.html"
)

type ShowParticipantVersionsPage struct {
	Header         *model.ParticipantVersionsHeader
	ParticipantURL string
	LatestURL      string
	Versions       []VersionRow
	Pager          pages.Pager
	Links          []repository.Link
}

type VersionRow struct {
	model.PublishedVersion
	URL string
}

func newShowParticipantVersionsPage(output ShowParticipantVersionsOutput, pager pages.Pager) ShowParticipantVersionsPage {
	header, versions := output.Header, output.Versions
	page := ShowParticipantVersionsPage{
		Header:         header,
		ParticipantURL: participantURL(header.Participant.Name),
		LatestURL:      versionURL(header.Participant.Name, header.LatestVersion.String),
		Versions:       make([]VersionRow, len(versions)),
		Pager:          pager,
		Links:          output.Links,
	}
	for i, version := range versions {
		page.Versions[i] = VersionRow{PublishedVersion: version, URL: versionURL(header.Participant.Name, version.Version)}
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
