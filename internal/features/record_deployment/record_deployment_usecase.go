package record_deployment

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type RecordDeploymentStatus int

const (
	DeploymentRecorded RecordDeploymentStatus = iota
	DeploymentRolledBack
	ParticipantNotFound
	VersionNotFound
	EnvironmentNotFound
)

type RecordDeploymentInput struct {
	Participant string
	Version     string
	Environment string
}

type RecordDeploymentOutput struct {
	Status RecordDeploymentStatus
}

type RecordDeploymentUseCase struct {
	deploymentRepository  *repository.DeploymentRepository
	participantRepository *repository.ParticipantRepository
	environmentRepository *repository.EnvironmentRepository
}

func NewRecordDeploymentUseCase(
	deploymentRepository *repository.DeploymentRepository,
	participantRepository *repository.ParticipantRepository,
	environmentRepository *repository.EnvironmentRepository,
) *RecordDeploymentUseCase {
	return &RecordDeploymentUseCase{
		deploymentRepository:  deploymentRepository,
		participantRepository: participantRepository,
		environmentRepository: environmentRepository,
	}
}

func (this *RecordDeploymentUseCase) Execute(ctx context.Context, input RecordDeploymentInput) RecordDeploymentOutput {
	participant, exists := this.participantRepository.FindByName(ctx, input.Participant)
	if !exists {
		return RecordDeploymentOutput{Status: ParticipantNotFound}
	}

	if !this.participantRepository.HasContractForVersion(ctx, participant.ID, input.Version) {
		return RecordDeploymentOutput{Status: VersionNotFound}
	}

	environment, exists := this.environmentRepository.FindByName(ctx, input.Environment)
	if !exists {
		return RecordDeploymentOutput{Status: EnvironmentNotFound}
	}

	deployment := model.NewDeployment(participant, input.Version, environment)
	recorded := this.deploymentRepository.Insert(ctx, deployment)

	if recorded && deployment.Rollback {
		return RecordDeploymentOutput{Status: DeploymentRolledBack}
	}

	return RecordDeploymentOutput{Status: DeploymentRecorded}
}
