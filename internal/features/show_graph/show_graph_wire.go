package show_graph

import (
	"net/url"
	"strconv"
)

const (
	graphNameParam = "name"

	showGraphTemplate = "show_graph.html"
)

type ShowGraphPage struct {
	Name         string
	Environments []EnvironmentSection
}

type EnvironmentSection struct {
	Name    string
	URL     string
	Members []MemberRow
	Errors  []string
}

type MemberRow struct {
	ParticipantName string
	ParticipantURL  string
	Version         string
	VersionURL      string
}

func newShowGraphPage(output ShowGraphOutput) ShowGraphPage {
	page := ShowGraphPage{Name: output.Name, Environments: make([]EnvironmentSection, 0, len(output.Environments))}
	for _, composed := range output.Environments {
		page.Environments = append(page.Environments, toEnvironmentSection(composed))
	}

	return page
}

func toEnvironmentSection(composed ComposedEnvironment) EnvironmentSection {
	section := EnvironmentSection{
		Name:    composed.Environment.Name,
		URL:     "/environments/" + strconv.FormatInt(composed.Environment.ID, 10),
		Members: make([]MemberRow, 0, len(composed.Members)),
		Errors:  composed.Errors,
	}
	for _, member := range composed.Members {
		section.Members = append(section.Members, MemberRow{
			ParticipantName: member.ParticipantName,
			ParticipantURL:  participantURL(member.ParticipantName),
			Version:         member.Version,
			VersionURL:      versionURL(member.ParticipantName, member.Version),
		})
	}

	return section
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
