package show_participant_relations

import "github.com/bidirekt/broker/internal/model"

const (
	participantNameParam = "name"

	showParticipantRelationsTemplate = "show_participant_relations.html"
)

type ShowParticipantRelationsPage struct {
	Header         *model.RelationsHeader
	Consumes       []model.ConsumedProvider
	ConsumedBy     []model.EnvironmentConsumers
	RelationsCount int
}

func newShowParticipantRelationsPage(output ShowParticipantRelationsOutput) ShowParticipantRelationsPage {
	return ShowParticipantRelationsPage{
		Header:         output.Header,
		Consumes:       output.Consumes,
		ConsumedBy:     output.ConsumedBy,
		RelationsCount: output.RelationsCount,
	}
}
