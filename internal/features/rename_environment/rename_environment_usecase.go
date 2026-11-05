package rename_environment

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type RenameEnvironmentStatus int

const (
	EnvironmentRenamed RenameEnvironmentStatus = iota
	EnvironmentInvalidInput
	EnvironmentNotFound
	EnvironmentAlreadyExists
)

type RenameEnvironmentByNameInput struct {
	OldName string
	NewName string
}

type RenameEnvironmentOutput struct {
	Status RenameEnvironmentStatus
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
