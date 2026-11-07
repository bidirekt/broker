package rename_environment

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type RenameEnvironmentStatus int

const (
	EnvironmentRenamed RenameEnvironmentStatus = iota
	EnvironmentInvalidInput
	EnvironmentNotFound
	EnvironmentAlreadyExists
	EnvironmentFound
)

type RenameEnvironmentByNameInput struct {
	OldName string
	NewName string
}

type RenameEnvironmentByIDInput struct {
	ID      int64
	NewName string
}

type RenameEnvironmentOutput struct {
	Status RenameEnvironmentStatus
}

type FindEnvironmentOutput struct {
	Status      RenameEnvironmentStatus
	Environment *model.Environment
}

type RenameEnvironmentUseCase struct {
	environmentRepository *repository.EnvironmentRepository
}

func NewRenameEnvironmentUseCase(environmentRepository *repository.EnvironmentRepository) *RenameEnvironmentUseCase {
	return &RenameEnvironmentUseCase{environmentRepository: environmentRepository}
}

func (this *RenameEnvironmentUseCase) RenameByName(ctx context.Context, input RenameEnvironmentByNameInput) RenameEnvironmentOutput {
	if validations.UnpaddedValue(input.OldName) != nil {
		return RenameEnvironmentOutput{Status: EnvironmentInvalidInput}
	}

	return this.rename(input.NewName, func() (found, conflict bool) {
		return this.environmentRepository.RenameByName(ctx, input.OldName, input.NewName)
	})
}

func (this *RenameEnvironmentUseCase) FindEnvironment(ctx context.Context, id int64) FindEnvironmentOutput {
	environment, found := this.environmentRepository.FindByID(ctx, id)
	if !found {
		return FindEnvironmentOutput{Status: EnvironmentNotFound}
	}

	return FindEnvironmentOutput{Status: EnvironmentFound, Environment: environment}
}

func (this *RenameEnvironmentUseCase) RenameByID(ctx context.Context, input RenameEnvironmentByIDInput) RenameEnvironmentOutput {
	return this.rename(input.NewName, func() (found, conflict bool) {
		return this.environmentRepository.RenameByID(ctx, input.ID, input.NewName)
	})
}

func (this *RenameEnvironmentUseCase) rename(newName string, write func() (found, conflict bool)) RenameEnvironmentOutput {
	if validations.UnpaddedValue(newName) != nil {
		return RenameEnvironmentOutput{Status: EnvironmentInvalidInput}
	}

	found, conflict := write()
	if conflict {
		return RenameEnvironmentOutput{Status: EnvironmentAlreadyExists}
	}

	if !found {
		return RenameEnvironmentOutput{Status: EnvironmentNotFound}
	}

	return RenameEnvironmentOutput{Status: EnvironmentRenamed}
}
