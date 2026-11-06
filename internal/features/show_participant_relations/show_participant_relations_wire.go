package show_participant_relations

import (
	"net/url"

	"github.com/bidirekt/broker/internal/model"
)

const (
	participantNameParam = "name"

	showParticipantRelationsTemplate = "show_participant_relations.html"
)

type ShowParticipantRelationsPage struct {
	Header         *model.RelationsHeader
	ParticipantURL string
	LatestURL      string
	Consumes       []ConsumedProviderRow
	ConsumedBy     []EnvironmentConsumersRow
	RelationsCount int
}

type ConsumedProviderRow struct {
	model.ConsumedProvider
	URL string
}

type EnvironmentConsumersRow struct {
	Environment model.Environment
	Consumers   []DeployedConsumerRow
}

type DeployedConsumerRow struct {
	model.Deployment
	URL string
}

func newShowParticipantRelationsPage(output ShowParticipantRelationsOutput) ShowParticipantRelationsPage {
	participantName := output.Header.Participant.Name
	page := ShowParticipantRelationsPage{
		Header:         output.Header,
		ParticipantURL: participantURL(participantName),
		LatestURL:      versionURL(participantName, output.Header.LatestVersion.String),
		Consumes:       make([]ConsumedProviderRow, 0, len(output.Consumes)),
		ConsumedBy:     make([]EnvironmentConsumersRow, 0, len(output.ConsumedBy)),
		RelationsCount: output.RelationsCount,
	}
	for _, provider := range output.Consumes {
		page.Consumes = append(page.Consumes, ConsumedProviderRow{ConsumedProvider: provider, URL: participantURL(provider.Name)})
	}
	for _, environment := range output.ConsumedBy {
		row := EnvironmentConsumersRow{
			Environment: environment.Environment,
			Consumers:   make([]DeployedConsumerRow, 0, len(environment.Consumers)),
		}
		for _, consumer := range environment.Consumers {
			row.Consumers = append(row.Consumers, DeployedConsumerRow{Deployment: consumer, URL: participantURL(consumer.Participant.Name)})
		}
		page.ConsumedBy = append(page.ConsumedBy, row)
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
