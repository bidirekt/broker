package create_environment

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type CreateEnvironmentStatus int

const (
	EnvironmentCreated CreateEnvironmentStatus = iota
	EnvironmentAlreadyExists
)

type CreateEnvironmentInput struct {
	Environment string
}

type CreateEnvironmentOutput struct {
	Status CreateEnvironmentStatus
	ID     int64
}

type CreateEnvironmentUseCase struct {
	environmentRepository *repository.EnvironmentRepository
}

func NewCreateEnvironmentUseCase(environmentRepository *repository.EnvironmentRepository) *CreateEnvironmentUseCase {
	return &CreateEnvironmentUseCase{environmentRepository: environmentRepository}
}

func (this *CreateEnvironmentUseCase) Execute(ctx context.Context, input CreateEnvironmentInput) CreateEnvironmentOutput {
	environment := model.NewEnvironment(input.Environment)
	created := this.environmentRepository.Create(ctx, environment)
	if !created {
		return CreateEnvironmentOutput{Status: EnvironmentAlreadyExists}
	}

	return CreateEnvironmentOutput{Status: EnvironmentCreated, ID: environment.ID}
}
