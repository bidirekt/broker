package show_participant_versions

import (
	"net/url"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
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
}

type VersionRow struct {
	model.PublishedVersion
	URL string
}

func newShowParticipantVersionsPage(header *model.ParticipantVersionsHeader, versions []model.PublishedVersion, pager pages.Pager) ShowParticipantVersionsPage {
	page := ShowParticipantVersionsPage{
		Header:         header,
		ParticipantURL: participantURL(header.Participant.Name),
		LatestURL:      versionURL(header.Participant.Name, header.LatestVersion.String),
		Versions:       make([]VersionRow, len(versions)),
		Pager:          pager,
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
