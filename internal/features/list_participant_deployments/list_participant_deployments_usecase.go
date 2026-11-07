package list_participant_deployments

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

type ListParticipantDeploymentsStatus int

const (
	DeploymentsListed ListParticipantDeploymentsStatus = iota
	ParticipantNotFound
	EnvironmentNotFound
)

type ListParticipantDeploymentsInput struct {
	ParticipantName string
	EnvironmentID   null.Int
	Page            int64
}

type ListParticipantDeploymentsOutput struct {
	Status       ListParticipantDeploymentsStatus
	Header       *repository.ParticipantDeploymentsHeader
	Environments []*model.Environment
	Environment  *model.Environment
	Deployments  []model.Deployment
	Total        int
}

type ListParticipantDeploymentsUseCase struct {
	participantDeploymentsRepository *repository.ParticipantDeploymentsRepository
}

func NewListParticipantDeploymentsUseCase(
	participantDeploymentsRepository *repository.ParticipantDeploymentsRepository,
) *ListParticipantDeploymentsUseCase {
	return &ListParticipantDeploymentsUseCase{participantDeploymentsRepository: participantDeploymentsRepository}
}

func (this *ListParticipantDeploymentsUseCase) Execute(
	ctx context.Context,
	input ListParticipantDeploymentsInput,
) ListParticipantDeploymentsOutput {
	header, found := this.participantDeploymentsRepository.FindHeader(ctx, input.ParticipantName)
	if !found {
		return ListParticipantDeploymentsOutput{Status: ParticipantNotFound}
	}

	output := ListParticipantDeploymentsOutput{Status: DeploymentsListed, Header: header}
	if input.EnvironmentID.Valid {
		output.Environment, found = this.participantDeploymentsRepository.FindEnvironment(ctx, input.EnvironmentID.Int64)
		if !found {
			return ListParticipantDeploymentsOutput{Status: EnvironmentNotFound}
		}
	}

	output.Environments = this.participantDeploymentsRepository.ListDeployedEnvironments(ctx, header.Participant.ID)
	output.Deployments, output.Total = this.participantDeploymentsRepository.List(
		ctx,
		header.Participant.ID,
		input.EnvironmentID,
		pages.PageSize,
		pages.Offset(input.Page),
	)

	return output
}
