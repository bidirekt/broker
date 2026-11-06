package show_participant_relations

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowParticipantRelationsStatus int

const (
	ParticipantFound ShowParticipantRelationsStatus = iota
	ParticipantNotFound
)

type ShowParticipantRelationsInput struct {
	Name string
}

type ShowParticipantRelationsOutput struct {
	Status         ShowParticipantRelationsStatus
	Header         *model.RelationsHeader
	Consumes       []model.ConsumedProvider
	ConsumedBy     []model.EnvironmentConsumers
	RelationsCount int
}

type ShowParticipantRelationsUseCase struct {
	participantRepository *repository.ParticipantRepository
}

func NewShowParticipantRelationsUseCase(participantRepository *repository.ParticipantRepository) *ShowParticipantRelationsUseCase {
	return &ShowParticipantRelationsUseCase{participantRepository: participantRepository}
}

func (this *ShowParticipantRelationsUseCase) Execute(ctx context.Context, input ShowParticipantRelationsInput) ShowParticipantRelationsOutput {
	header, found := this.participantRepository.FindRelationsHeader(ctx, input.Name)
	if !found {
		return ShowParticipantRelationsOutput{Status: ParticipantNotFound}
	}

	consumes := this.participantRepository.ListConsumedProviders(ctx, header.Participant.ID)
	consumedBy := this.participantRepository.ListConsumersByEnvironment(ctx, header.Participant.Name)

	return ShowParticipantRelationsOutput{
		Status:         ParticipantFound,
		Header:         header,
		Consumes:       consumes,
		ConsumedBy:     consumedBy,
		RelationsCount: countDistinctNames(consumes, consumedBy),
	}
}

func countDistinctNames(consumes []model.ConsumedProvider, consumedBy []model.EnvironmentConsumers) int {
	names := make(map[string]struct{})
	for _, provider := range consumes {
		names[provider.Name] = struct{}{}
	}
	for _, environment := range consumedBy {
		for _, consumer := range environment.Consumers {
			names[consumer.Name] = struct{}{}
		}
	}

	return len(names)
}
