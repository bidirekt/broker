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
	Header   *model.ParticipantVersionsHeader
	Versions []VersionRow
	Pager    pages.Pager
}

type VersionRow struct {
	model.PublishedVersion
	URL string
}

func newShowParticipantVersionsPage(header *model.ParticipantVersionsHeader, versions []model.PublishedVersion, pager pages.Pager) ShowParticipantVersionsPage {
	page := ShowParticipantVersionsPage{Header: header, Versions: make([]VersionRow, len(versions)), Pager: pager}
	for i, version := range versions {
		page.Versions[i] = VersionRow{PublishedVersion: version, URL: versionURL(header.Participant.Name, version.Version)}
	}

	return page
}

func versionURL(participantName, version string) string {
	if version == "." || version == ".." {
		return ""
	}

	return "/participants/" + url.PathEscape(participantName) + "/versions/" + url.PathEscape(version)
}
